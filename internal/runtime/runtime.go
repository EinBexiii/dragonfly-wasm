package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	extism "github.com/extism/go-sdk"

	"github.com/EinBexiii/dragonfly-wasm/pkg/config"
	"github.com/EinBexiii/dragonfly-wasm/pkg/types"
)

type Runtime struct {
	config    *config.RuntimeConfig
	instances map[types.PluginID]*PluginInstance
	mu        sync.RWMutex
	hostFuncs []extism.HostFunction
}

type PluginInstance struct {
	ID       types.PluginID
	Manifest types.PluginManifest
	Plugin   *extism.Plugin
	State    types.PluginState
	LoadedAt time.Time
	Metrics  *PluginMetrics
	mu       sync.Mutex
}

type PluginMetrics struct {
	EventsProcessed     uint64
	TotalProcessingTime time.Duration
	Errors              uint64
	LastEventTime       time.Time
	mu                  sync.Mutex
}

func NewRuntime(cfg *config.RuntimeConfig, hostFuncs []extism.HostFunction) *Runtime {
	return &Runtime{
		config:    cfg,
		instances: make(map[types.PluginID]*PluginInstance),
		hostFuncs: hostFuncs,
	}
}

func (r *Runtime) LoadPlugin(ctx context.Context, id types.PluginID, manifest types.PluginManifest, wasmPath string) (*PluginInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[id]; exists {
		return nil, types.NewPluginError(id, "load", types.ErrPluginAlreadyLoaded)
	}

	configStrings := make(map[string]string)
	for k, v := range manifest.Config {
		if s, ok := v.(string); ok {
			configStrings[k] = s
		}
	}

	extismManifest := extism.Manifest{
		Wasm:   []extism.Wasm{extism.WasmFile{Path: wasmPath}},
		Memory: &extism.ManifestMemory{MaxPages: uint32(r.config.MaxMemoryBytes / 65536)},
		Config: configStrings,
	}

	plugin, err := extism.NewPlugin(ctx, extismManifest, extism.PluginConfig{EnableWasi: true}, r.hostFuncs)
	if err != nil {
		return nil, types.NewPluginError(id, "create plugin", fmt.Errorf("%w: %v", types.ErrInvalidWASM, err))
	}

	if err := r.verifyExports(plugin); err != nil {
		plugin.Close()
		return nil, types.NewPluginError(id, "verify exports", err)
	}

	instance := &PluginInstance{
		ID:       id,
		Manifest: manifest,
		Plugin:   plugin,
		State:    types.StateLoaded,
		LoadedAt: time.Now(),
		Metrics:  &PluginMetrics{},
	}
	r.instances[id] = instance
	return instance, nil
}

func (r *Runtime) UnloadPlugin(id types.PluginID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, exists := r.instances[id]
	if !exists {
		return types.NewPluginError(id, "unload", types.ErrPluginNotFound)
	}

	instance.mu.Lock()
	instance.Plugin.Close()
	instance.State = types.StateUnloaded
	instance.mu.Unlock()
	delete(r.instances, id)
	return nil
}

func (r *Runtime) GetInstance(id types.PluginID) (*PluginInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	instance, exists := r.instances[id]
	return instance, exists
}

func (r *Runtime) GetAllInstances() []*PluginInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*PluginInstance, 0, len(r.instances))
	for _, instance := range r.instances {
		instances = append(instances, instance)
	}
	return instances
}

func (r *Runtime) CallPlugin(ctx context.Context, id types.PluginID, function string, input []byte) ([]byte, error) {
	instance, exists := r.GetInstance(id)
	if !exists {
		return nil, types.NewPluginError(id, "call", types.ErrPluginNotFound)
	}
	return instance.Call(ctx, function, input, r.config.EventTimeout.Duration)
}

func (r *Runtime) verifyExports(plugin *extism.Plugin) error {
	for _, export := range []string{"plugin_init", "handle_event"} {
		if !plugin.FunctionExists(export) {
			return fmt.Errorf("%w: %s", types.ErrMissingExport, export)
		}
	}
	return nil
}

func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, instance := range r.instances {
		instance.mu.Lock()
		instance.Plugin.Close()
		instance.State = types.StateUnloaded
		instance.mu.Unlock()
		delete(r.instances, id)
	}
	return nil
}

func (p *PluginInstance) Call(ctx context.Context, function string, input []byte, timeout time.Duration) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.State != types.StateEnabled && p.State != types.StateLoaded {
		return nil, types.NewPluginError(p.ID, "call", types.ErrPluginNotEnabled)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultCh := make(chan struct {
		output []byte
		err    error
	}, 1)

	go func() {
		_, output, err := p.Plugin.Call(function, input)
		resultCh <- struct {
			output []byte
			err    error
		}{output, err}
	}()

	select {
	case <-ctx.Done():
		p.Metrics.recordError()
		return nil, types.NewPluginError(p.ID, "call", types.ErrPluginTimeout)
	case result := <-resultCh:
		if result.err != nil {
			p.Metrics.recordError()
			return nil, types.NewPluginError(p.ID, "call", result.err)
		}
		p.Metrics.recordSuccess(time.Since(start))
		return result.output, nil
	}
}

func (p *PluginInstance) Enable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.State == types.StateLoaded || p.State == types.StateDisabled {
		p.State = types.StateEnabled
	}
}

func (p *PluginInstance) Disable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.State == types.StateEnabled {
		p.State = types.StateDisabled
	}
}

func (p *PluginInstance) IsEnabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.State == types.StateEnabled
}

func (p *PluginInstance) GetMetrics() PluginMetrics {
	p.Metrics.mu.Lock()
	defer p.Metrics.mu.Unlock()
	return PluginMetrics{
		EventsProcessed:     p.Metrics.EventsProcessed,
		TotalProcessingTime: p.Metrics.TotalProcessingTime,
		Errors:              p.Metrics.Errors,
		LastEventTime:       p.Metrics.LastEventTime,
	}
}

func (m *PluginMetrics) recordSuccess(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EventsProcessed++
	m.TotalProcessingTime += duration
	m.LastEventTime = time.Now()
}

func (m *PluginMetrics) recordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors++
}

func (m *PluginMetrics) AverageProcessingTime() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.EventsProcessed == 0 {
		return 0
	}
	return m.TotalProcessingTime / time.Duration(m.EventsProcessed)
}
