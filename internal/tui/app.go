package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
	"github.com/glory0216/taux/internal/provider/claude"
	"github.com/glory0216/taux/internal/stats"
	"github.com/glory0216/taux/internal/tui/component"
	"github.com/glory0216/taux/internal/tui/view"
)

// tabID identifies which tab is active.
type tabID int

const (
	tabSessions tabID = iota
	tabStats
	tabProjects
)

// viewMode identifies the current view within a tab.
type viewMode int

const (
	viewList viewMode = iota
	viewDetail
)

// --- Bubbletea messages ---

type sessionListMsg struct {
	sessionList []model.Session
	err         error
}

type sessionDetailMsg struct {
	detail *model.SessionDetail
	err    error
}

type statsMsg struct {
	stats *model.StatsCache
	agg   *model.AggregatedStats
	err   error
}

type projectListMsg struct {
	projectList []model.Project
}

type statusMsg struct {
	status *provider.ProviderStatus
}

type tickMsg time.Time

type killResultMsg struct {
	sessionID string
	err       error
}

type deleteResultMsg struct {
	sessionID string
	err       error
}

type memorizeResultMsg struct {
	sessionID string
	outPath   string
	err       error
}

type cleanBrokenResultMsg struct {
	deleted int
	err     error
}

type cleanOldResultMsg struct {
	deleted int
	days    int
	err     error
}

// AttachRequest is exported so the caller can check after TUI exits.
type AttachRequest struct {
	SessionID string
	Alias     string // tmux window name (empty → falls back to short session ID)
}

// ReplayRequest is exported so the caller can launch the replay TUI after dashboard exits.
type ReplayRequest struct {
	SessionID string
	FilePath  string
	Project   string
	Model     string
}

// Model is the main bubbletea model for the TUI dashboard.
type Model struct {
	registry *provider.Registry
	cfg      *config.Config
	version  string

	width, height int
	activeTab     tabID
	viewMode      viewMode

	// Session list
	sessionList []model.Session
	cursor      int
	offset      int // scroll offset

	// Detail view
	detail *model.SessionDetail

	// Stats
	stats *model.StatsCache
	agg   *model.AggregatedStats

	// Projects
	projectList []model.Project

	// Aggregate status for status bar
	providerStatus *provider.ProviderStatus

	// Filter
	filterActive bool
	filterText   string

	// State
	statusText string
	loading    bool
	err        error
	keys       keyMap

	// Disk usage
	diskUsageBytes int64

	// forceRefresh prevents tick from overwriting a manual refresh in flight
	forceRefresh bool

	// Confirm dialog
	confirmActive bool
	confirmPrompt string
	confirmCmd    tea.Cmd

	// Session alias
	aliasMap        map[string]string
	renameActive    bool
	renameText      string
	renameSessionID string

	// Clean old sessions input
	cleanOldActive bool
	cleanOldText   string

	// Attach request (set before quitting to signal attach)
	attachRequest *AttachRequest

	// Replay request (set before quitting to signal replay)
	replayRequest *ReplayRequest
}

// GetAttachRequest returns the attach request if the user chose to attach.
func (m *Model) GetAttachRequest() *AttachRequest {
	return m.attachRequest
}

// GetReplayRequest returns the replay request if the user chose to replay.
func (m *Model) GetReplayRequest() *ReplayRequest {
	return m.replayRequest
}

// NewModel creates a new TUI model.
func NewModel(registry *provider.Registry, cfg *config.Config, version string) *Model {
	configDir := filepath.Dir(config.ConfigPath())
	return &Model{
		registry: registry,
		cfg:      cfg,
		version:  version,
		keys:     defaultKeyMap,
		aliasMap: config.LoadAlias(configDir),
	}
}

// Init loads initial data.
func (m *Model) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(
		tea.ClearScreen,
		m.loadSessionList(),
		m.loadStats(),
		m.loadStatus(),
		m.tickCmd(),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.forceRefresh {
			// Skip tick — a manual refresh is in flight
			return m, m.tickCmd()
		}
		return m, tea.Batch(
			m.loadSessionList(),
			m.loadStatus(),
			m.tickCmd(),
		)

	case sessionListMsg:
		m.loading = false
		m.forceRefresh = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.sessionList = msg.sessionList
		m.projectList = model.BuildProjectList(msg.sessionList)
		if m.statusText == "Refreshing..." {
			m.statusText = ""
		}
		// Supplement today stats from live session data
		if m.agg != nil {
			stats.SupplementToday(m.agg, msg.sessionList, config.ExpandPath(m.cfg.Providers.Claude.DataDir), m.claudeTodayTokens)
		}
		// Clamp cursor
		filtered := m.filteredSessionList()
		if m.cursor >= len(filtered) {
			m.cursor = max(0, len(filtered)-1)
		}
		return m, nil

	case sessionDetailMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusText = "Error loading session detail"
			return m, nil
		}
		m.detail = msg.detail
		m.viewMode = viewDetail
		return m, nil

	case statsMsg:
		if msg.err == nil {
			m.stats = msg.stats
			m.agg = msg.agg
			// Supplement today stats from live session data (if already loaded)
			if len(m.sessionList) > 0 {
				stats.SupplementToday(m.agg, m.sessionList, config.ExpandPath(m.cfg.Providers.Claude.DataDir), m.claudeTodayTokens)
			}
		}
		return m, nil

	case statusMsg:
		m.providerStatus = msg.status
		return m, nil

	case aliasMoveMsg:
		delete(m.aliasMap, msg.oldSID)
		m.applyAlias(msg.newSID, msg.alias)
		return m, nil

	case killResultMsg:
		if msg.err != nil {
			m.statusText = "Kill failed: " + msg.err.Error()
		} else {
			m.statusText = "Killed session " + msg.sessionID
		}
		return m, m.loadSessionList()

	case deleteResultMsg:
		if msg.err != nil {
			m.statusText = "Delete failed: " + msg.err.Error()
		} else {
			m.statusText = "Deleted session " + msg.sessionID
		}
		return m, m.loadSessionList()

	case memorizeResultMsg:
		if msg.err != nil {
			m.statusText = "Memorize failed: " + msg.err.Error()
		} else {
			short := msg.sessionID
			if len(short) > 6 {
				short = short[:6]
			}
			m.statusText = "Memorized " + short + " → " + msg.outPath
		}
		return m, m.loadSessionList()

	case cleanBrokenResultMsg:
		if msg.err != nil {
			m.statusText = "Clean failed: " + msg.err.Error()
		} else if msg.deleted == 0 {
			m.statusText = "No broken sessions found"
		} else {
			m.statusText = fmt.Sprintf("Cleaned %d broken sessions", msg.deleted)
		}
		return m, m.loadSessionList()

	case cleanOldResultMsg:
		if msg.err != nil {
			m.statusText = "Clean failed: " + msg.err.Error()
		} else if msg.deleted == 0 {
			m.statusText = fmt.Sprintf("No sessions older than %dd", msg.days)
		} else {
			m.statusText = fmt.Sprintf("Deleted %d sessions older than %dd", msg.deleted, msg.days)
		}
		return m, m.loadSessionList()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes key presses.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If confirm dialog is active, handle y/n
	if m.confirmActive {
		return m.handleConfirmKey(msg)
	}

	// If rename is active, handle text input
	if m.renameActive {
		return m.handleRenameKey(msg)
	}

	// If clean-old days input is active, handle text input
	if m.cleanOldActive {
		return m.handleCleanOldKey(msg)
	}

	// If filter is active, handle text input
	if m.filterActive {
		return m.handleFilterKey(msg)
	}

	switch {
	case matchKey(msg, m.keys.Quit):
		return m, tea.Quit

	case matchKey(msg, m.keys.Back):
		if m.viewMode == viewDetail {
			m.viewMode = viewList
			m.detail = nil
			return m, nil
		}
		return m, tea.Quit

	case matchKey(msg, m.keys.Tab):
		m.activeTab = (m.activeTab + 1) % 3
		m.viewMode = viewList
		m.cursor = 0
		m.offset = 0
		return m, nil

	case matchKey(msg, m.keys.ShiftTab):
		m.activeTab = (m.activeTab + 2) % 3
		m.viewMode = viewList
		m.cursor = 0
		m.offset = 0
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.viewMode == viewList {
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Down):
		if m.viewMode == viewList {
			filtered := m.currentListLen()
			if m.cursor < filtered-1 {
				m.cursor++
				visibleCount := m.visibleSessionCount()
				if m.cursor >= m.offset+visibleCount {
					m.offset = m.cursor - visibleCount + 1
				}
			}
		}
		return m, nil

	case matchKey(msg, m.keys.PageDown):
		if m.viewMode == viewList {
			filtered := m.currentListLen()
			visibleCount := m.visibleSessionCount()
			m.cursor += visibleCount
			if m.cursor >= filtered {
				m.cursor = filtered - 1
			}
			if m.cursor >= m.offset+visibleCount {
				m.offset = m.cursor - visibleCount + 1
			}
		}
		return m, nil

	case matchKey(msg, m.keys.PageUp):
		if m.viewMode == viewList {
			visibleCount := m.visibleSessionCount()
			m.cursor -= visibleCount
			if m.cursor < 0 {
				m.cursor = 0
			}
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Enter):
		if m.viewMode == viewList && m.activeTab == tabSessions {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				m.loading = true
				return m, m.loadSessionDetail(filtered[m.cursor].ID)
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Search):
		if m.viewMode == viewList {
			m.filterActive = true
			return m, nil
		}
		return m, nil

	case matchKey(msg, m.keys.Refresh):
		m.loading = true
		m.statusText = "Refreshing..."
		m.registry.ClearAllCache()
		m.forceRefresh = true
		return m, tea.Batch(
			m.loadSessionList(),
			m.loadStats(),
			m.loadStatus(),
		)

	case matchKey(msg, m.keys.Kill):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				short := s.ShortID
				if s.Status == model.SessionActive {
					m.setConfirm(fmt.Sprintf("Kill session %s? [y/N]", short), m.killSession(s.ID))
				} else {
					m.setConfirm(fmt.Sprintf("Delete session %s? [y/N]", short), m.deleteSession(s.ID))
				}
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Delete):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				m.setConfirm(fmt.Sprintf("Delete session %s? [y/N]", s.ShortID), m.deleteSession(s.ID))
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Memorize):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				memorizeDir := config.ExpandPath(m.cfg.Memorize.Dir)
				m.setConfirm(
					fmt.Sprintf("Memorize & delete %s? [y/N]", s.ShortID),
					m.memorizeAndDeleteSession(s.ID, memorizeDir),
				)
			}
		}
		return m, nil

	case matchKey(msg, m.keys.MemorizeKeep):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				memorizeDir := config.ExpandPath(m.cfg.Memorize.Dir)
				m.setConfirm(
					fmt.Sprintf("Memorize %s (keep file)? [y/N]", s.ShortID),
					m.memorizeSessionOnly(s.ID, memorizeDir),
				)
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Attach):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				if s.Environment == "ide" {
					m.statusText = "Cannot attach: IDE session (use Cursor/VSCode)"
					return m, nil
				}
				if isBrokenSession(s) {
					sid := s.ID
					m.setConfirm(
						fmt.Sprintf("Broken session %s. Delete it? [y/N]", s.ShortID),
						m.deleteSession(sid),
					)
					return m, nil
				}
				m.attachRequest = &AttachRequest{
					SessionID: s.ID,
					Alias:     config.GetAlias(m.aliasMap, s.ID),
				}
				return m, tea.Quit
			}
		}
		return m, nil

	case matchKey(msg, m.keys.Replay):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				if s.FilePath == "" {
					m.statusText = "No file path for this session"
					return m, nil
				}
				m.replayRequest = &ReplayRequest{
					SessionID: s.ID,
					FilePath:  s.FilePath,
					Project:   s.Project,
					Model:     s.Model,
				}
				return m, tea.Quit
			}
		}
		return m, nil

	case matchKey(msg, m.keys.CleanBroken):
		m.setConfirm("Clean all broken sessions? [y/N]", m.cleanBrokenSessions())
		return m, nil

	case matchKey(msg, m.keys.CleanOld):
		m.cleanOldActive = true
		m.cleanOldText = "30"
		m.statusText = ""
		return m, nil

	case matchKey(msg, m.keys.Rename):
		if m.activeTab == tabSessions && m.viewMode == viewList {
			filtered := m.filteredSessionList()
			if m.cursor >= 0 && m.cursor < len(filtered) {
				s := filtered[m.cursor]
				m.renameActive = true
				m.renameSessionID = s.ID
				m.renameText = config.GetAlias(m.aliasMap, s.ID)
				m.statusText = ""
			}
		}
		return m, nil
	}

	return m, nil
}

// setConfirm activates the confirm dialog.
func (m *Model) setConfirm(prompt string, cmd tea.Cmd) {
	m.confirmActive = true
	m.confirmPrompt = prompt
	m.confirmCmd = cmd
	m.statusText = ""
}

// handleConfirmKey handles y/n input for the confirm dialog.
func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	defer func() {
		m.confirmActive = false
		m.confirmPrompt = ""
	}()

	s := string(msg.Runes)
	if s == "y" || s == "Y" {
		cmd := m.confirmCmd
		m.confirmCmd = nil
		return m, cmd
	}

	m.confirmCmd = nil
	m.statusText = "Cancelled"
	return m, nil
}

// handleRenameKey handles text input for session rename.
func (m *Model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.renameActive = false
		m.renameText = ""
		m.renameSessionID = ""
		m.statusText = "Cancelled"
		return m, nil
	case tea.KeyEnter:
		alias := strings.TrimSpace(m.renameText)
		sid := m.renameSessionID
		m.renameActive = false
		m.renameText = ""
		m.renameSessionID = ""

		if alias == "" {
			// Remove alias
			m.applyAlias(sid, "")
			return m, nil
		}

		// Check for duplicate alias on a different session
		if existingSID := m.findSessionByAlias(alias); existingSID != "" && existingSID != sid {
			existingShort := existingSID
			if len(existingShort) > 6 {
				existingShort = existingShort[:6]
			}
			// Capture values for the confirm closure
			newSID, newAlias := sid, alias
			m.setConfirm(
				fmt.Sprintf("\"%s\" already used by %s. Move? [y/N]", alias, existingShort),
				func() tea.Msg {
					return aliasMoveMsg{oldSID: existingSID, newSID: newSID, alias: newAlias}
				},
			)
			return m, nil
		}

		m.applyAlias(sid, alias)
		return m, nil
	case tea.KeyBackspace:
		if len(m.renameText) > 0 {
			m.renameText = m.renameText[:len(m.renameText)-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.renameText += string(msg.Runes)
		}
		return m, nil
	}
}

// aliasMoveMsg is dispatched when the user confirms moving an alias from one session to another.
type aliasMoveMsg struct {
	oldSID string
	newSID string
	alias  string
}

// findSessionByAlias returns the session ID that currently holds the given alias, or "".
func (m *Model) findSessionByAlias(alias string) string {
	for sid, a := range m.aliasMap {
		if a == alias {
			return sid
		}
	}
	return ""
}

// applyAlias sets or removes an alias and persists to disk.
func (m *Model) applyAlias(sid, alias string) {
	if alias == "" {
		delete(m.aliasMap, sid)
	} else {
		m.aliasMap[sid] = alias
	}

	configDir := filepath.Dir(config.ConfigPath())
	if err := config.SaveAlias(configDir, m.aliasMap); err != nil {
		m.statusText = "Save alias failed: " + err.Error()
	} else if alias == "" {
		m.statusText = "Alias removed"
	} else {
		m.statusText = "Alias set: " + alias
	}
}

// handleFilterKey handles key input while the filter bar is active.
func (m *Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.filterActive = false
		m.filterText = ""
		m.cursor = 0
		m.offset = 0
		return m, nil
	case tea.KeyEnter:
		m.filterActive = false
		m.cursor = 0
		m.offset = 0
		return m, nil
	case tea.KeyBackspace:
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.cursor = 0
			m.offset = 0
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.filterText += string(msg.Runes)
			m.cursor = 0
			m.offset = 0
		}
		return m, nil
	}
}

// handleCleanOldKey handles text input for the days prompt (L key).
func (m *Model) handleCleanOldKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.cleanOldActive = false
		m.cleanOldText = ""
		m.statusText = "Cancelled"
		return m, nil
	case tea.KeyEnter:
		text := strings.TrimSpace(m.cleanOldText)
		m.cleanOldActive = false
		m.cleanOldText = ""

		days, err := strconv.Atoi(text)
		if err != nil || days <= 0 {
			m.statusText = "Invalid number: " + text
			return m, nil
		}

		m.setConfirm(
			fmt.Sprintf("Delete all sessions older than %dd? [y/N]", days),
			m.cleanOldSessions(days),
		)
		return m, nil
	case tea.KeyBackspace:
		if len(m.cleanOldText) > 0 {
			m.cleanOldText = m.cleanOldText[:len(m.cleanOldText)-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.cleanOldText += string(msg.Runes)
		}
		return m, nil
	}
}

// View renders the TUI.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var sections []string

	// Greeting banner
	sections = append(sections, m.renderGreeting())

	// Tab bar
	tabBar := component.RenderTabs(
		[]string{"Sessions", "Stats", "Projects"},
		int(m.activeTab),
		m.width,
	)
	sections = append(sections, tabBar)

	// Filter bar (if active or has text)
	if m.filterActive || m.filterText != "" {
		sections = append(sections, view.RenderFilterBar(m.filterText, m.filterActive, m.width))
	}

	// Separator
	sections = append(sections, DimStyle.Render(strings.Repeat("─", m.width)))

	// Main content
	contentHeight := m.contentHeight()
	var content string
	switch {
	case m.viewMode == viewDetail && m.detail != nil:
		content = view.RenderSessionDetail(m.detail, m.width, contentHeight)
	case m.activeTab == tabSessions:
		content = view.RenderSessionList(m.filteredSessionList(), m.aliasMap, m.cursor, m.offset, m.width, contentHeight)
	case m.activeTab == tabStats:
		content = view.RenderStatsPanel(m.stats, m.agg, m.diskUsageBytes, m.width, contentHeight)
	case m.activeTab == tabProjects:
		content = view.RenderProjectList(m.projectList, m.cursor, m.offset, m.width, contentHeight)
	}
	sections = append(sections, content)

	// Bottom separator
	sections = append(sections, DimStyle.Render(strings.Repeat("─", m.width)))

	// Status bar
	displayStatus := m.statusText
	if m.confirmActive {
		displayStatus = m.confirmPrompt
	} else if m.cleanOldActive {
		displayStatus = "Days (default 30): " + m.cleanOldText + "█"
	} else if m.renameActive {
		displayStatus = "Rename: " + m.renameText + "█"
	}
	statusBar := component.RenderStatusBar(m.providerStatus, displayStatus, m.width)
	sections = append(sections, statusBar)

	return strings.Join(sections, "\n")
}

// renderGreeting builds the box banner header with live stats, key hints, and git URL.
func (m *Model) renderGreeting() string {
	w := m.width
	if w < 20 {
		w = 20
	}

	// Gather stats
	activeCount := 0
	totalCount := len(m.sessionList)
	for _, s := range m.sessionList {
		if s.Status == model.SessionActive {
			activeCount++
		}
	}
	todayTokens := ""
	if m.agg != nil && m.agg.TodayTokens > 0 {
		todayTokens = fmt.Sprintf(" · %s tokens today", formatCompactNumber(m.agg.TodayTokens))
	}

	// Inner width (inside box borders)
	inner := w - 4 // 2 for "│ " and " │"
	if inner < 10 {
		inner = 10
	}

	borderColor := lipgloss.NewStyle().Foreground(ColorBorder)
	titleColor := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	dimColor := lipgloss.NewStyle().Foreground(ColorDim)
	activeColor := lipgloss.NewStyle().Foreground(ColorActive)
	keyColor := lipgloss.NewStyle().Foreground(ColorAccent)
	labelColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))

	// Build lines
	topBorder := borderColor.Render("┌" + strings.Repeat("─", inner+2) + "┐")
	botBorder := borderColor.Render("└" + strings.Repeat("─", inner+2) + "┘")

	pad := func(content string, visibleLen int) string {
		gap := inner - visibleLen
		if gap < 0 {
			gap = 0
		}
		return borderColor.Render("│") + " " + content + strings.Repeat(" ", gap) + " " + borderColor.Render("│")
	}

	// Line 1: Logo + version
	logo := titleColor.Render("⬡  t a u x")
	ver := dimColor.Render("v" + m.version)
	logoVisLen := 10 // "⬡  t a u x"
	verVisLen := len("v" + m.version)
	gap := inner - logoVisLen - verVisLen
	if gap < 1 {
		gap = 1
	}
	line1 := logo + strings.Repeat(" ", gap) + ver
	line1Full := borderColor.Render("│") + " " + line1 + " " + borderColor.Render("│")

	// Line 2: Subtitle
	subtitle := dimColor.Render("extend tmux for AI sessions")
	line2 := pad(subtitle, 28)

	// Line 3: empty
	lineEmpty := pad("", 0)

	// Line 4: Stats
	statsStr := activeColor.Render(fmt.Sprintf("● %d active", activeCount)) +
		dimColor.Render(fmt.Sprintf(" · ○ %d sessions", totalCount)) +
		dimColor.Render(todayTokens)
	statsVisLen := len(fmt.Sprintf("● %d active · ○ %d sessions", activeCount, totalCount)) + len(todayTokens)
	line4 := pad(statsStr, statsVisLen)

	// Line 5: Keys row 1 — session actions
	kr1 := keyColor.Render("a") + labelColor.Render(" attach") + "  " +
		keyColor.Render("K") + labelColor.Render(" kill") + "    " +
		keyColor.Render("d") + labelColor.Render(" delete") + "  " +
		keyColor.Render("/") + labelColor.Render(" search")
	kr1VisLen := len("a attach  K kill    d delete  / search")
	line5 := pad(kr1, kr1VisLen)

	// Line 6: Keys row 2 — memorize & maintenance
	kr2 := keyColor.Render("M") + labelColor.Render(" memorize") + "  " +
		keyColor.Render("m") + labelColor.Render(" archive") + "  " +
		keyColor.Render("C") + labelColor.Render(" clean") + "  " +
		keyColor.Render("L") + labelColor.Render(" clean old")
	kr2VisLen := len("M memorize  m archive  C clean  L clean old")
	line6 := pad(kr2, kr2VisLen)

	// Line 7: Keys row 3 — navigation & misc
	kr3 := keyColor.Render("R") + labelColor.Render(" replay") + "  " +
		keyColor.Render("n") + labelColor.Render(" rename") + "  " +
		keyColor.Render("r") + labelColor.Render(" refresh") + "   " +
		keyColor.Render("q") + labelColor.Render(" quit")
	kr3VisLen := len("R replay  n rename  r refresh   q quit")
	line7 := pad(kr3, kr3VisLen)

	return strings.Join([]string{
		topBorder,
		line1Full,
		line2,
		lineEmpty,
		line4,
		lineEmpty,
		line5,
		line6,
		line7,
		botBorder,
	}, "\n")
}

// formatCompactNumber formats a number with SI suffixes.
func formatCompactNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// --- Commands ---

func (m *Model) loadSessionList() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		sessionList, err := m.registry.AllSession(ctx, provider.Filter{})
		return sessionListMsg{sessionList: sessionList, err: err}
	}
}

func (m *Model) loadSessionDetail(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, p := range m.registry.Available() {
			detail, err := p.GetSession(ctx, id)
			if err == nil && detail != nil {
				return sessionDetailMsg{detail: detail}
			}
		}
		return sessionDetailMsg{err: nil}
	}
}

func (m *Model) loadStats() tea.Cmd {
	return func() tea.Msg {
		claudeDataDir := config.ExpandPath(m.cfg.Providers.Claude.DataDir)
		statsPath := filepath.Join(claudeDataDir, "stats-cache.json")

		data, err := os.ReadFile(statsPath)
		if err != nil {
			return statsMsg{err: err}
		}

		var statsCache model.StatsCache
		if err := json.Unmarshal(data, &statsCache); err != nil {
			return statsMsg{err: err}
		}

		overrideMap := m.cfg.Pricing.ToTokenPriceMap()
		agg := stats.AggregateStats(&statsCache, overrideMap)

		// Disk usage
		projectsDir := filepath.Join(claudeDataDir, "projects")
		pattern := filepath.Join(projectsDir, "*", "*.jsonl")
		matchList, _ := filepath.Glob(pattern)
		var diskBytes int64
		for _, match := range matchList {
			if st, statErr := os.Stat(match); statErr == nil {
				diskBytes += st.Size()
			}
		}
		agg.DiskUsageBytes = diskBytes

		return statsMsg{stats: &statsCache, agg: agg}
	}
}

func (m *Model) loadStatus() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		status, _ := m.registry.AggregateStatus(ctx)
		return statusMsg{status: status}
	}
}

func (m *Model) killSession(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, p := range m.registry.Available() {
			if err := p.KillSession(ctx, id); err == nil {
				return killResultMsg{sessionID: id}
			}
		}
		return killResultMsg{sessionID: id, err: nil}
	}
}

func (m *Model) deleteSession(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, p := range m.registry.Available() {
			if err := p.DeleteSession(ctx, id); err == nil {
				return deleteResultMsg{sessionID: id}
			}
		}
		return deleteResultMsg{sessionID: id, err: fmt.Errorf("delete failed")}
	}
}

func (m *Model) memorizeAndDeleteSession(id string, outDir string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Try each provider for memorize (currently only claude supports it)
		for _, p := range m.registry.Available() {
			if cp, ok := p.(interface {
				MemorizeSession(id string, outDir string) (string, error)
			}); ok {
				outPath, err := cp.MemorizeSession(id, outDir)
				if err != nil {
					return memorizeResultMsg{sessionID: id, err: err}
				}
				// Delete the session after memorize
				_ = p.DeleteSession(ctx, id)
				return memorizeResultMsg{sessionID: id, outPath: outPath}
			}
		}
		return memorizeResultMsg{sessionID: id, err: fmt.Errorf("no provider supports memorize")}
	}
}

func (m *Model) memorizeSessionOnly(id string, outDir string) tea.Cmd {
	return func() tea.Msg {
		for _, p := range m.registry.Available() {
			if cp, ok := p.(interface {
				MemorizeSession(id string, outDir string) (string, error)
			}); ok {
				outPath, err := cp.MemorizeSession(id, outDir)
				if err != nil {
					return memorizeResultMsg{sessionID: id, err: err}
				}
				return memorizeResultMsg{sessionID: id, outPath: outPath}
			}
		}
		return memorizeResultMsg{sessionID: id, err: fmt.Errorf("no provider supports memorize")}
	}
}

func (m *Model) cleanBrokenSessions() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		sessionList, err := m.registry.AllSession(ctx, provider.Filter{})
		if err != nil {
			return cleanBrokenResultMsg{err: err}
		}

		cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		var deleted int
		for _, s := range sessionList {
			isBroken := false
			if s.StartedAt.IsZero() && s.LastActive.IsZero() {
				isBroken = true
			} else if !s.StartedAt.IsZero() && s.StartedAt.Before(cutoff) {
				isBroken = true
			} else if s.StartedAt.IsZero() && !s.LastActive.IsZero() && s.LastActive.Before(cutoff) {
				isBroken = true
			}
			if isBroken {
				for _, p := range m.registry.Available() {
					if err := p.DeleteSession(ctx, s.ID); err == nil {
						deleted++
						break
					}
				}
			}
		}
		return cleanBrokenResultMsg{deleted: deleted}
	}
}

// isBrokenSession returns true if a session has broken/missing timestamps.
func isBrokenSession(s model.Session) bool {
	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if s.StartedAt.IsZero() && s.LastActive.IsZero() {
		return true
	}
	if !s.StartedAt.IsZero() && s.StartedAt.Before(cutoff) {
		return true
	}
	if s.StartedAt.IsZero() && !s.LastActive.IsZero() && s.LastActive.Before(cutoff) {
		return true
	}
	return false
}

func (m *Model) cleanOldSessions(days int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		sessionList, err := m.registry.AllSession(ctx, provider.Filter{})
		if err != nil {
			return cleanOldResultMsg{days: days, err: err}
		}

		cutoff := time.Now().AddDate(0, 0, -days)
		var deleted int
		for _, s := range sessionList {
			ts := s.LastActive
			if ts.IsZero() {
				ts = s.StartedAt
			}
			if ts.IsZero() || ts.Before(cutoff) {
				for _, p := range m.registry.Available() {
					if err := p.DeleteSession(ctx, s.ID); err == nil {
						deleted++
						break
					}
				}
			}
		}
		return cleanOldResultMsg{deleted: deleted, days: days}
	}
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- Helpers ---

func (m *Model) filteredSessionList() []model.Session {
	if m.filterText == "" {
		return m.sessionList
	}

	aliasMap := m.aliasMap
	// First: metadata filter (fast)
	metaFiltered := provider.FilterSessionList(m.sessionList, m.filterText, func(id string) string {
		return config.GetAlias(aliasMap, id)
	})

	// If metadata matched some sessions, return those
	if len(metaFiltered) > 0 {
		return metaFiltered
	}

	// Fallback: full-text search in session files (slower)
	var result []model.Session
	for _, s := range m.sessionList {
		if s.FilePath != "" && claude.SearchInSession(s.FilePath, m.filterText) {
			result = append(result, s)
		}
	}
	return result
}

func (m *Model) currentListLen() int {
	switch m.activeTab {
	case tabSessions:
		return len(m.filteredSessionList())
	case tabProjects:
		return len(m.projectList)
	default:
		return 0
	}
}

func (m *Model) visibleSessionCount() int {
	// Sessions tab uses 2 lines per session (row + description)
	h := m.contentHeight()
	return max((h-1)/2, 1) // -1 for header, /2 for two lines each
}

func (m *Model) contentHeight() int {
	// Chrome: greeting(11) + tabs(1) + separator(1) + separator(1) + statusbar(1) = 15
	// Plus filter bar if active
	used := 15
	if m.filterActive || m.filterText != "" {
		used++
	}
	h := m.height - used
	if h < 1 {
		h = 1
	}
	return h
}

func matchKey(msg tea.KeyMsg, binding key.Binding) bool {
	for _, k := range binding.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// claudeTodayTokens adapts claude.SumTodayTokens to the stats.TodayTokensFunc signature.
func (m *Model) claudeTodayTokens(dataDir string) stats.TodayTokens {
	ts := claude.SumTodayTokens(dataDir)
	return stats.TodayTokens{
		IOTokens:   ts.IOTokens,
		CacheRead:  ts.CacheRead,
		CacheWrite: ts.CacheWrite,
	}
}

