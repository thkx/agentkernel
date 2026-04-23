package observability

import (
	"testing"
	"time"
)

func TestMetrics_RecordAndGet(t *testing.T) {
	metrics := NewMetrics()

	// Record some latencies
	metrics.Record(100 * time.Millisecond)
	metrics.Record(200 * time.Millisecond)
	metrics.Record(150 * time.Millisecond)

	// Check QPS (should be calculated based on time)
	qps := metrics.QPS()
	if qps < 0 {
		t.Errorf("QPS should be non-negative, got %f", qps)
	}

	// Check average latency
	avg := metrics.AvgLatency()
	expectedAvg := 150 * time.Millisecond // (100 + 200 + 150) / 3
	if avg != expectedAvg {
		t.Errorf("expected average latency %v, got %v", expectedAvg, avg)
	}

	// Check min/max
	if metrics.Fastest() != 100*time.Millisecond {
		t.Errorf("expected fastest 100ms, got %v", metrics.Fastest())
	}

	if metrics.Slowest() != 200*time.Millisecond {
		t.Errorf("expected slowest 200ms, got %v", metrics.Slowest())
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	metrics := NewMetrics()

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			metrics.Record(time.Duration(i) * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = metrics.QPS()
			_ = metrics.AvgLatency()
		}
		done <- true
	}()

	<-done
	<-done

	// If we get here without issues, basic concurrency works
}