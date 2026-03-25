package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/model"
)

func newReplayCmd(app *App) *cobra.Command {
	var noTools bool

	cmd := &cobra.Command{
		Use:   "replay <session-id>",
		Short: "Interactive conversation viewer",
		Long:  "Browse a session's conversation in a scrollable TUI. Tool calls are collapsible.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, argList []string) error {
			ctx := cmd.Context()

			fullID, err := resolveSessionID(app, ctx, argList[0])
			if err != nil {
				return err
			}

			// Find session file path and metadata
			var filePath, sessionProvider string
			var projectName, modelName string
			sessionList, _ := app.Registry.AllSession(ctx, emptyFilter())
			for _, s := range sessionList {
				if s.ID == fullID {
					filePath = s.FilePath
					sessionProvider = s.Provider
					projectName = s.Project
					modelName = s.Model
					break
				}
			}
			if filePath == "" {
				return fmt.Errorf("session file not found: %s", argList[0])
			}
			if sessionProvider != "claude" {
				providerLabel := sessionProvider
				if providerLabel == "" {
					providerLabel = "unknown"
				}
				return fmt.Errorf("taux replay is only supported for Claude sessions (this is a %s session)", providerLabel)
			}

			// Parse conversation
			turnList, err := parseConversation(filePath, noTools)
			if err != nil {
				return err
			}
			if len(turnList) == 0 {
				fmt.Println("No conversation found in this session.")
				return nil
			}

			shortID := fullID
			if len(shortID) > 6 {
				shortID = shortID[:6]
			}

			m := newReplayModel(turnList, shortID, projectName, modelName)
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}

	cmd.Flags().BoolVar(&noTools, "no-tools", false, "Hide tool call entries")

	return cmd
}

// replayTurn represents a single conversation turn for display.
type replayTurn struct {
	timestamp string
	role      string // "user", "assistant", "tool"
	content   string // full content (not truncated)
	toolName  string // for tool turns
}

// parseConversation reads a JSONL file and extracts conversation turns.
func parseConversation(filePath string, noTools bool) ([]replayTurn, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	var turnList []replayTurn

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec model.JSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		if rec.IsSidechain {
			continue
		}

		ts := ""
		if !rec.Timestamp.IsZero() {
			ts = rec.Timestamp.Format("15:04:05")
		}

		switch rec.Type {
		case "user":
			text := extractUserTextFull(rec.Message)
			if text == "" {
				continue
			}
			turnList = append(turnList, replayTurn{
				timestamp: ts,
				role:      "user",
				content:   text,
			})

		case "assistant":
			turns := extractAssistantTurnList(rec.Message, noTools)
			for i := range turns {
				turns[i].timestamp = ts
			}
			turnList = append(turnList, turns...)
		}
	}

	return turnList, nil
}

// extractUserTextFull extracts user message text without truncation.
func extractUserTextFull(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}

	var msg struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ""
	}
	if msg.Role != "user" {
		return ""
	}

	var text string
	if err := json.Unmarshal(msg.Content, &text); err == nil {
		return stripTags(text)
	}

	var blocks []model.ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err == nil {
		var partList []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				partList = append(partList, b.Text)
			}
		}
		return stripTags(strings.Join(partList, "\n"))
	}

	return ""
}

// extractAssistantTurnList extracts assistant text and tool_use entries without truncation.
func extractAssistantTurnList(raw json.RawMessage, noTools bool) []replayTurn {
	if raw == nil {
		return nil
	}

	var msg model.AssistantMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	if msg.Role != "assistant" {
		return nil
	}

	var blocks []model.ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}

	var turnList []replayTurn
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				turnList = append(turnList, replayTurn{
					role:    "assistant",
					content: b.Text,
				})
			}
		case "tool_use":
			if noTools {
				continue
			}
			summary := b.Name
			detail := ""
			if b.Input != nil {
				var inputMap map[string]interface{}
				if err := json.Unmarshal(b.Input, &inputMap); err == nil {
					if fp, ok := inputMap["file_path"].(string); ok {
						detail = fp
					} else if cmd, ok := inputMap["command"].(string); ok {
						detail = cmd
					} else if pattern, ok := inputMap["pattern"].(string); ok {
						detail = pattern
					} else if content, ok := inputMap["content"].(string); ok {
						if len(content) > 200 {
							detail = content[:197] + "..."
						} else {
							detail = content
						}
					}
				}
			}
			content := summary
			if detail != "" {
				content = summary + ": " + detail
			}
			turnList = append(turnList, replayTurn{
				role:     "tool",
				toolName: b.Name,
				content:  content,
			})
		}
	}

	return turnList
}

// ── Replay TUI Model ──

type replayModel struct {
	turnList    []replayTurn
	lineList    []replayLine // pre-rendered lines
	offset      int          // scroll offset (line index)
	width       int
	height      int
	showTools   bool
	sessionID   string
	projectName string
	modelName   string
}

type replayLine struct {
	text   string // styled text
	isTool bool   // whether this line belongs to a tool turn
}

func newReplayModel(turnList []replayTurn, sessionID, projectName, modelName string) *replayModel {
	return &replayModel{
		turnList:    turnList,
		showTools:   true,
		sessionID:   sessionID,
		projectName: projectName,
		modelName:   modelName,
	}
}

func (m *replayModel) Init() tea.Cmd {
	return nil
}

func (m *replayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildLineList()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "j", "down":
			m.scrollDown(1)
		case "k", "up":
			m.scrollUp(1)
		case "d", "ctrl+d":
			m.scrollDown(m.contentHeight() / 2)
		case "u", "ctrl+u":
			m.scrollUp(m.contentHeight() / 2)
		case "f", "pgdown", " ":
			m.scrollDown(m.contentHeight())
		case "b", "pgup":
			m.scrollUp(m.contentHeight())
		case "g", "home":
			m.offset = 0
		case "G", "end":
			m.scrollToEnd()
		case "t":
			m.showTools = !m.showTools
			m.rebuildLineList()
		}
	}
	return m, nil
}

func (m *replayModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	header := headerStyle.Render(fmt.Sprintf(" Replay: %s", m.sessionID))
	if m.projectName != "" {
		header += dimStyle.Render(fmt.Sprintf("  [%s]", m.projectName))
	}
	if m.modelName != "" {
		header += dimStyle.Render(fmt.Sprintf("  %s", m.modelName))
	}

	// Footer
	toolsIndicator := "tools:on"
	if !m.showTools {
		toolsIndicator = "tools:off"
	}
	visibleLines := m.visibleLineCount()
	totalLines := len(m.lineList)
	pct := 0
	if totalLines > 0 {
		pct = ((m.offset + visibleLines) * 100) / totalLines
		if pct > 100 {
			pct = 100
		}
	}
	footer := dimStyle.Render(fmt.Sprintf(
		" j/k:scroll  d/u:half-page  g/G:top/bottom  t:toggle-tools(%s)  q:quit  [%d%%]",
		toolsIndicator, pct))

	// Content area
	contentH := m.contentHeight()
	var visibleList []string

	for i := m.offset; i < m.offset+contentH && i < len(m.lineList); i++ {
		line := m.lineList[i]
		if line.isTool && !m.showTools {
			continue
		}
		visibleList = append(visibleList, line.text)
	}

	// Pad
	for len(visibleList) < contentH {
		visibleList = append(visibleList, "")
	}

	return header + "\n" + strings.Join(visibleList, "\n") + "\n" + footer
}

func (m *replayModel) contentHeight() int {
	return m.height - 2 // header + footer
}

func (m *replayModel) visibleLineCount() int {
	count := 0
	for i := m.offset; i < len(m.lineList) && count < m.contentHeight(); i++ {
		if m.lineList[i].isTool && !m.showTools {
			continue
		}
		count++
	}
	return count
}

func (m *replayModel) scrollDown(n int) {
	maxOffset := len(m.lineList) - m.contentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.offset += n
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

func (m *replayModel) scrollUp(n int) {
	m.offset -= n
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *replayModel) scrollToEnd() {
	maxOffset := len(m.lineList) - m.contentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.offset = maxOffset
}

func (m *replayModel) rebuildLineList() {
	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4")).Bold(true)
	assistantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true)
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	tsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	textStyle := lipgloss.NewStyle()
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	contentWidth := m.width - 2
	if contentWidth < 20 {
		contentWidth = 20
	}

	m.lineList = nil

	for _, turn := range m.turnList {
		isTool := turn.role == "tool"

		// Separator between non-tool turns
		if !isTool && len(m.lineList) > 0 {
			m.lineList = append(m.lineList, replayLine{
				text: separatorStyle.Render(strings.Repeat("\u2500", min(contentWidth, 60))),
			})
		}

		// Role header
		var roleLabel string
		switch turn.role {
		case "user":
			roleLabel = userStyle.Render("USER")
		case "assistant":
			roleLabel = assistantStyle.Render("ASSISTANT")
		case "tool":
			roleLabel = toolStyle.Render("  TOOL")
		}

		header := " " + roleLabel
		if turn.timestamp != "" {
			header += "  " + tsStyle.Render(turn.timestamp)
		}
		m.lineList = append(m.lineList, replayLine{text: header, isTool: isTool})

		// Content lines — wrap to width
		wrappedLineList := wrapText(turn.content, contentWidth-1)
		for _, wl := range wrappedLineList {
			styledLine := " " + textStyle.Render(wl)
			if isTool {
				styledLine = "  " + toolStyle.Render(wl)
			}
			m.lineList = append(m.lineList, replayLine{text: styledLine, isTool: isTool})
		}
	}

	// Clamp offset
	maxOffset := len(m.lineList) - m.contentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// wrapText wraps text to a maximum width, respecting existing newlines.
func wrapText(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var result []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			result = append(result, "")
			continue
		}

		runeList := []rune(paragraph)
		for len(runeList) > maxWidth {
			// Try to break at a space
			breakAt := maxWidth
			for i := maxWidth; i > maxWidth/2; i-- {
				if runeList[i] == ' ' {
					breakAt = i
					break
				}
			}
			result = append(result, string(runeList[:breakAt]))
			runeList = runeList[breakAt:]
			// Skip leading space after break
			if len(runeList) > 0 && runeList[0] == ' ' {
				runeList = runeList[1:]
			}
		}
		if len(runeList) > 0 {
			result = append(result, string(runeList))
		}
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
