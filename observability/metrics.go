package observability

import (
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	start        time.Time
	count        int64
	totalLatency int64
	mu           sync.Mutex
	fastest      time.Duration
	slowest      time.Duration
}

func NewMetrics() *Metrics {
	return &Metrics{
		start:   time.Now(),
		fastest: time.Duration(1<<63 - 1),
		slowest: 0,
	}
}

func (m *Metrics) Record(latency time.Duration) {
	atomic.AddInt64(&m.count, 1)
	atomic.AddInt64(&m.totalLatency, int64(latency))
	m.mu.Lock()
	defer m.mu.Unlock()
	if latency < m.fastest {
		m.fastest = latency
	}
	if latency > m.slowest {
		m.slowest = latency
	}
}

func (m *Metrics) Count() int64 {
	return atomic.LoadInt64(&m.count)
}

func (m *Metrics) QPS() float64 {
	duration := time.Since(m.start).Seconds()
	if duration <= 0 {
		return 0
	}
	return float64(m.Count()) / duration
}

func (m *Metrics) AvgLatency() time.Duration {
	count := m.Count()
	if count == 0 {
		return 0
	}
	return time.Duration(atomic.LoadInt64(&m.totalLatency) / count)
}

func (m *Metrics) Fastest() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fastest == time.Duration(1<<63-1) {
		return 0
	}
	return m.fastest
}

func (m *Metrics) Slowest() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.slowest
}

func (m *Metrics) Total() int64 {
	return m.Count()
}
