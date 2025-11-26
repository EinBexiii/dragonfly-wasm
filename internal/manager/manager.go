package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"

	"github.com/EinBexiii/dragonfly-wasm/pkg/config"
	"github.com/EinBexiii/dragonfly-wasm/pkg/events"
	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
)

type Manager struct {
	config     *config.Config
	logger     *zap.Logger
	dispatcher *events.Dispatcher
	plugins    map[string]*LoadedPlugin
	loadOrder  []string
	mu         sync.RWMutex
	hostFuncs  []extism.HostFunction
	serverAPI  ServerAPI
	ctx        context.Context
	cancel     context.CancelFunc
}

type LoadedPlugin struct {
	Info     *plugin.Info
	Instance *extism.Plugin
	mu       sync.Mutex
}

type ServerAPI interface {
	GetPlayer(uuid string) (PlayerAPI, bool)
	GetAllPlayers() []PlayerAPI
	GetWorld(name string) (WorldAPI, bool)
	GetDefaultWorld() WorldAPI
	BroadcastMessage(msg string)
}

type PlayerAPI interface {
	UUID() string
	Name() string
	SendMessage(msg string)
	Teleport(x, y, z float64, worldName string) error
	Kick(reason string)
	SetHealth(health float64)
	SetGameMode(mode int)
	Position() (x, y, z float64)
	World() WorldAPI
}

type WorldAPI interface {
	Name() string
	GetBlock(x, y, z int) (blockType string, properties map[string]string)
	SetBlock(x, y, z int, blockType string, properties map[string]string) error
}

func New(cfg *config.Config, logger *zap.Logger, serverAPI ServerAPI) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		config:     cfg,
		logger:     logger.Named("plugin-manager"),
		dispatcher: events.NewDispatcher(logger.Named("event-dispatcher")),
		plugins:    make(map[string]*LoadedPlugin),
		serverAPI:  serverAPI,
		ctx:        ctx,
		cancel:     cancel,
	}
	m.hostFuncs = m.createHostFunctions()
	return m
}

func (m *Manager) LoadAll(ctx context.Context) error {
	entries, err := os.ReadDir(m.config.PluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Info("creating plugin directory", zap.String("path", m.config.PluginDir))
			return os.MkdirAll(m.config.PluginDir, 0o755)
		}
		return fmt.Errorf("read plugin directory: %w", err)
	}

	var manifests []*plugin.Manifest
	manifestPaths := make(map[string]string)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(m.config.PluginDir, entry.Name())
		manifestPath := filepath.Join(pluginPath, "plugin.toml")

		manifest, err := m.loadManifest(manifestPath)
		if err != nil {
			m.logger.Warn("failed to load manifest", zap.String("path", manifestPath), zap.Error(err))
			continue
		}

		if !m.config.IsPluginEnabled(manifest.ID) {
			m.logger.Debug("plugin disabled", zap.String("id", manifest.ID))
			continue
		}

		manifests = append(manifests, manifest)
		manifestPaths[manifest.ID] = pluginPath
	}

	sorted, err := m.sortByDependencies(manifests)
	if err != nil {
		return fmt.Errorf("dependency resolution: %w", err)
	}

	for _, manifest := range sorted {
		if err := m.loadPlugin(ctx, manifest, manifestPaths[manifest.ID]); err != nil {
			m.logger.Error("failed to load plugin", zap.String("id", manifest.ID), zap.Error(err))
		}
	}

	m.logger.Info("plugins loaded", zap.Int("count", len(m.plugins)))
	return nil
}

func (m *Manager) loadManifest(path string) (*plugin.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest plugin.Manifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}
	return &manifest, nil
}

func (m *Manager) sortByDependencies(manifests []*plugin.Manifest) ([]*plugin.Manifest, error) {
	manifestMap := make(map[string]*plugin.Manifest, len(manifests))
	for _, manifest := range manifests {
		manifestMap[manifest.ID] = manifest
	}

	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, manifest := range manifests {
		if _, exists := inDegree[manifest.ID]; !exists {
			inDegree[manifest.ID] = 0
		}

		for _, dep := range manifest.Dependencies {
			if !dep.Optional {
				if _, exists := manifestMap[dep.ID]; !exists {
					return nil, fmt.Errorf("plugin %s requires missing dependency %s", manifest.ID, dep.ID)
				}
			}
			if _, exists := manifestMap[dep.ID]; exists {
				inDegree[manifest.ID]++
				dependents[dep.ID] = append(dependents[dep.ID], manifest.ID)
			}
		}

		for _, id := range manifest.LoadAfter {
			if _, exists := manifestMap[id]; exists {
				inDegree[manifest.ID]++
				dependents[id] = append(dependents[id], manifest.ID)
			}
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var result []*plugin.Manifest
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		result = append(result, manifestMap[id])

		for _, dependent := range dependents[id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				sort.Strings(queue)
			}
		}
	}

	if len(result) != len(manifests) {
		return nil, errors.New("circular dependency detected")
	}
	return result, nil
}

func (m *Manager) loadPlugin(ctx context.Context, manifest *plugin.Manifest, pluginPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[manifest.ID]; exists {
		return fmt.Errorf("plugin %s already loaded", manifest.ID)
	}

	wasmPath := filepath.Join(pluginPath, manifest.EntryPoint)
	dataPath := filepath.Join(m.config.DataDir, manifest.ID)

	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	info := plugin.NewInfo(manifest, wasmPath, dataPath)
	info.State = plugin.StateLoading
	info.LoadedAt = time.Now()

	limits := m.config.GetEffectiveLimits(manifest.Limits)
	extismManifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: wasmPath},
		},
		Memory: &extism.ManifestMemory{
			MaxPages: uint32(limits.MaxMemoryMB * 16),
		},
	}

	instance, err := extism.NewPlugin(ctx, extismManifest, extism.PluginConfig{EnableWasi: true}, m.hostFuncs)
	if err != nil {
		info.State = plugin.StateError
		return fmt.Errorf("create WASM instance: %w", err)
	}

	if !instance.FunctionExists("plugin_init") {
		instance.Close()
		return errors.New("missing required export: plugin_init")
	}
	if !instance.FunctionExists("handle_event") {
		instance.Close()
		return errors.New("missing required export: handle_event")
	}

	loaded := &LoadedPlugin{Info: info, Instance: instance}

	initCtx, cancel := context.WithTimeout(ctx, time.Duration(limits.MaxExecutionMs)*time.Millisecond*10)
	defer cancel()

	if _, _, err = loaded.Instance.Call("plugin_init", nil); err != nil {
		instance.Close()
		if initCtx.Err() != nil {
			return errors.New("plugin initialization timed out")
		}
		return fmt.Errorf("plugin initialization failed: %w", err)
	}

	info.State = plugin.StateLoaded
	m.plugins[manifest.ID] = loaded
	m.loadOrder = append(m.loadOrder, manifest.ID)

	m.registerEventHandlers(loaded)

	m.logger.Info("plugin loaded", zap.String("id", manifest.ID), zap.String("version", manifest.Version.String()))
	return nil
}

func (m *Manager) registerEventHandlers(loaded *LoadedPlugin) {
	for _, sub := range loaded.Info.Manifest.Events {
		m.dispatcher.Subscribe(sub.Event, events.Subscription{
			PluginID:        loaded.Info.Manifest.ID,
			Priority:        sub.Priority,
			Handler:         m.createEventHandler(loaded, sub.Event),
			IgnoreCancelled: sub.IgnoreCancelled,
		})
	}
}

func (m *Manager) createEventHandler(loaded *LoadedPlugin, eventType plugin.EventType) events.Handler {
	return func(ctx context.Context, data []byte) (*events.EventResult, error) {
		loaded.mu.Lock()
		defer loaded.mu.Unlock()

		if loaded.Info.State != plugin.StateEnabled && loaded.Info.State != plugin.StateLoaded {
			return nil, nil
		}

		limits := m.config.GetEffectiveLimits(loaded.Info.Manifest.Limits)
		ctx, cancel := context.WithTimeout(ctx, time.Duration(limits.MaxExecutionMs)*time.Millisecond)
		defer cancel()

		start := time.Now()
		envelope := append([]byte(eventType), 0)
		envelope = append(envelope, data...)

		resultCh := make(chan struct {
			output []byte
			err    error
		}, 1)

		go func() {
			_, output, err := loaded.Instance.Call("handle_event", envelope)
			resultCh <- struct {
				output []byte
				err    error
			}{output, err}
		}()

		select {
		case <-ctx.Done():
			loaded.Info.Metrics.RecordError(ctx.Err())
			return nil, fmt.Errorf("handler timeout for %s", eventType)

		case result := <-resultCh:
			loaded.Info.Metrics.RecordCall(time.Since(start))
			if result.err != nil {
				loaded.Info.Metrics.RecordError(result.err)
				return nil, result.err
			}

			eventResult := parseEventResult(result.output)
			loaded.Info.Metrics.RecordEvent(eventType, eventResult.Cancelled)
			return eventResult, nil
		}
	}
}

func parseEventResult(data []byte) *events.EventResult {
	result := &events.EventResult{Modifications: make(map[string]string)}
	if len(data) > 0 {
		result.Cancelled = data[0] == 1
	}
	return result
}

func (m *Manager) EnablePlugin(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	loaded, exists := m.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	loaded.mu.Lock()
	defer loaded.mu.Unlock()

	if loaded.Info.State == plugin.StateEnabled {
		return nil
	}

	if loaded.Info.State != plugin.StateLoaded && loaded.Info.State != plugin.StateDisabled {
		return fmt.Errorf("plugin %s cannot be enabled in state %s", id, loaded.Info.State)
	}

	loaded.Info.State = plugin.StateEnabling

	if loaded.Instance.FunctionExists("on_enable") {
		if _, _, err := loaded.Instance.Call("on_enable", nil); err != nil {
			loaded.Info.State = plugin.StateError
			loaded.Info.Metrics.RecordError(err)
			return fmt.Errorf("enable callback failed: %w", err)
		}
	}

	loaded.Info.State = plugin.StateEnabled
	loaded.Info.EnabledAt = time.Now()
	m.logger.Info("plugin enabled", zap.String("id", id))
	return nil
}

func (m *Manager) DisablePlugin(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	loaded, exists := m.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	loaded.mu.Lock()
	defer loaded.mu.Unlock()

	if loaded.Info.State != plugin.StateEnabled {
		return nil
	}

	loaded.Info.State = plugin.StateDisabling

	if loaded.Instance.FunctionExists("on_disable") {
		_, _, _ = loaded.Instance.Call("on_disable", nil)
	}

	loaded.Info.State = plugin.StateDisabled
	loaded.Info.DisabledAt = time.Now()
	m.logger.Info("plugin disabled", zap.String("id", id))
	return nil
}

func (m *Manager) UnloadPlugin(id string) error {
	m.mu.Lock()
	loaded, exists := m.plugins[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("plugin %s not found", id)
	}

	if loaded.Info.State == plugin.StateEnabled {
		m.mu.Unlock()
		if err := m.DisablePlugin(id); err != nil {
			return err
		}
		m.mu.Lock()
	}

	m.dispatcher.Unsubscribe(id)
	loaded.Instance.Close()
	loaded.Info.State = plugin.StateUnloaded

	delete(m.plugins, id)
	m.loadOrder = slices.DeleteFunc(m.loadOrder, func(s string) bool { return s == id })
	m.mu.Unlock()

	m.logger.Info("plugin unloaded", zap.String("id", id))
	return nil
}

func (m *Manager) EnableAll() error {
	m.mu.RLock()
	ids := slices.Clone(m.loadOrder)
	m.mu.RUnlock()

	for _, id := range ids {
		if err := m.EnablePlugin(id); err != nil {
			m.logger.Error("failed to enable plugin", zap.String("id", id), zap.Error(err))
		}
	}
	return nil
}

func (m *Manager) DisableAll() {
	m.mu.RLock()
	ids := slices.Clone(m.loadOrder)
	m.mu.RUnlock()

	slices.Reverse(ids)
	for _, id := range ids {
		if err := m.DisablePlugin(id); err != nil {
			m.logger.Error("failed to disable plugin", zap.String("id", id), zap.Error(err))
		}
	}
}

func (m *Manager) Close() error {
	m.cancel()
	m.DisableAll()

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, loaded := range m.plugins {
		loaded.Instance.Close()
		delete(m.plugins, id)
	}
	return nil
}

func (m *Manager) GetPlugin(id string) (*LoadedPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[id]
	return p, ok
}

func (m *Manager) GetAllPlugins() []*LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LoadedPlugin, 0, len(m.plugins))
	for _, id := range m.loadOrder {
		result = append(result, m.plugins[id])
	}
	return result
}

func (m *Manager) Dispatcher() *events.Dispatcher {
	return m.dispatcher
}
