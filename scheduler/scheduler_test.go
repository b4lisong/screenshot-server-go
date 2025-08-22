package scheduler

import (
	"errors"
	"image"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockCapture creates a mock capture function for testing.
func mockCapture(shouldError bool) CaptureFunc {
	return func() (image.Image, error) {
		if shouldError {
			return nil, errors.New("mock capture error")
		}
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	}
}

// mockSave creates a mock save function for testing.
func mockSave(counter *int32, shouldError bool) SaveFunc {
	return func(img image.Image, isAutomatic bool) error {
		if shouldError {
			return errors.New("mock save error")
		}
		atomic.AddInt32(counter, 1)
		return nil
	}
}

// TestScheduler_StartStop tests basic start/stop functionality.
func TestScheduler_StartStop(t *testing.T) {
	var saveCount int32
	scheduler := New(
		mockCapture(false),
		mockSave(&saveCount, false),
	)

	// Start scheduler
	if err := scheduler.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	// Verify it's running
	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start()")
	}

	// Stop scheduler
	scheduler.Stop()

	// Verify it's stopped
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop()")
	}
}

// TestScheduler_DoubleStart tests that starting twice returns an error.
func TestScheduler_DoubleStart(t *testing.T) {
	scheduler := New(mockCapture(false), mockSave(nil, false))

	if err := scheduler.Start(); err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	defer scheduler.Stop()

	// Second start should fail
	if err := scheduler.Start(); err == nil {
		t.Error("second start should return an error")
	}
}

// TestScheduler_CalculateNextCapture tests the scheduling logic.
func TestScheduler_CalculateNextCapture(t *testing.T) {
	scheduler := New(mockCapture(false), mockSave(nil, false))

	tests := []struct {
		name    string
		now     time.Time
		wantMin time.Time
		wantMax time.Time
	}{
		{
			name:    "middle of hour",
			now:     time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			wantMin: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			wantMax: time.Date(2024, 1, 1, 15, 59, 59, 0, time.UTC),
		},
		{
			name:    "start of hour",
			now:     time.Date(2024, 1, 1, 14, 2, 0, 0, time.UTC),
			wantMin: time.Date(2024, 1, 1, 14, 2, 0, 0, time.UTC),
			wantMax: time.Date(2024, 1, 1, 14, 7, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a deterministic random number generator for testing
			rng := rand.New(rand.NewSource(1))
			next := scheduler.calculateNextCapture(tt.now, rng)

			if next.Before(tt.wantMin) || next.After(tt.wantMax) {
				t.Errorf("next capture %v not in range [%v, %v]",
					next, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestScheduler_ConcurrentStartStop tests thread safety of Start() and Stop().
// This test verifies that the race condition fix prevents concurrent issues.
func TestScheduler_ConcurrentStartStop(t *testing.T) {
	scheduler := New(mockCapture(false), mockSave(nil, false))

	const numGoroutines = 10
	const numIterations = 100

	// Run multiple goroutines concurrently calling Start() and Stop()
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Randomly start and stop
				if j%2 == 0 {
					scheduler.Start() // Ignore errors - we expect some
				} else {
					scheduler.Stop()
				}
			}
		}()
	}

	wg.Wait()

	// Ensure we end in a clean state
	scheduler.Stop()
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after final Stop()")
	}
}

// TestScheduler_MultipleStops tests that calling Stop() multiple times is safe.
func TestScheduler_MultipleStops(t *testing.T) {
	scheduler := New(mockCapture(false), mockSave(nil, false))

	// Start the scheduler
	if err := scheduler.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	// Call Stop() multiple times - should not panic or deadlock
	scheduler.Stop()
	scheduler.Stop() // Second stop should be no-op
	scheduler.Stop() // Third stop should be no-op

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after multiple Stop() calls")
	}
}

// TestScheduler_StartAfterStop tests that Start() works after Stop().
func TestScheduler_StartAfterStop(t *testing.T) {
	scheduler := New(mockCapture(false), mockSave(nil, false))

	// Start -> Stop -> Start cycle
	for i := 0; i < 3; i++ {
		if err := scheduler.Start(); err != nil {
			t.Fatalf("iteration %d: failed to start scheduler: %v", i, err)
		}

		if !scheduler.IsRunning() {
			t.Fatalf("iteration %d: scheduler should be running after Start()", i)
		}

		scheduler.Stop()

		if scheduler.IsRunning() {
			t.Fatalf("iteration %d: scheduler should not be running after Stop()", i)
		}
	}
}
