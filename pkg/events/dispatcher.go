package events

import (
	"context"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
)

type Handler func(ctx context.Context, data []byte) (*EventResult, error)

type Subscription struct {
	PluginID        string
	Priority        plugin.Priority
	Handler         Handler
	IgnoreCancelled bool
}

type EventResult struct {
	Cancelled     bool
	Modifications map[string]string
	Error         string
}

type Dispatcher struct {
	mu            sync.RWMutex
	subscriptions map[plugin.EventType][]Subscription
	logger        *zap.Logger
	eventCount    map[plugin.EventType]uint64
	cancelCount   map[plugin.EventType]uint64
	dispatchTimes map[plugin.EventType]time.Duration
}

func NewDispatcher(logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		subscriptions: make(map[plugin.EventType][]Subscription),
		logger:        logger,
		eventCount:    make(map[plugin.EventType]uint64),
		cancelCount:   make(map[plugin.EventType]uint64),
		dispatchTimes: make(map[plugin.EventType]time.Duration),
	}
}

func (d *Dispatcher) Subscribe(event plugin.EventType, sub Subscription) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.subscriptions[event] = append(d.subscriptions[event], sub)
	slices.SortFunc(d.subscriptions[event], func(a, b Subscription) int {
		return int(a.Priority - b.Priority)
	})
}

func (d *Dispatcher) Unsubscribe(pluginID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for event, subs := range d.subscriptions {
		d.subscriptions[event] = slices.DeleteFunc(subs, func(s Subscription) bool {
			return s.PluginID == pluginID
		})
	}
}

func (d *Dispatcher) UnsubscribeEvent(pluginID string, event plugin.EventType) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.subscriptions[event] = slices.DeleteFunc(d.subscriptions[event], func(s Subscription) bool {
		return s.PluginID == pluginID
	})
}

type DispatchResult struct {
	Cancelled     bool
	Modifications map[string]string
	Handlers      int
	Duration      time.Duration
	Errors        []error
}

type EventData interface {
	String() string
}

func (d *Dispatcher) Dispatch(ctx context.Context, event plugin.EventType, data EventData) (*DispatchResult, error) {
	d.mu.RLock()
	subs := slices.Clone(d.subscriptions[event])
	d.mu.RUnlock()

	if len(subs) == 0 {
		return &DispatchResult{}, nil
	}

	start := time.Now()
	serialized := []byte(data.String())
	result := &DispatchResult{Modifications: make(map[string]string)}

	for _, sub := range subs {
		if result.Cancelled && sub.IgnoreCancelled {
			continue
		}

		handlerResult, err := sub.Handler(ctx, serialized)
		if err != nil {
			result.Errors = append(result.Errors, err)
			d.logger.Error("event handler error",
				zap.String("plugin", sub.PluginID),
				zap.String("event", string(event)),
				zap.Error(err),
			)
			continue
		}

		result.Handlers++
		if handlerResult != nil {
			if handlerResult.Cancelled {
				result.Cancelled = true
			}
			for k, v := range handlerResult.Modifications {
				result.Modifications[k] = v
			}
		}
	}

	result.Duration = time.Since(start)

	d.mu.Lock()
	d.eventCount[event]++
	if result.Cancelled {
		d.cancelCount[event]++
	}
	d.dispatchTimes[event] += result.Duration
	d.mu.Unlock()

	return result, nil
}

func (d *Dispatcher) DispatchAsync(ctx context.Context, event plugin.EventType, data EventData) <-chan *DispatchResult {
	ch := make(chan *DispatchResult, 1)
	go func() {
		result, err := d.Dispatch(ctx, event, data)
		if err != nil {
			result = &DispatchResult{Errors: []error{err}}
		}
		ch <- result
		close(ch)
	}()
	return ch
}

func (d *Dispatcher) HasSubscribers(event plugin.EventType) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.subscriptions[event]) > 0
}

func (d *Dispatcher) SubscriberCount(event plugin.EventType) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.subscriptions[event])
}

type EventMetrics struct {
	EventType     plugin.EventType
	TotalCount    uint64
	CancelCount   uint64
	TotalDuration time.Duration
}

func (d *Dispatcher) GetMetrics() []EventMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()

	metrics := make([]EventMetrics, 0, len(d.eventCount))
	for event := range d.eventCount {
		metrics = append(metrics, EventMetrics{
			EventType:     event,
			TotalCount:    d.eventCount[event],
			CancelCount:   d.cancelCount[event],
			TotalDuration: d.dispatchTimes[event],
		})
	}
	return metrics
}

func (d *Dispatcher) ResetMetrics() {
	d.mu.Lock()
	defer d.mu.Unlock()

	clear(d.eventCount)
	clear(d.cancelCount)
	clear(d.dispatchTimes)
}
