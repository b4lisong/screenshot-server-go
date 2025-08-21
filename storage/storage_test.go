package storage

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestImage creates a simple test image.
// In Go tests, helper functions conventionally start with lowercase.
func createTestImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// Fill with solid color for testing
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	return img
}

// TestFileStorage_Save tests the Save method.
// Test function names must start with Test and take *testing.T.
func TestFileStorage_Save(t *testing.T) {
	// Create temporary directory for test
	// t.TempDir() is cleaned up automatically after the test
	tempDir := t.TempDir()

	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	// Table-driven tests are idiomatic in Go
	tests := []struct {
		name        string
		isAutomatic bool
		wantType    string
	}{
		{
			name:        "manual screenshot",
			isAutomatic: false,
			wantType:    "manual",
		},
		{
			name:        "automatic screenshot",
			isAutomatic: true,
			wantType:    "auto",
		},
	}

	for _, tt := range tests {
		// Use t.Run for sub-tests - provides better test output
		t.Run(tt.name, func(t *testing.T) {
			img := createTestImage()

			screenshot, err := storage.Save(img, tt.isAutomatic)
			if err != nil {
				t.Fatalf("saving screenshot: %v", err)
			}

			// Verify the file exists
			if _, err := os.Stat(screenshot.Path); err != nil {
				t.Errorf("screenshot file not found: %v", err)
			}

			// Verify metadata
			if screenshot.IsAutomatic != tt.isAutomatic {
				t.Errorf("IsAutomatic = %v, want %v", screenshot.IsAutomatic, tt.isAutomatic)
			}

			// Verify filename contains type indicator
			filename := filepath.Base(screenshot.Path)
			if !strings.Contains(filename, tt.wantType) {
				t.Errorf("filename %s doesn't contain type indicator %s", filename, tt.wantType)
			}
		})
	}
}

// TestFileStorage_List tests the List method.
func TestFileStorage_List(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	// Create multiple screenshots
	img := createTestImage()
	for i := 0; i < 5; i++ {
		_, err := storage.Save(img, i%2 == 0) // Alternate between auto and manual
		if err != nil {
			t.Fatalf("saving screenshot %d: %v", i, err)
		}
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Test listing with limit
	screenshots, err := storage.List(3)
	if err != nil {
		t.Fatalf("listing screenshots: %v", err)
	}

	if len(screenshots) != 3 {
		t.Errorf("List(3) returned %d screenshots, want 3", len(screenshots))
	}

	// Verify order (newest first)
	for i := 1; i < len(screenshots); i++ {
		if screenshots[i-1].CapturedAt.Before(screenshots[i].CapturedAt) {
			t.Error("screenshots not sorted newest first")
		}
	}
}

// TestFileStorage_Cleanup tests the Cleanup method.
func TestFileStorage_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	img := createTestImage()

	// Create an old screenshot by manually creating a file with an old timestamp in the name
	oldTime := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	oldDir := filepath.Join(tempDir, oldTime.Format("2006"), oldTime.Format("01"), oldTime.Format("02"))
	if err := os.MkdirAll(oldDir, 0750); err != nil {
		t.Fatalf("creating old directory: %v", err)
	}

	oldFilename := fmt.Sprintf("%s_manual.png", oldTime.Format("20060102_150405.000000000"))
	oldPath := filepath.Join(oldDir, oldFilename)

	file, err := os.Create(oldPath)
	if err != nil {
		t.Fatalf("creating old screenshot file: %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encoding old screenshot: %v", err)
	}

	// Save a recent screenshot (should not be removed)
	recentScreenshot, err := storage.Save(img, false)
	if err != nil {
		t.Fatalf("saving recent screenshot: %v", err)
	}

	// Run cleanup for files older than 7 days
	if err := storage.Cleanup(7 * 24 * time.Hour); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify old file was removed
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old screenshot was not removed")
	}

	// Verify recent file still exists
	if _, err := os.Stat(recentScreenshot.Path); err != nil {
		t.Error("recent screenshot was incorrectly removed")
	}
}
