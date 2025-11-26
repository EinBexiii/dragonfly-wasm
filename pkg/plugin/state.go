package plugin

import (
	"maps"
	"sync"
	"time"
)

type State int

const (
	StateUnloaded State = iota
	StateLoading
	StateLoaded
	StateEnabling
	StateEnabled
	StateDisabling
	StateDisabled
	StateError
)

func (s State) String() string {
	names := [...]string{"unloaded", "loading", "loaded", "enabling", "enabled", "disabling", "disabled", "error"}
	if int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

type Metrics struct {
	mu                   sync.RWMutex
	TotalCalls           uint64
	TotalExecutionTime   time.Duration
	AverageExecutionTime time.Duration
	MaxExecutionTime     time.Duration
	LastExecutionTime    time.Duration
	MemoryUsageBytes     uint64
	PeakMemoryBytes      uint64
	EventsHandled        map[EventType]uint64
	EventsCancelled      map[EventType]uint64
	ErrorCount           uint64
	LastError            string
	LastErrorTime        time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		EventsHandled:   make(map[EventType]uint64),
		EventsCancelled: make(map[EventType]uint64),
	}
}

func (m *Metrics) RecordCall(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalCalls++
	m.TotalExecutionTime += duration
	m.LastExecutionTime = duration
	if duration > m.MaxExecutionTime {
		m.MaxExecutionTime = duration
	}
	m.AverageExecutionTime = m.TotalExecutionTime / time.Duration(m.TotalCalls)
}

func (m *Metrics) RecordEvent(event EventType, cancelled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EventsHandled[event]++
	if cancelled {
		m.EventsCancelled[event]++
	}
}

func (m *Metrics) RecordError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ErrorCount++
	m.LastError = err.Error()
	m.LastErrorTime = time.Now()
}

func (m *Metrics) RecordMemory(bytes uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MemoryUsageBytes = bytes
	if bytes > m.PeakMemoryBytes {
		m.PeakMemoryBytes = bytes
	}
}

func (m *Metrics) Snapshot() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Metrics{
		TotalCalls:           m.TotalCalls,
		TotalExecutionTime:   m.TotalExecutionTime,
		AverageExecutionTime: m.AverageExecutionTime,
		MaxExecutionTime:     m.MaxExecutionTime,
		LastExecutionTime:    m.LastExecutionTime,
		MemoryUsageBytes:     m.MemoryUsageBytes,
		PeakMemoryBytes:      m.PeakMemoryBytes,
		ErrorCount:           m.ErrorCount,
		LastError:            m.LastError,
		LastErrorTime:        m.LastErrorTime,
		EventsHandled:        maps.Clone(m.EventsHandled),
		EventsCancelled:      maps.Clone(m.EventsCancelled),
	}
}

type Info struct {
	Manifest   *Manifest
	State      State
	Metrics    *Metrics
	LoadedAt   time.Time
	EnabledAt  time.Time
	DisabledAt time.Time
	DataPath   string
	WASMPath   string
}

func NewInfo(manifest *Manifest, wasmPath, dataPath string) *Info {
	return &Info{
		Manifest: manifest,
		State:    StateUnloaded,
		Metrics:  NewMetrics(),
		WASMPath: wasmPath,
		DataPath: dataPath,
	}
}
