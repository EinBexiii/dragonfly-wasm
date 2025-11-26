package config

import (
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
)

type Config struct {
	PluginDir       string                `toml:"plugin_dir"`
	DataDir         string                `toml:"data_dir"`
	EnabledPlugins  []string              `toml:"enabled_plugins"`
	DisabledPlugins []string              `toml:"disabled_plugins"`
	DefaultLimits   plugin.ResourceLimits `toml:"default_limits"`
	GlobalLimits    plugin.ResourceLimits `toml:"global_limits"`
	Security        SecurityConfig        `toml:"security"`
	Logging         LoggingConfig         `toml:"logging"`
	Performance     PerformanceConfig     `toml:"performance"`
}

type SecurityConfig struct {
	RequireSignedPlugins bool     `toml:"require_signed_plugins"`
	SandboxMode          bool     `toml:"sandbox_mode"`
	TrustedPublicKeys    []string `toml:"trusted_public_keys"`
}

type LoggingConfig struct {
	Level           string `toml:"level"`
	LogPluginOutput bool   `toml:"log_plugin_output"`
	LogEvents       bool   `toml:"log_events"`
	LogPerformance  bool   `toml:"log_performance"`
}

type PerformanceConfig struct {
	PoolSize              int    `toml:"pool_size"`
	EnableCaching         bool   `toml:"enable_caching"`
	CacheDir              string `toml:"cache_dir"`
	ParallelEventDispatch bool   `toml:"parallel_event_dispatch"`
	EventQueueSize        int    `toml:"event_queue_size"`
	WorkerCount           int    `toml:"worker_count"`
}

type RuntimeConfig struct {
	MaxMemoryBytes int64
	EventTimeout   Duration
	EnableFuel     bool
	MaxFuel        uint64
}

type Duration struct {
	time.Duration
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxMemoryBytes: 64 << 20,
		EventTimeout:   Duration{100 * time.Millisecond},
		MaxFuel:        1_000_000,
	}
}

func DefaultConfig() Config {
	return Config{
		PluginDir:     "plugins",
		DataDir:       "plugin_data",
		DefaultLimits: plugin.DefaultResourceLimits(),
		GlobalLimits: plugin.ResourceLimits{
			MaxMemoryMB:    256,
			MaxExecutionMs: 1000,
			MaxFuel:        10_000_000,
		},
		Security: SecurityConfig{
			SandboxMode: true,
		},
		Logging: LoggingConfig{
			Level:           "info",
			LogPluginOutput: true,
		},
		Performance: PerformanceConfig{
			PoolSize:              4,
			EnableCaching:         true,
			CacheDir:              "plugin_cache",
			ParallelEventDispatch: true,
			EventQueueSize:        1000,
			WorkerCount:           4,
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadOrDefault(path string) *Config {
	if cfg, err := Load(path); err == nil {
		return cfg
	}
	cfg := DefaultConfig()
	return &cfg
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) IsPluginEnabled(id string) bool {
	if slices.Contains(c.DisabledPlugins, id) {
		return false
	}
	if len(c.EnabledPlugins) == 0 {
		return true
	}
	return slices.Contains(c.EnabledPlugins, id)
}

func (c *Config) GetEffectiveLimits(limits plugin.ResourceLimits) plugin.ResourceLimits {
	if limits.MaxMemoryMB == 0 {
		limits.MaxMemoryMB = c.DefaultLimits.MaxMemoryMB
	}
	if limits.MaxExecutionMs == 0 {
		limits.MaxExecutionMs = c.DefaultLimits.MaxExecutionMs
	}
	if limits.MaxFuel == 0 {
		limits.MaxFuel = c.DefaultLimits.MaxFuel
	}

	limits.MaxMemoryMB = min(limits.MaxMemoryMB, c.GlobalLimits.MaxMemoryMB)
	limits.MaxExecutionMs = min(limits.MaxExecutionMs, c.GlobalLimits.MaxExecutionMs)
	limits.MaxFuel = min(limits.MaxFuel, c.GlobalLimits.MaxFuel)

	return limits
}
