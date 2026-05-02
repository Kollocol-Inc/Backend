package safego

import (
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestGo_RecoversAndCounts(t *testing.T) {
	const location = "test.panic.location"

	before := counterValue(t, location)

	Go(location, func() {
		panic("boom")
	})

	waitForCounter(t, location, before+1)
}

func TestGo_NoPanic_DoesNotInc(t *testing.T) {
	const location = "test.normal.location"

	before := counterValue(t, location)

	var wg sync.WaitGroup
	wg.Add(1)
	Go(location, wg.Done)
	wg.Wait()

	after := counterValue(t, location)
	if got, want := after-before, 0.0; got != want {
		t.Fatalf("counter must not increment for clean run %q: got %v, want %v", location, got, want)
	}
}

func waitForCounter(t *testing.T, location string, want float64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if counterValue(t, location) >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("counter for %q did not reach %v in time (got %v)", location, want, counterValue(t, location))
}

func counterValue(t *testing.T, location string) float64 {
	t.Helper()

	c, err := goroutinePanics.GetMetricWithLabelValues(location)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%q): %v", location, err)
	}
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Counter == nil || m.Counter.Value == nil {
		return 0
	}
	return *m.Counter.Value
}
