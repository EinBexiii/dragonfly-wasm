package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	extism "github.com/extism/go-sdk"
	"go.uber.org/zap"

	"github.com/EinBexiii/dragonfly-wasm/pkg/config"
	"github.com/EinBexiii/dragonfly-wasm/pkg/events"
	"github.com/EinBexiii/dragonfly-wasm/pkg/host"
	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
)

type Instance struct {
	mu            sync.RWMutex
	info          *plugin.Info
	plugin        *extism.Plugin
	manifest      extism.Manifest
	config        *config.Config
	logger        *zap.Logger
	hostFunctions *host.FunctionProvider
	lastCall      time.Time
	callCount     uint64
	errorCount    uint64
	totalFuel     uint64
}

func NewInstance(info *plugin.Info, wasmBytes []byte, cfg *config.Config, hostFuncs *host.FunctionProvider, logger *zap.Logger) (*Instance, error) {
	inst := &Instance{
		info:          info,
		config:        cfg,
		hostFunctions: hostFuncs,
		logger:        logger.With(zap.String("plugin", info.Manifest.ID)),
		manifest: extism.Manifest{
			Wasm: []extism.Wasm{extism.WasmData{Data: wasmBytes}},
		},
	}

	limits := cfg.GetEffectiveLimits(info.Manifest.Limits)
	_ = limits

	p, err := extism.NewPlugin(context.Background(), inst.manifest, extism.PluginConfig{EnableWasi: true}, hostFuncs.CreateHostFunctions(info.Manifest.ID))
	if err != nil {
		return nil, fmt.Errorf("create extism plugin: %w", err)
	}
	inst.plugin = p
	return inst, nil
}

func (i *Instance) Info() *plugin.Info { return i.info }

func (i *Instance) Call(ctx context.Context, funcName string, input []byte) ([]byte, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	start := time.Now()
	i.lastCall = start
	i.callCount++

	timeout := time.Duration(i.config.GetEffectiveLimits(i.info.Manifest.Limits).MaxExecutionMs) * time.Millisecond
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultCh := make(chan struct {
		result []byte
		err    error
	}, 1)

	go func() {
		_, result, err := i.plugin.Call(funcName, input)
		resultCh <- struct {
			result []byte
			err    error
		}{result, err}
	}()

	select {
	case <-callCtx.Done():
		i.errorCount++
		i.info.Metrics.RecordError(ctx.Err())
		return nil, fmt.Errorf("plugin call timed out after %v", timeout)
	case res := <-resultCh:
		i.info.Metrics.RecordCall(time.Since(start))
		if res.err != nil {
			i.errorCount++
			i.info.Metrics.RecordError(res.err)
			return nil, res.err
		}
		return res.result, nil
	}
}

func (i *Instance) HandleEvent(ctx context.Context, event plugin.EventType, data []byte) (*events.EventResult, error) {
	if !i.info.Manifest.SubscribedTo(event) {
		return nil, nil
	}

	result, err := i.Call(ctx, "on_"+string(event), data)
	if err != nil {
		return nil, err
	}

	eventResult := parseEventResult(result)
	i.info.Metrics.RecordEvent(event, eventResult.Cancelled)
	return eventResult, nil
}

func (i *Instance) Enable(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.info.State != plugin.StateLoaded && i.info.State != plugin.StateDisabled {
		return fmt.Errorf("cannot enable plugin in state: %s", i.info.State)
	}

	i.info.State = plugin.StateEnabling
	if i.plugin.FunctionExists("on_enable") {
		if _, _, err := i.plugin.Call("on_enable", nil); err != nil {
			i.info.State = plugin.StateError
			return fmt.Errorf("on_enable: %w", err)
		}
	}

	i.info.State = plugin.StateEnabled
	i.info.EnabledAt = time.Now()
	i.logger.Info("plugin enabled")
	return nil
}

func (i *Instance) Disable(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.info.State != plugin.StateEnabled {
		return fmt.Errorf("cannot disable plugin in state: %s", i.info.State)
	}

	i.info.State = plugin.StateDisabling
	if i.plugin.FunctionExists("on_disable") {
		if _, _, err := i.plugin.Call("on_disable", nil); err != nil {
			i.logger.Warn("on_disable failed", zap.Error(err))
		}
	}

	i.info.State = plugin.StateDisabled
	i.info.DisabledAt = time.Now()
	i.logger.Info("plugin disabled")
	return nil
}

func (i *Instance) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.info.State == plugin.StateEnabled {
		i.info.State = plugin.StateDisabling
		if i.plugin.FunctionExists("on_disable") {
			_, _, _ = i.plugin.Call("on_disable", nil)
		}
	}

	i.plugin.Close()
	i.info.State = plugin.StateUnloaded
	i.logger.Info("plugin unloaded")
	return nil
}

func (i *Instance) IsEnabled() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info.State == plugin.StateEnabled
}

func (i *Instance) State() plugin.State {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info.State
}

func parseEventResult(data []byte) *events.EventResult {
	result := &events.EventResult{Modifications: make(map[string]string)}
	if len(data) > 0 && data[0] == 1 {
		result.Cancelled = true
	}
	return result
}
