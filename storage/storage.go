// Package storage handles screenshot persistence and retrieval.
// It provides a file-based storage system with automatic organization
// and cleanup capabilities.
package storage

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Screenshot represents a captured screenshot with its metadata.
// In Go, we embed behavior (methods) with data (fields) in structs.
type Screenshot struct {
	// ID is the unique identifier (timestamp-based)
	ID string
	// Path is the absolute filesystem path
	Path string
	// CapturedAt is when the screenshot was taken
	CapturedAt time.Time
	// IsAutomatic indicates if this was an automatic capture
	IsAutomatic bool
}

// Storage defines the interface for screenshot storage operations.
//
// INTERFACE DESIGN PRINCIPLES IN ACTION:
// 1. Small interface: Only 4 methods, each with a single responsibility
// 2. Clear contracts: Each method has obvious input/output expectations
// 3. Error returns: Every method that can fail returns an error as the last value
// 4. Focused purpose: All methods relate to screenshot storage
//
// Any type that implements these 4 methods automatically satisfies this interface.
// This is Go's "implicit interface satisfaction" - no explicit declaration needed.
type Storage interface {
	// Save stores a screenshot and returns its metadata
	// Returns pointer to avoid copying large structs, error for failure cases
	Save(img image.Image, isAutomatic bool) (*Screenshot, error)

	// List returns recent screenshots, newest first
	// Takes limit to prevent unbounded memory usage, returns slice + error
	List(limit int) ([]*Screenshot, error)

	// Get retrieves a specific screenshot by ID
	// Returns pointer (nil if not found) and error for failure cases
	Get(id string) (*Screenshot, error)

	// Cleanup removes screenshots older than the specified duration
	// Returns error to report any cleanup failures (partial failures possible)
	Cleanup(olderThan time.Duration) error
}

// FileStorage implements Storage using the filesystem.
// The zero value is not usable - use NewFileStorage to create instances.
type FileStorage struct {
	// baseDir is the root directory for all screenshots
	baseDir string
}

// NewFileStorage creates a new file-based storage system.
//
// ERROR HANDLING PATTERNS DEMONSTRATED:
// 1. Early return on error - don't continue if path resolution fails
// 2. Error wrapping with %w - preserves original error for unwrapping
// 3. Contextual error messages - tells caller exactly what failed
// 4. Constructor pattern - returns concrete type, not interface
func NewFileStorage(baseDir string) (*FileStorage, error) {
	// Convert to absolute path for consistency
	// ERROR PATTERN 1: Check immediately and wrap with context
	absPath, err := filepath.Abs(baseDir)
	if err != nil {
		// %w wraps the error - caller can use errors.Is() or errors.Unwrap()
		// Message provides context about what operation failed
		return nil, fmt.Errorf("resolving base directory: %w", err)
	}

	// Create directory with read/write/execute permissions for owner only
	// 0750 = rwxr-x--- (owner: all, group: read/execute, others: none)
	// ERROR PATTERN 2: Same pattern - check and wrap
	if err := os.MkdirAll(absPath, 0750); err != nil {
		return nil, fmt.Errorf("creating base directory: %w", err)
	}

	// Success: return concrete type (not interface)
	return &FileStorage{baseDir: absPath}, nil
}

// Save implements the Storage interface for FileStorage.
//
// INTERFACE IMPLEMENTATION NOTES:
// - This method signature exactly matches Storage.Save()
// - No explicit "implements" declaration needed
// - Go compiler automatically recognizes FileStorage satisfies Storage interface
//
// COMPREHENSIVE ERROR HANDLING DEMONSTRATED:
// - Multiple failure points, each handled appropriately
// - Resource cleanup on failure (remove partial file)
// - Contextual error messages for debugging
func (fs *FileStorage) Save(img image.Image, isAutomatic bool) (*Screenshot, error) {
	now := time.Now()

	// Create directory structure: screenshots/2024/01/15/
	// This makes it easy to browse and clean up old files
	year := now.Format("2006")
	month := now.Format("01")
	day := now.Format("02")

	dir := filepath.Join(fs.baseDir, year, month, day)
	// ERROR HANDLING: Directory creation can fail (permissions, disk space, etc.)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("creating directory structure: %w", err)
	}

	// Generate unique filename with timestamp and type indicator
	// Format: 20240115_143052_auto.png or 20240115_143052_manual.png
	typeIndicator := "manual"
	if isAutomatic {
		typeIndicator = "auto"
	}

	// Use high precision timestamp for uniqueness even with rapid captures
	filename := fmt.Sprintf("%s_%s.png",
		now.Format("20060102_150405.000000000"),
		typeIndicator)

	fullPath := filepath.Join(dir, filename)

	// Create file with restricted permissions (owner read/write only)
	// os.O_EXCL ensures we fail if file already exists (prevents overwrites)
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	// ERROR HANDLING: File creation can fail for many reasons
	if err != nil {
		return nil, fmt.Errorf("creating screenshot file: %w", err)
	}
	// DEFER PATTERN: Ensure cleanup regardless of how function exits
	// This runs even if png.Encode fails or function panics
	defer file.Close()

	// Encode as PNG - idiomatic to use encoder directly
	if err := png.Encode(file, img); err != nil {
		// ERROR HANDLING WITH CLEANUP: If encoding fails, remove the partial file
		// We ignore the error from os.Remove because we're already handling a more important error
		os.Remove(fullPath)
		return nil, fmt.Errorf("encoding screenshot: %w", err)
	}

	// Success path: Create and return the Screenshot metadata
	screenshot := &Screenshot{
		ID:          now.Format("20060102_150405.000000000"),
		Path:        fullPath,
		CapturedAt:  now,
		IsAutomatic: isAutomatic,
	}

	return screenshot, nil
}

// List retrieves the most recent screenshots up to the specified limit.
// It walks the directory tree efficiently and sorts by timestamp.
func (fs *FileStorage) List(limit int) ([]*Screenshot, error) {
	var screenshots []*Screenshot

	// Walk the directory tree - this is more efficient than recursion
	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		// Handle walk errors gracefully
		if err != nil {
			// Log but continue walking
			return nil
		}

		// Skip directories and non-PNG files
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".png") {
			return nil
		}

		// Parse screenshot metadata from filename
		screenshot, err := fs.parseScreenshot(path, info)
		if err != nil {
			// Skip invalid files but continue
			return nil
		}

		screenshots = append(screenshots, screenshot)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	// Sort by captured time, newest first
	// This is idiomatic Go - define the sorting behavior inline
	sort.Slice(screenshots, func(i, j int) bool {
		return screenshots[i].CapturedAt.After(screenshots[j].CapturedAt)
	})

	// Apply limit
	if len(screenshots) > limit {
		screenshots = screenshots[:limit]
	}

	return screenshots, nil
}

// Get retrieves a specific screenshot by ID.
func (fs *FileStorage) Get(id string) (*Screenshot, error) {
	// Search for the file by walking the directory tree
	var found *Screenshot

	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if this file matches the ID
		if strings.Contains(info.Name(), id) {
			screenshot, err := fs.parseScreenshot(path, info)
			if err != nil {
				return nil
			}
			if screenshot.ID == id {
				found = screenshot
				// Stop walking once found
				return filepath.SkipDir
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("searching for screenshot: %w", err)
	}

	if found == nil {
		return nil, fmt.Errorf("screenshot with ID %s not found", id)
	}

	return found, nil
}

// Cleanup removes screenshots older than the specified duration.
//
// ADVANCED ERROR HANDLING PATTERNS:
// 1. Partial success handling - some operations can fail while others succeed
// 2. Error collection - gather multiple errors instead of failing fast
// 3. Graceful degradation - continue cleanup even if some files can't be removed
// 4. Error aggregation - report summary of failures to caller
func (fs *FileStorage) Cleanup(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	var cleanupErrors []error // PATTERN: Collect multiple errors

	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		// PATTERN: Handle walk errors gracefully - don't stop entire cleanup
		if err != nil || info.IsDir() {
			return nil // Continue walking despite individual file errors
		}

		// Parse the file to check its age
		screenshot, err := fs.parseScreenshot(path, info)
		if err != nil {
			// PATTERN: Skip invalid files but continue cleanup
			return nil
		}

		// Remove if older than cutoff
		if screenshot.CapturedAt.Before(cutoff) {
			if err := os.Remove(path); err != nil {
				// PATTERN: Collect error but don't fail entire operation
				// This allows cleanup to continue for other files
				cleanupErrors = append(cleanupErrors, fmt.Errorf("removing %s: %w", path, err))
			}
		}

		return nil
	})

	// ERROR HANDLING: Check if the walk itself failed
	if err != nil {
		return fmt.Errorf("walking directory during cleanup: %w", err)
	}

	// PATTERN: Aggregate multiple errors into single failure
	if len(cleanupErrors) > 0 {
		// Return error indicating partial failure
		// In production, you might log individual errors and return summary
		return fmt.Errorf("cleanup completed with %d errors", len(cleanupErrors))
	}

	// Also clean up empty directories
	// Note: We don't handle errors here because empty dir removal is optional
	fs.removeEmptyDirs()

	return nil // Success: all old files removed, no errors
}

// parseScreenshot extracts metadata from a screenshot file.
// This is a helper method that encapsulates the parsing logic.
func (fs *FileStorage) parseScreenshot(path string, info os.FileInfo) (*Screenshot, error) {
	// Extract filename without extension
	filename := strings.TrimSuffix(info.Name(), ".png")
	parts := strings.Split(filename, "_")

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid screenshot filename format %s: expected format YYYYMMDD_HHMMSS.nnnnnnnnn_type", filename)
	}

	// Parse timestamp from filename
	// Format: 20240115_143052.000000000_auto
	timeStr := parts[0] + "_" + parts[1]
	capturedAt, err := time.Parse("20060102_150405.000000000", timeStr)
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp %s from filename %s: %w", timeStr, filename, err)
	}

	// Determine if automatic
	isAutomatic := len(parts) > 2 && parts[2] == "auto"

	return &Screenshot{
		ID:          timeStr,
		Path:        path,
		CapturedAt:  capturedAt,
		IsAutomatic: isAutomatic,
	}, nil
}

// removeEmptyDirs cleans up empty directories after file cleanup.
// This keeps our storage directory tidy.
func (fs *FileStorage) removeEmptyDirs() {
	// Walk in reverse order to remove deepest directories first
	var dirs []string

	filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && path != fs.baseDir {
			dirs = append(dirs, path)
		}
		return nil
	})

	// Process in reverse order (deepest first)
	for i := len(dirs) - 1; i >= 0; i-- {
		// Try to remove - will fail if not empty (which is what we want)
		os.Remove(dirs[i])
	}
}

// ReadScreenshot loads a screenshot image from disk.
// This is a utility function for serving images.
func ReadScreenshot(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening screenshot file: %w", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decoding screenshot: %w", err)
	}

	return img, nil
}
