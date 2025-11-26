package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/EinBexiii/dragonfly-wasm/pkg/config"
	"github.com/EinBexiii/dragonfly-wasm/pkg/host"
	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
	"go.uber.org/zap"
)

var (
	ErrPoolExhausted = errors.New("instance pool exhausted")
	ErrPoolClosed    = errors.New("instance pool is closed")
)

type Pool struct {
	mu        sync.Mutex
	manifest  *plugin.Manifest
	wasmBytes []byte
	config    *config.Config
	hostFuncs *host.FunctionProvider
	logger    *zap.Logger
	instances chan *Instance
	size      int
	created   int
	closed    bool
}

func NewPool(manifest *plugin.Manifest, wasmBytes []byte, cfg *config.Config, hostFuncs *host.FunctionProvider, logger *zap.Logger, size int) (*Pool, error) {
	if size <= 0 {
		size = cfg.Performance.PoolSize
	}

	pool := &Pool{
		manifest:  manifest,
		wasmBytes: wasmBytes,
		config:    cfg,
		hostFuncs: hostFuncs,
		logger:    logger,
		instances: make(chan *Instance, size),
		size:      size,
	}

	for i := 0; i < size; i++ {
		inst, err := pool.createInstance()
		if err != nil {
			pool.Close()
			return nil, err
		}
		pool.instances <- inst
		pool.created++
	}
	return pool, nil
}

func (p *Pool) createInstance() (*Instance, error) {
	info := plugin.NewInfo(p.manifest, "", "")
	info.State = plugin.StateLoaded
	info.LoadedAt = time.Now()
	return NewInstance(info, p.wasmBytes, p.config, p.hostFuncs, p.logger)
}

func (p *Pool) Acquire(ctx context.Context) (*Instance, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p.mu.Unlock()

	select {
	case inst := <-p.instances:
		return inst, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, ErrPoolExhausted
	}
}

func (p *Pool) AcquireWait(ctx context.Context) (*Instance, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p.mu.Unlock()

	select {
	case inst := <-p.instances:
		return inst, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pool) Release(inst *Instance) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		inst.Close()
		return
	}
	p.mu.Unlock()

	select {
	case p.instances <- inst:
	default:
		inst.Close()
	}
}

func (p *Pool) Size() int      { return p.size }
func (p *Pool) Available() int { return len(p.instances) }

func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	close(p.instances)
	for inst := range p.instances {
		inst.Close()
	}
	return nil
}

func (p *Pool) WithInstance(ctx context.Context, fn func(*Instance) error) error {
	inst, err := p.AcquireWait(ctx)
	if err != nil {
		return err
	}
	defer p.Release(inst)
	return fn(inst)
}
