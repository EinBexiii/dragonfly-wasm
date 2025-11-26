package manager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"

	"github.com/EinBexiii/dragonfly-wasm/pkg/config"
	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
)

type LoadError struct {
	PluginID string
	Path     string
	Err      error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("failed to load plugin %s from %s: %v", e.PluginID, e.Path, e.Err)
}

func (e *LoadError) Unwrap() error { return e.Err }

type Loader struct {
	config *config.Config
	logger *zap.Logger
}

func NewLoader(cfg *config.Config, logger *zap.Logger) *Loader {
	return &Loader{config: cfg, logger: logger}
}

type DiscoveredPlugin struct {
	Manifest  *plugin.Manifest
	WASMPath  string
	DataPath  string
	Directory string
}

func (l *Loader) Discover() ([]DiscoveredPlugin, error) {
	if err := os.MkdirAll(l.config.PluginDir, 0o755); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}

	entries, err := os.ReadDir(l.config.PluginDir)
	if err != nil {
		return nil, fmt.Errorf("read plugin directory: %w", err)
	}

	var discovered []DiscoveredPlugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(l.config.PluginDir, entry.Name())
		dp, err := l.loadFromDirectory(pluginPath)
		if err != nil {
			l.logger.Warn("failed to load plugin", zap.String("path", pluginPath), zap.Error(err))
			continue
		}

		if !l.config.IsPluginEnabled(dp.Manifest.ID) {
			l.logger.Debug("plugin disabled", zap.String("id", dp.Manifest.ID))
			continue
		}

		discovered = append(discovered, *dp)
		l.logger.Info("discovered plugin",
			zap.String("id", dp.Manifest.ID),
			zap.String("name", dp.Manifest.Name),
			zap.String("version", dp.Manifest.Version.String()),
		)
	}

	return discovered, nil
}

func (l *Loader) loadFromDirectory(dir string) (*DiscoveredPlugin, error) {
	manifestPath := filepath.Join(dir, "plugin.toml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, errors.New("plugin.toml not found")
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest plugin.Manifest
	if err := toml.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	wasmPath := filepath.Join(dir, manifest.EntryPoint)
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("WASM file not found: %s", manifest.EntryPoint)
	}

	dataPath := filepath.Join(l.config.DataDir, manifest.ID)
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	return &DiscoveredPlugin{
		Manifest:  &manifest,
		WASMPath:  wasmPath,
		DataPath:  dataPath,
		Directory: dir,
	}, nil
}

func (l *Loader) LoadWASM(dp *DiscoveredPlugin) ([]byte, error) {
	data, err := os.ReadFile(dp.WASMPath)
	if err != nil {
		return nil, &LoadError{PluginID: dp.Manifest.ID, Path: dp.WASMPath, Err: err}
	}
	return data, nil
}

func (l *Loader) ResolveDependencies(plugins []DiscoveredPlugin) ([]DiscoveredPlugin, error) {
	pluginMap := make(map[string]*DiscoveredPlugin, len(plugins))
	for i := range plugins {
		pluginMap[plugins[i].Manifest.ID] = &plugins[i]
	}

	for _, dp := range plugins {
		for _, dep := range dp.Manifest.Dependencies {
			if _, exists := pluginMap[dep.ID]; !exists && !dep.Optional {
				return nil, fmt.Errorf("plugin %s requires missing dependency: %s", dp.Manifest.ID, dep.ID)
			}
		}
	}

	var sorted []DiscoveredPlugin
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(id string) error
	visit = func(id string) error {
		if visited[id] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("circular dependency detected involving: %s", id)
		}

		dp, exists := pluginMap[id]
		if !exists {
			return nil
		}

		visiting[id] = true

		for _, dep := range dp.Manifest.Dependencies {
			if err := visit(dep.ID); err != nil {
				return err
			}
		}

		for _, after := range dp.Manifest.LoadAfter {
			if err := visit(after); err != nil {
				return err
			}
		}

		visiting[id] = false
		visited[id] = true
		sorted = append(sorted, *dp)
		return nil
	}

	for _, dp := range plugins {
		if err := visit(dp.Manifest.ID); err != nil {
			return nil, err
		}
	}

	return sorted, nil
}
