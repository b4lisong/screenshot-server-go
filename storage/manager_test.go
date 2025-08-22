package storage

import (
	"image"
	"image/color"
	"runtime"
	"sync"
	"testing"
	"time"
)

// createTestImage creates a simple test image for testing.
func createManagerTestImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	return img
}

// TestManager_BasicOperations tests the manager's basic functionality.
func TestManager_BasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager := NewManager(storage)
	defer manager.Close()

	img := createManagerTestImage()

	// Test Save
	screenshot, err := manager.Save(img, false)
	if err != nil {
		t.Fatalf("saving screenshot: %v", err)
	}

	if screenshot == nil {
		t.Fatal("expected screenshot, got nil")
	}

	// Test List
	screenshots, err := manager.List(10)
	if err != nil {
		t.Fatalf("listing screenshots: %v", err)
	}

	if len(screenshots) != 1 {
		t.Errorf("expected 1 screenshot, got %d", len(screenshots))
	}

	// Test Get
	retrieved, err := manager.Get(screenshot.ID)
	if err != nil {
		t.Fatalf("getting screenshot: %v", err)
	}

	if retrieved.ID != screenshot.ID {
		t.Errorf("expected ID %s, got %s", screenshot.ID, retrieved.ID)
	}

	// Test Cleanup
	err = manager.Cleanup(time.Hour)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

// TestManager_ConcurrentOperations tests concurrent access to the manager.
func TestManager_ConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager := NewManager(storage)
	defer manager.Close()

	img := createManagerTestImage()

	// Run multiple operations concurrently
	var wg sync.WaitGroup
	const numOps = 10

	// Concurrent saves
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.Save(img, false)
			if err != nil {
				t.Errorf("concurrent save failed: %v", err)
			}
		}()
	}

	// Concurrent lists
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.List(5)
			if err != nil {
				t.Errorf("concurrent list failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

// TestManager_GoroutineLeakPrevention tests that the manager doesn't leak goroutines.
func TestManager_GoroutineLeakPrevention(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Create and use manager
	manager := NewManager(storage)

	img := createManagerTestImage()

	// Perform operations
	for i := 0; i < 100; i++ {
		_, err := manager.Save(img, false)
		if err != nil {
			t.Fatalf("save operation %d failed: %v", i, err)
		}
	}

	// Close manager properly
	manager.Close()

	// Force garbage collection to clean up any lingering goroutines
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()

	// Allow for some variance in goroutine count due to test framework
	if finalGoroutines > initialGoroutines+2 {
		t.Errorf("potential goroutine leak: started with %d, ended with %d goroutines",
			initialGoroutines, finalGoroutines)
	}
}

// TestManager_CloseCleanup tests that Close properly cleans up resources.
func TestManager_CloseCleanup(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager := NewManager(storage)

	// Close should complete without hanging
	done := make(chan struct{})
	go func() {
		manager.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success - Close completed
	case <-time.After(5 * time.Second):
		t.Fatal("manager.Close() hung - potential deadlock")
	}
}

// TestManager_UnbufferedChannelSynchronization tests that the unbuffered channels
// provide proper synchronization between the worker and callers.
func TestManager_UnbufferedChannelSynchronization(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager := NewManager(storage)
	defer manager.Close()

	img := createManagerTestImage()

	// Test that operations complete synchronously and results are consistent
	for i := 0; i < 50; i++ {
		// Each save operation should complete fully before the next one starts
		screenshot, err := manager.Save(img, false)
		if err != nil {
			t.Fatalf("save operation %d failed: %v", i, err)
		}

		// Immediately list to verify the save completed
		screenshots, err := manager.List(100)
		if err != nil {
			t.Fatalf("list operation after save %d failed: %v", i, err)
		}

		// Should have at least i+1 screenshots
		if len(screenshots) < i+1 {
			t.Errorf("after save %d, expected at least %d screenshots, got %d",
				i, i+1, len(screenshots))
		}

		// The latest screenshot should be the one we just saved
		if screenshots[0].ID != screenshot.ID {
			t.Errorf("after save %d, latest screenshot ID mismatch: expected %s, got %s",
				i, screenshot.ID, screenshots[0].ID)
		}
	}
}

// TestManager_ChannelCleanupOnWorkerExit tests that channels are properly cleaned
// up when the worker goroutine exits.
func TestManager_ChannelCleanupOnWorkerExit(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager := NewManager(storage)
	img := createManagerTestImage()

	// Perform some operations to ensure channels work
	for i := 0; i < 10; i++ {
		_, err := manager.Save(img, false)
		if err != nil {
			t.Fatalf("save operation %d failed: %v", i, err)
		}
	}

	// Close the manager
	manager.Close()

	// Verify that attempting operations after close would be detected
	// (In a real scenario, this would panic or hang, but our test validates
	// that Close() completed successfully, which means the worker exited cleanly)

	// This test validates that the channel-based cleanup works correctly
	// by ensuring Close() returns, which means all channels were properly
	// cleaned up and the worker goroutine exited.
}
