package config

import (
	"os"
	"path/filepath"
	"runtime"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/glory0216/taux/internal/pricing"
)

// Config is the top-level configuration loaded from ~/.config/taux/config.toml.
type Config struct {
	General   GeneralConfig   `toml:"general"`
	Providers ProvidersConfig `toml:"providers"`
	Tmux      TmuxConfig      `toml:"tmux"`
	Memorize  MemorizeConfig  `toml:"memorize"`
	Cleanup   CleanupConfig   `toml:"cleanup"`
	Pricing   PricingConfig   `toml:"pricing"`
}

type GeneralConfig struct {
	DefaultLimit int `toml:"default_limit"`
	CacheTTL     int `toml:"cache_ttl"`
}

type ProvidersConfig struct {
	Enabled []string      `toml:"enabled"`
	Claude  ClaudeConfig  `toml:"claude"`
	Cursor  CursorConfig  `toml:"cursor"`
	Aider   AiderConfig   `toml:"aider"`
	Codex   CodexConfig   `toml:"codex"`
	Gemini  GeminiConfig  `toml:"gemini"`
}

type ClaudeConfig struct {
	DataDir string `toml:"data_dir"`
}

type CursorConfig struct {
	DataDir string `toml:"data_dir"`
}

// DefaultCursorDataDir returns the platform-specific Cursor data directory.
func DefaultCursorDataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User")
	default: // linux
		return filepath.Join(home, ".config", "Cursor", "User")
	}
}

type TmuxConfig struct {
	StatusInterval  int                `toml:"status_interval"`
	StatusPosition  string             `toml:"status_position"`
	KeybindingList  TmuxKeybindingList `toml:"keybinding_list"`
}

type TmuxKeybindingList struct {
	Dashboard  string `toml:"dashboard"`
	ActiveList string `toml:"active_list"`
	Stats      string `toml:"stats"`
}

type MemorizeConfig struct {
	Dir string `toml:"dir"` // directory to save memorized sessions
}

type CleanupConfig struct {
	DefaultMaxAge string `toml:"default_max_age"`
}

type AiderConfig struct {
	ScanDirList []string `toml:"scan_dir_list"`
}

type CodexConfig struct {
	DataDir string `toml:"data_dir"` // default: ~/.codex (CODEX_HOME env takes priority)
}

type GeminiConfig struct {
	DataDir string `toml:"data_dir"` // default: ~/.gemini (GEMINI_HOME env takes priority)
}

type PricingConfig struct {
	Override map[string]PricingOverride `toml:"override"`
}

type PricingOverride struct {
	Input      float64 `toml:"input"`       // $/MTok
	Output     float64 `toml:"output"`      // $/MTok
	CacheRead  float64 `toml:"cache_read"`  // $/MTok
	CacheWrite float64 `toml:"cache_write"` // $/MTok
}

// ToTokenPriceMap converts config pricing overrides to pricing package types.
func (pc *PricingConfig) ToTokenPriceMap() map[string]pricing.TokenPrice {
	if len(pc.Override) == 0 {
		return nil
	}
	result := make(map[string]pricing.TokenPrice, len(pc.Override))
	for model, o := range pc.Override {
		result[model] = pricing.TokenPrice{
			Input:      o.Input,
			Output:     o.Output,
			CacheRead:  o.CacheRead,
			CacheWrite: o.CacheWrite,
		}
	}
	return result
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			DefaultLimit: 20,
			CacheTTL:     10,
		},
		Providers: ProvidersConfig{
			Enabled: []string{"claude", "cursor", "aider", "codex", "gemini"},
			Claude: ClaudeConfig{
				DataDir: "~/.claude",
			},
			Cursor: CursorConfig{
				DataDir: DefaultCursorDataDir(),
			},
			Aider: AiderConfig{
				ScanDirList: []string{},
			},
			Codex: CodexConfig{
				DataDir: "~/.codex",
			},
			Gemini: GeminiConfig{
				DataDir: "~/.gemini",
			},
		},
		Tmux: TmuxConfig{
			StatusInterval: 10,
			StatusPosition: "right",
			KeybindingList: TmuxKeybindingList{
				Dashboard:  "H",
				ActiveList: "A",
				Stats:      "S",
			},
		},
		Memorize: MemorizeConfig{
			Dir: "~/.taux/memories",
		},
		Cleanup: CleanupConfig{
			DefaultMaxAge: "720h",
		},
	}
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "taux", "config.toml")
}

// Load reads the config from disk, falling back to defaults.
func Load() *Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}
	_ = toml.Unmarshal(data, cfg)
	return cfg
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ExpandPath resolves ~ to the user's home directory.
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
