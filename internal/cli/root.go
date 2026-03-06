package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/glory0216/taux/internal/cache"
	"github.com/glory0216/taux/internal/config"
	"github.com/glory0216/taux/internal/provider"
	"github.com/glory0216/taux/internal/provider/aider"
	"github.com/glory0216/taux/internal/provider/claude"
	"github.com/glory0216/taux/internal/provider/codex"
	"github.com/glory0216/taux/internal/provider/copilot"
	"github.com/glory0216/taux/internal/provider/cursor"
	"github.com/glory0216/taux/internal/provider/gemini"
)

// Version and Commit are set at build time via ldflags.
var (
	Version = "dev"
	Commit  = "none"
)

// App holds shared state for all commands.
type App struct {
	Registry *provider.Registry
	Config   *config.Config
	Cache    *cache.Cache
}

// NewApp creates the app with all providers registered.
func NewApp() *App {
	cfg := config.Load()
	ttl := time.Duration(cfg.General.CacheTTL) * time.Second
	c := cache.New(ttl)

	claudeDataDir := config.ExpandPath(cfg.Providers.Claude.DataDir)
	cursorDataDir := cfg.Providers.Cursor.DataDir
	if cursorDataDir == "" {
		cursorDataDir = config.DefaultCursorDataDir()
	}
	cursorDataDir = config.ExpandPath(cursorDataDir)

	// Expand aider scan directories
	var aiderScanDirList []string
	for _, dir := range cfg.Providers.Aider.ScanDirList {
		aiderScanDirList = append(aiderScanDirList, config.ExpandPath(dir))
	}

	// Codex data dir: CODEX_HOME env → config → ~/.codex
	codexDataDir := os.Getenv("CODEX_HOME")
	if codexDataDir == "" {
		codexDataDir = config.ExpandPath(cfg.Providers.Codex.DataDir)
	}

	// Gemini data dir: GEMINI_HOME env → config → ~/.gemini
	geminiDataDir := os.Getenv("GEMINI_HOME")
	if geminiDataDir == "" {
		geminiDataDir = config.ExpandPath(cfg.Providers.Gemini.DataDir)
	}

	reg := provider.NewRegistry(
		claude.New(claudeDataDir, c),
		cursor.New(cursorDataDir, c),
		aider.New(aiderScanDirList, c),
		codex.New(codexDataDir, c),
		gemini.New(geminiDataDir, c),
		copilot.New(),
	)

	return &App{
		Registry: reg,
		Config:   cfg,
		Cache:    c,
	}
}

// NewRootCmd creates the root cobra command with all subcommands.
func NewRootCmd() *cobra.Command {
	app := NewApp()

	rootCmd := &cobra.Command{
		Use:   "taux",
		Short: "taux — extend tmux for AI sessions",
		Long:  "Manage, observe, and clean up your AI agent sessions — without leaving your terminal.",
		// Do not print usage on error from subcommands.
		SilenceUsage: true,
		// taux (서브커맨드 없이) 실행 시:
		// - tmux 밖이면 → tmux 세션 생성 후 그 안에서 대시보드 실행
		// - tmux 안이면 → 바로 대시보드 실행
		RunE: func(cmd *cobra.Command, args []string) error {
			ensureTmuxSetup()
			if !isInsideTmux() {
				return launchInTmux()
			}
			return runDashboard(app)
		},
	}

	rootCmd.Version = fmt.Sprintf("%s (%s)", Version, Commit)
	rootCmd.SetVersionTemplate("taux {{.Version}}\n")

	// Register subcommands (kubectl-style)
	rootCmd.AddCommand(
		newGetCmd(app),       // taux get [sessions|projects|stats]
		newDescribeCmd(app),  // taux describe <id>
		newLogsCmd(app),      // taux logs <id>
		newReplayCmd(app),    // taux replay <id>
		newAttachCmd(app),    // taux attach <id>
		newKillCmd(app),      // taux kill <id>
		newDeleteCmd(app),    // taux delete <id>
		newMemorizeCmd(app),  // taux memorize <id>
		newCleanCmd(app),     // taux clean
		newStatusCmd(app),    // taux status (tmux)
		newSearchCmd(app),    // taux search <query>
		newDashboardCmd(app), // taux dashboard
		newSetupCmd(),        // taux setup
		newSelfUpdateCmd(),   // taux self-update
		newUninstallCmd(),    // taux uninstall
	)

	return rootCmd
}

// isInsideTmux checks if we're running inside a tmux session.
func isInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// launchInTmux creates or attaches to a tmux session named "taux" that runs
// the taux dashboard. This replaces the current process via exec.
func launchInTmux() error {
	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}

	tauxBin, err := exec.LookPath("taux")
	if err != nil {
		// Fallback to current executable
		tauxBin, _ = os.Executable()
	}

	// Check if a tmux session named "taux" already exists
	checkErr := exec.Command("tmux", "has-session", "-t", "taux").Run()
	if checkErr == nil {
		// Session exists — attach to it
		argv := []string{tmuxBin, "attach-session", "-t", "taux"}
		return syscall.Exec(tmuxBin, argv, syscall.Environ())
	}

	// Create new tmux session running "taux dashboard"
	argv := []string{tmuxBin, "new-session", "-s", "taux", tauxBin, "dashboard"}
	return syscall.Exec(tmuxBin, argv, syscall.Environ())
}

// ensureTmuxSetup silently adds the taux block to ~/.tmux.conf if not present,
// and reloads tmux. Runs only once — skips if block already exists.
func ensureTmuxSetup() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	tmuxConfPath := filepath.Join(home, ".tmux.conf")

	existing, _ := os.ReadFile(tmuxConfPath)
	if strings.Contains(string(existing), tauxBlockStart) {
		return // already configured
	}

	newContent := replaceOrAppendBlock(string(existing), tauxBlock)
	if err := os.WriteFile(tmuxConfPath, []byte(newContent), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not update %s: %v\n", tmuxConfPath, err)
		return
	}

	fmt.Printf("tmux configured: added taux block to %s\n", tmuxConfPath)

	// Reload tmux if running
	if isTmuxRunning() {
		_ = exec.Command("tmux", "source-file", tmuxConfPath).Run()
		fmt.Println("tmux config reloaded.")
	}
	fmt.Println()
}
