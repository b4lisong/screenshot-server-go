package scheduler

import (
	"errors"
	"image"
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
			next := scheduler.calculateNextCapture(tt.now)

			if next.Before(tt.wantMin) || next.After(tt.wantMax) {
				t.Errorf("next capture %v not in range [%v, %v]",
					next, tt.wantMin, tt.wantMax)
			}
		})
	}
}
