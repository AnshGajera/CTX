package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the full ctx configuration.
type Config struct {
	Core       CoreConfig       `toml:"core"`
	Extraction ExtractionConfig `toml:"extraction"`
	Privacy    PrivacyConfig    `toml:"privacy"`
	Sync       SyncConfig       `toml:"sync"`
}

// CoreConfig holds core runtime settings.
type CoreConfig struct {
	APIURL    string `toml:"api_url"`
	AutoSync  bool   `toml:"auto_sync"`
	WatchMode bool   `toml:"watch_mode"`
	MLURL     string `toml:"ml_url"`
}

// ExtractionConfig toggles extraction sections.
type ExtractionConfig struct {
	Architecture   bool `toml:"architecture"`
	APIEndpoints   bool `toml:"api_endpoints"`
	DatabaseSchema bool `toml:"database_schema"`
	Dependencies   bool `toml:"dependencies"`
	BusinessRules  bool `toml:"business_rules"`
	EnvVars        bool `toml:"env_vars"`
}

// PrivacyConfig controls sanitization.
type PrivacyConfig struct {
	ExcludePatterns []string `toml:"exclude_patterns"`
	RedactValues    bool     `toml:"redact_values"`
	HashIdentifiers bool     `toml:"hash_identifiers"`
}

// SyncConfig controls remote sync behavior.
type SyncConfig struct {
	IntervalSeconds int  `toml:"interval_seconds"`
	OnGitCommit     bool `toml:"on_git_commit"`
	OnFileSave      bool `toml:"on_file_save"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		Core: CoreConfig{
			APIURL:    "https://api.ctx.dev",
			AutoSync:  true,
			WatchMode: true,
			MLURL:     "http://localhost:8001",
		},
		Extraction: ExtractionConfig{
			Architecture:   true,
			APIEndpoints:   true,
			DatabaseSchema: true,
			Dependencies:   true,
			BusinessRules:  true,
			EnvVars:        true,
		},
		Privacy: PrivacyConfig{
			ExcludePatterns: []string{"*.pem", "*.key", "secrets/*"},
			RedactValues:    true,
			HashIdentifiers: false,
		},
		Sync: SyncConfig{
			IntervalSeconds: 300,
			OnGitCommit:     true,
			OnFileSave:      false,
		},
	}
}

// DefaultConfigPath returns ~/.config/ctx/config.toml.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "ctx", "config.toml"), nil
}

// ProjectConfigPath returns .ctx/config.toml under root.
func ProjectConfigPath(root string) string {
	return filepath.Join(root, ".ctx", "config.toml")
}

// Load loads config from path, falling back to defaults for missing file/keys.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	// Only apply URL defaults when the key is absent, so an explicit
	// empty value (e.g. ml_url="") disables the feature instead of
	// being forced back to the default.
	var raw map[string]any
	_ = toml.Unmarshal(data, &raw)
	coreRaw, _ := raw["core"].(map[string]any)
	if _, ok := coreRaw["api_url"]; !ok {
		if cfg.Core.APIURL == "" {
			cfg.Core.APIURL = "https://api.ctx.dev"
		}
	}
	if _, ok := coreRaw["ml_url"]; !ok {
		if cfg.Core.MLURL == "" {
			cfg.Core.MLURL = "http://localhost:8001"
		}
	}
	return cfg, nil
}

// Save writes cfg to path, creating parent dirs.
func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

// WriteDefault writes the default config to path.
func WriteDefault(path string) error {
	return Default().Save(path)
}
