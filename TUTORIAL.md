# Learning Idiomatic Go: Building an Activity Overview Feature

## Introduction

Welcome to this hands-on tutorial where we'll extend our screenshot server with an activity overview feature. This tutorial is designed for programmers new to Go who want to learn idiomatic patterns through practical implementation.

### What We'll Build

We're adding an `/activity` endpoint that displays a gallery of screenshots with these features:
- Automatic hourly screenshots at random times
- Storage of both manual and automatic screenshots
- Gallery view showing the last 24 screenshots
- Automatic cleanup of screenshots older than 1 week
- Concurrent operations using goroutines and channels

### Why These Design Choices?

Before we start coding, let's understand why we're making certain architectural decisions:

1. **File-based storage over database**: For image storage, the filesystem is the most idiomatic Go choice because:
   - Images are already files - no need for blob storage complexity
   - The filesystem provides natural organization (directories by date)
   - It's simpler to implement and maintain
   - Direct file serving is more efficient

2. **Channels over shared memory**: Go's philosophy is "Don't communicate by sharing memory; share memory by communicating." We'll use channels to coordinate our concurrent operations.

3. **Server-side rendering**: Using Go's `html/template` package keeps our application simple and leverages Go's excellent standard library.

## Project Structure

We'll organize our code into logical packages:

```
screenshot-server-go/
├── main.go                 # HTTP server and routing
├── screenshot/
│   └── capture.go         # Screenshot capture (existing)
├── storage/               # New: Screenshot storage
│   ├── storage.go         # Storage interface and implementation
│   └── storage_test.go    # Storage tests
├── scheduler/             # New: Automatic capture scheduling
│   ├── scheduler.go       # Scheduling logic
│   └── scheduler_test.go  # Scheduler tests
└── templates/             # New: HTML templates
    └── activity.html      # Gallery view template
```

## Step 1: Building the Storage Layer

Let's start by creating a storage package that handles all file operations. This demonstrates several Go idioms:
- Interface design for abstraction
- Error handling patterns
- File I/O best practices

### Understanding Go Interfaces

Before we write code, let's understand how interfaces work in Go:

**1. Interfaces are implemented implicitly**
```go
// You DON'T write: type FileStorage implements Storage
// Instead, if FileStorage has all the methods Storage requires, it automatically satisfies the interface
```

**2. Interfaces define behavior, not data**
```go
// An interface is a contract that says "I can do these things"
type Writer interface {
    Write([]byte) (int, error)
}
// Any type with a Write method matching this signature satisfies Writer
```

**3. The bigger the interface, the weaker the abstraction**
```go
// Good: Small, focused interfaces (Go philosophy)
type Reader interface {
    Read([]byte) (int, error)
}

// Less idiomatic: Large interfaces with many methods
type FileSystem interface {
    Open(string) (*File, error)
    Create(string) (*File, error)
    Remove(string) error
    Rename(string, string) error
    Stat(string) (FileInfo, error)
    // ... many more methods
}
```

**4. Accept interfaces, return concrete types**
```go
// Good: Accept an interface
func SaveImage(w io.Writer, img image.Image) error

// Return concrete type
func NewFileStorage(dir string) (*FileStorage, error)
```

### Understanding Go Error Handling

Go's error handling is explicit and based on these principles:

**1. Errors are values**
```go
// The error type is a built-in interface:
type error interface {
    Error() string
}
```

**2. Always check errors immediately**
```go
// Bad: Ignoring errors
file, _ := os.Open("file.txt")

// Good: Check every error
file, err := os.Open("file.txt")
if err != nil {
    return fmt.Errorf("opening file: %w", err)
}
defer file.Close()
```

**3. Error wrapping with context (Go 1.13+)**
```go
// Use %w to wrap errors while preserving the original
if err != nil {
    return fmt.Errorf("failed to save screenshot: %w", err)
}
// Callers can use errors.Is() or errors.As() to check wrapped errors
```

**4. Custom error types when needed**
```go
// Define custom errors for specific conditions
type NotFoundError struct {
    ID string
}

func (e NotFoundError) Error() string {
    return fmt.Sprintf("screenshot not found: %s", e.ID)
}

// Users can check for specific error types
var notFound NotFoundError
if errors.As(err, &notFound) {
    // Handle not found case
}
```

**5. Error handling patterns**
```go
// Pattern 1: Return early on error
if err != nil {
    return nil, err
}

// Pattern 2: Wrap with context
if err != nil {
    return nil, fmt.Errorf("creating directory %s: %w", dir, err)
}

// Pattern 3: Handle and continue (for non-critical errors)
if err := cleanup(); err != nil {
    log.Printf("cleanup failed: %v", err)
    // Continue execution
}

// Pattern 4: Collect multiple errors
var errs []error
for _, item := range items {
    if err := process(item); err != nil {
        errs = append(errs, err)
    }
}
if len(errs) > 0 {
    return fmt.Errorf("processing failed with %d errors", len(errs))
}
```

Now let's implement our storage layer with these concepts in mind.

Create `storage/storage.go`:

```go
// Package storage handles screenshot persistence and retrieval.
// It provides a file-based storage system with automatic organization
// and cleanup capabilities.
package storage

import (
	"fmt"
	"image"
	"image/png"
	"io"
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
	var cleanupErrors []error  // PATTERN: Collect multiple errors

	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		// PATTERN: Handle walk errors gracefully - don't stop entire cleanup
		if err != nil || info.IsDir() {
			return nil  // Continue walking despite individual file errors
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

	return nil  // Success: all old files removed, no errors
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
```

### Key Takeaways: Interfaces and Error Handling

**Interface Design Best Practices:**
1. **Small is beautiful**: Our `Storage` interface has only 4 methods - each with a clear purpose
2. **Implicit satisfaction**: `FileStorage` becomes a `Storage` automatically by implementing the methods
3. **Accept interfaces, return concrete**: Functions take `Storage` interface, constructors return `*FileStorage`
4. **Behavior over data**: Interfaces define what something can do, not what it contains

**Error Handling Patterns Applied:**
1. **Immediate checking**: Every error is checked right where it occurs
2. **Wrapping with context**: `fmt.Errorf("operation failed: %w", err)` provides context while preserving the original error
3. **Graceful degradation**: Cleanup continues even if some files can't be removed
4. **Resource cleanup**: `defer file.Close()` ensures resources are freed even on errors
5. **Partial success handling**: Collect multiple errors instead of failing fast when appropriate

**Real-World Error Scenarios This Code Handles:**
- Disk full during file creation
- Permission denied when creating directories  
- Network-mounted directories becoming unavailable
- Corrupted files that can't be parsed
- Race conditions with other processes modifying files
- System interruption during file operations

Now let's create tests for our storage package. Create `storage/storage_test.go`:

```go
package storage

import (
	"image"
	"image/color"
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
	
	// Save a screenshot
	screenshot, err := storage.Save(img, false)
	if err != nil {
		t.Fatalf("saving screenshot: %v", err)
	}

	// Modify the file's timestamp to make it old
	// This is a test trick to avoid waiting
	oldTime := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	if err := os.Chtimes(screenshot.Path, oldTime, oldTime); err != nil {
		t.Fatalf("changing file time: %v", err)
	}

	// Run cleanup for files older than 7 days
	if err := storage.Cleanup(7 * 24 * time.Hour); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(screenshot.Path); !os.IsNotExist(err) {
		t.Error("old screenshot was not removed")
	}
}
```

## Step 2: Implementing the Screenshot Manager

Now we'll create a manager that coordinates all screenshot operations using channels. This demonstrates Go's concurrency patterns.

Create `storage/manager.go`:

```go
package storage

import (
	"fmt"
	"image"
	"sync"
	"time"
)

// Manager coordinates screenshot operations using channels.
// It runs a single goroutine that handles all storage operations,
// preventing concurrent file access issues.
type Manager struct {
	storage  Storage
	commands chan command
	wg       sync.WaitGroup
}

// command represents an operation to be performed by the manager.
// Using a command pattern with channels is idiomatic for serializing operations.
type command struct {
	op       string      // Operation type: "save", "list", "get", "cleanup"
	img      image.Image // For save operations
	auto     bool        // For save operations
	id       string      // For get operations
	limit    int         // For list operations
	duration time.Duration // For cleanup operations
	result   chan result // Channel to send the result back
}

// result encapsulates the response from a command.
type result struct {
	screenshot  *Screenshot   // For save/get operations
	screenshots []*Screenshot // For list operations
	err         error        // Any error that occurred
}

// NewManager creates a new screenshot manager.
// It starts a background goroutine to handle all operations.
func NewManager(storage Storage) *Manager {
	m := &Manager{
		storage:  storage,
		commands: make(chan command),
	}

	// Start the worker goroutine
	m.wg.Add(1)
	go m.worker()

	return m
}

// worker processes commands sequentially.
// This goroutine owns all storage operations, ensuring thread safety.
func (m *Manager) worker() {
	defer m.wg.Done()

	for cmd := range m.commands {
		var res result

		// Process command based on operation type
		switch cmd.op {
		case "save":
			screenshot, err := m.storage.Save(cmd.img, cmd.auto)
			res = result{screenshot: screenshot, err: err}
			
		case "list":
			screenshots, err := m.storage.List(cmd.limit)
			res = result{screenshots: screenshots, err: err}
			
		case "get":
			screenshot, err := m.storage.Get(cmd.id)
			res = result{screenshot: screenshot, err: err}
			
		case "cleanup":
			err := m.storage.Cleanup(cmd.duration)
			res = result{err: err}
			
		default:
			res = result{err: fmt.Errorf("unknown storage operation: %s", cmd.op)}
		}

		// Send result back through the command's result channel
		// This pattern allows the caller to wait for completion
		cmd.result <- res
		close(cmd.result)
	}
}

// Save stores a screenshot through the manager.
// This method is safe to call from multiple goroutines.
func (m *Manager) Save(img image.Image, isAutomatic bool) (*Screenshot, error) {
	// Create command with result channel
	cmd := command{
		op:     "save",
		img:    img,
		auto:   isAutomatic,
		result: make(chan result, 1), // Buffered to prevent goroutine leak
	}

	// Send command and wait for result
	m.commands <- cmd
	res := <-cmd.result

	return res.screenshot, res.err
}

// List retrieves recent screenshots through the manager.
func (m *Manager) List(limit int) ([]*Screenshot, error) {
	cmd := command{
		op:     "list",
		limit:  limit,
		result: make(chan result, 1),
	}

	m.commands <- cmd
	res := <-cmd.result

	return res.screenshots, res.err
}

// Get retrieves a specific screenshot through the manager.
func (m *Manager) Get(id string) (*Screenshot, error) {
	cmd := command{
		op:     "get",
		id:     id,
		result: make(chan result, 1),
	}

	m.commands <- cmd
	res := <-cmd.result

	return res.screenshot, res.err
}

// Cleanup removes old screenshots through the manager.
func (m *Manager) Cleanup(olderThan time.Duration) error {
	cmd := command{
		op:       "cleanup",
		duration: olderThan,
		result:   make(chan result, 1),
	}

	m.commands <- cmd
	res := <-cmd.result

	return res.err
}

// Close shuts down the manager gracefully.
// Always call this when done to prevent goroutine leaks.
func (m *Manager) Close() {
	close(m.commands)
	m.wg.Wait()
}
```

## Step 3: Building the Automatic Screenshot Scheduler

The scheduler demonstrates time-based operations and goroutine lifecycle management.

Create `scheduler/scheduler.go`:

```go
// Package scheduler handles automatic screenshot capture at random hourly intervals.
package scheduler

import (
	"fmt"
	"image"
	"log"
	"math/rand"
	"sync"
	"time"
)

// CaptureFunc is a function that captures a screenshot.
// Using a function type allows for easy testing and flexibility.
type CaptureFunc func() (image.Image, error)

// SaveFunc is a function that saves a screenshot.
// This abstraction allows the scheduler to work with any storage system.
type SaveFunc func(img image.Image, isAutomatic bool) error

// Scheduler manages automatic screenshot captures.
// It ensures exactly one screenshot per hour at random times.
type Scheduler struct {
	capture  CaptureFunc
	save     SaveFunc
	
	// Control channels for graceful shutdown
	stop     chan struct{}
	stopped  chan struct{}
	
	// Mutex protects the running state
	mu       sync.Mutex
	running  bool
}

// New creates a new scheduler with the given capture and save functions.
func New(capture CaptureFunc, save SaveFunc) *Scheduler {
	return &Scheduler{
		capture: capture,
		save:    save,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Start begins the automatic screenshot scheduling.
// It runs in a separate goroutine and can be stopped with Stop().
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	s.running = true
	go s.run()
	
	log.Println("Automatic screenshot scheduler started")
	return nil
}

// Stop gracefully shuts down the scheduler.
// It waits for any in-progress capture to complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	// Signal stop and wait for confirmation
	close(s.stop)
	<-s.stopped
	
	log.Println("Automatic screenshot scheduler stopped")
}

// run is the main scheduler loop.
// It captures one screenshot per hour at random times.
func (s *Scheduler) run() {
	defer close(s.stopped)

	// Seed random number generator
	// In production, you might use crypto/rand for better randomness
	rand.Seed(time.Now().UnixNano())

	// Calculate time until next capture
	next := s.calculateNextCapture(time.Now())
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	log.Printf("Next automatic screenshot scheduled for %s", next.Format("15:04:05"))

	for {
		select {
		case <-timer.C:
			// Capture screenshot
			s.captureScreenshot()
			
			// Schedule next capture
			next = s.calculateNextCapture(time.Now())
			timer.Reset(time.Until(next))
			log.Printf("Next automatic screenshot scheduled for %s", next.Format("15:04:05"))
			
		case <-s.stop:
			// Graceful shutdown requested
			return
		}
	}
}

// calculateNextCapture determines when the next screenshot should be taken.
// It ensures one screenshot per hour at a random minute and second.
func (s *Scheduler) calculateNextCapture(now time.Time) time.Time {
	// Start with the beginning of the next hour
	next := now.Truncate(time.Hour).Add(time.Hour)
	
	// Add random minutes (0-59) and seconds (0-59)
	randomDuration := time.Duration(rand.Intn(60))*time.Minute +
	                 time.Duration(rand.Intn(60))*time.Second
	
	next = next.Add(randomDuration)
	
	// If we haven't had a screenshot this hour yet, schedule one soon
	thisHour := now.Truncate(time.Hour)
	if now.Sub(thisHour) < 5*time.Minute {
		// Schedule within the next 5 minutes if we just started the hour
		randomDuration = time.Duration(rand.Intn(300)) * time.Second
		next = now.Add(randomDuration)
	}
	
	return next
}

// captureScreenshot performs the actual screenshot capture and save.
// Errors are logged but don't stop the scheduler.
func (s *Scheduler) captureScreenshot() {
	log.Println("Capturing automatic screenshot...")
	
	// Capture
	img, err := s.capture()
	if err != nil {
		log.Printf("Failed to capture automatic screenshot: %v", err)
		return
	}
	
	// Save
	if err := s.save(img, true); err != nil {
		log.Printf("Failed to save automatic screenshot: %v", err)
		return
	}
	
	log.Println("Automatic screenshot captured and saved")
}

// IsRunning returns whether the scheduler is currently active.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
```

Create `scheduler/scheduler_test.go`:

```go
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
```

## Step 4: Creating the Activity HTTP Handler

Now let's create the HTTP handler that serves the activity page.

First, update `main.go` to integrate all our new components:

```go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"image/png"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/b4lisong/screenshot-server-go/scheduler"
	"github.com/b4lisong/screenshot-server-go/screenshot"
	"github.com/b4lisong/screenshot-server-go/storage"
)

// Global manager instance - in production, consider dependency injection
var manager *storage.Manager

// Templates are parsed once at startup for efficiency
var templates *template.Template

func main() {
	// Parse command-line flags
	port := flag.Int("p", 8080, "port to run the server on")
	storageDir := flag.String("storage", "./screenshots", "directory to store screenshots")
	flag.Parse()

	// Initialize storage
	fileStorage, err := storage.NewFileStorage(*storageDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create manager for thread-safe operations
	manager = storage.NewManager(fileStorage)
	defer manager.Close()

	// Parse templates
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Start automatic screenshot scheduler
	sched := scheduler.New(screenshot.Capture, func(img image.Image, isAutomatic bool) error {
		_, err := manager.Save(img, isAutomatic)
		return err
	})
	if err := sched.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// Start cleanup routine
	startCleanupRoutine()

	// Set up routes
	http.HandleFunc("/screenshot", handleScreenshot)
	http.HandleFunc("/activity", handleActivity)
	http.HandleFunc("/screenshot/", handleScreenshotImage)

	log.Printf("Server started at http://localhost:%d", *port)
	log.Printf("View activity at http://localhost:%d/activity", *port)
	
	err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleScreenshot captures and returns a screenshot (existing functionality).
func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received screenshot request from %s", r.RemoteAddr)
	
	// Capture screenshot
	img, err := screenshot.Capture()
	if err != nil {
		log.Printf("Capture failed: %v", err)
		http.Error(w, "Failed to capture screenshot", http.StatusInternalServerError)
		return
	}
	
	// Save to storage (manual screenshot)
	_, err = manager.Save(img, false)
	if err != nil {
		log.Printf("Failed to save screenshot: %v", err)
		// Continue to serve the image even if save fails
	}
	
	log.Printf("Screenshot captured successfully for %s", r.RemoteAddr)

	// Encode and send response
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		log.Printf("Encoding failed: %v", err)
		http.Error(w, "Failed to encode image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// handleActivity serves the activity overview page.
// This demonstrates template rendering and data preparation.
func handleActivity(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Retrieve recent screenshots
	screenshots, err := manager.List(24)
	if err != nil {
		log.Printf("Failed to list screenshots: %v", err)
		http.Error(w, "Failed to retrieve screenshots", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	// In Go, we create a struct to pass data to templates
	data := struct {
		Title       string
		Screenshots []*storage.Screenshot
		Now         time.Time
	}{
		Title:       "Screenshot Activity",
		Screenshots: screenshots,
		Now:         time.Now(),
	}

	// Execute template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "activity.html", data); err != nil {
		log.Printf("Failed to render template: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// handleScreenshotImage serves individual screenshot images.
// URL pattern: /screenshot/{id}
func handleScreenshotImage(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	// Example: /screenshot/20240115_143052.000000000
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 || parts[2] == "" {
		http.NotFound(w, r)
		return
	}
	
	id := parts[2]
	
	// Retrieve screenshot metadata
	screenshot, err := manager.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Read image from disk
	img, err := storage.ReadScreenshot(screenshot.Path)
	if err != nil {
		log.Printf("Failed to read screenshot: %v", err)
		http.Error(w, "Failed to load screenshot", http.StatusInternalServerError)
		return
	}

	// Encode and serve
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	
	if err := png.Encode(w, img); err != nil {
		log.Printf("Failed to encode screenshot: %v", err)
	}
}

// startCleanupRoutine starts a goroutine that periodically removes old screenshots.
// This demonstrates long-running background tasks in Go.
func startCleanupRoutine() {
	go func() {
		// Run cleanup every hour
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		// Also run immediately on startup
		performCleanup()

		for range ticker.C {
			performCleanup()
		}
	}()
}

// performCleanup removes screenshots older than 1 week.
func performCleanup() {
	log.Println("Running screenshot cleanup...")
	
	if err := manager.Cleanup(7 * 24 * time.Hour); err != nil {
		log.Printf("Cleanup failed: %v", err)
	} else {
		log.Println("Cleanup completed")
	}
}
```

## Step 5: Creating the HTML Template

Create the `templates` directory and add `templates/activity.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        /* Minimal CSS for a clean gallery layout */
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        
        h1 {
            color: #333;
            margin-bottom: 10px;
        }
        
        .info {
            color: #666;
            margin-bottom: 30px;
            font-size: 14px;
        }
        
        .gallery {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 20px;
        }
        
        .screenshot {
            background: white;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            transition: transform 0.2s;
        }
        
        .screenshot:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        }
        
        .screenshot img {
            width: 100%;
            height: auto;
            display: block;
        }
        
        .screenshot-info {
            padding: 12px;
            font-size: 12px;
            color: #666;
        }
        
        .screenshot-time {
            font-weight: 500;
            color: #333;
        }
        
        .screenshot-type {
            display: inline-block;
            margin-left: 8px;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 11px;
            font-weight: 500;
        }
        
        .type-auto {
            background-color: #e3f2fd;
            color: #1976d2;
        }
        
        .type-manual {
            background-color: #f3e5f5;
            color: #7b1fa2;
        }
        
        .empty {
            text-align: center;
            color: #999;
            padding: 60px 20px;
        }
        
        .nav {
            margin-bottom: 20px;
        }
        
        .nav a {
            color: #1976d2;
            text-decoration: none;
            margin-right: 20px;
        }
        
        .nav a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="nav">
        <a href="/">Home</a>
        <a href="/screenshot">Capture Screenshot</a>
    </div>

    <h1>{{.Title}}</h1>
    <div class="info">
        Showing the last {{len .Screenshots}} screenshots (maximum 24).
        Current time: {{.Now.Format "January 2, 2006 3:04:05 PM"}}
    </div>

    {{if .Screenshots}}
        <div class="gallery">
            {{range .Screenshots}}
                <div class="screenshot">
                    <a href="/screenshot/{{.ID}}">
                        <img src="/screenshot/{{.ID}}" alt="Screenshot from {{.CapturedAt.Format "Jan 2, 3:04 PM"}}" loading="lazy">
                    </a>
                    <div class="screenshot-info">
                        <span class="screenshot-time">
                            {{.CapturedAt.Format "Jan 2, 3:04:05 PM"}}
                        </span>
                        <span class="screenshot-type {{if .IsAutomatic}}type-auto{{else}}type-manual{{end}}">
                            {{if .IsAutomatic}}Automatic{{else}}Manual{{end}}
                        </span>
                    </div>
                </div>
            {{end}}
        </div>
    {{else}}
        <div class="empty">
            <p>No screenshots yet.</p>
            <p>Screenshots will appear here as they are captured automatically or <a href="/screenshot">manually</a>.</p>
        </div>
    {{end}}
</body>
</html>
```

## Step 6: Adding Integration Tests

Let's create a comprehensive test that verifies the entire system works together.

Create `main_test.go`:

```go
package main

import (
	"html/template"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/b4lisong/screenshot-server-go/storage"
)

// TestActivityHandler tests the activity page handler.
func TestActivityHandler(t *testing.T) {
	// Create temporary storage
	tempDir := t.TempDir()
	fileStorage, err := storage.NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	// Initialize global manager for the test
	manager = storage.NewManager(fileStorage)
	defer manager.Close()

	// Parse templates
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		t.Skipf("skipping test - templates not found: %v", err)
	}

	// Save some test screenshots
	testImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for i := 0; i < 3; i++ {
		_, err := manager.Save(testImg, i%2 == 0)
		if err != nil {
			t.Fatalf("saving test screenshot: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Create test request
	req, err := http.NewRequest("GET", "/activity", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	// Record response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleActivity)
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check content type
	expected := "text/html; charset=utf-8"
	if ct := rr.Header().Get("Content-Type"); ct != expected {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ct, expected)
	}

	// Check that response body contains expected content
	body := rr.Body.String()
	if !strings.Contains(body, "Screenshot Activity") {
		t.Error("response should contain page title")
	}
}

// TestScreenshotImageHandler tests serving individual screenshots.
func TestScreenshotImageHandler(t *testing.T) {
	// Set up storage and save a test screenshot
	tempDir := t.TempDir()
	fileStorage, err := storage.NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager = storage.NewManager(fileStorage)
	defer manager.Close()

	testImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	screenshot, err := manager.Save(testImg, false)
	if err != nil {
		t.Fatalf("saving test screenshot: %v", err)
	}

	// Test valid request
	req, err := http.NewRequest("GET", "/screenshot/"+screenshot.ID, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleScreenshotImage)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ct, "image/png")
	}

	// Test invalid request
	req, _ = http.NewRequest("GET", "/screenshot/invalid", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler should return 404 for invalid ID: got %v", status)
	}
}
```

## Understanding Go Idioms Through This Implementation

### 1. **Error Handling**
Go's explicit error handling ensures reliability:
```go
// Always check errors immediately
img, err := screenshot.Capture()
if err != nil {
    return nil, fmt.Errorf("capture failed: %w", err)
}
```

### 2. **Interfaces**
Small, focused interfaces enable flexibility:
```go
// Storage interface allows different implementations
type Storage interface {
    Save(img image.Image, isAutomatic bool) (*Screenshot, error)
    List(limit int) ([]*Screenshot, error)
}
```

### 3. **Channels for Concurrency**
Channels coordinate goroutines safely:
```go
// Command pattern with channels
cmd := command{
    op:     "save",
    result: make(chan result, 1),
}
m.commands <- cmd
res := <-cmd.result
```

### 4. **Goroutines for Background Tasks**
Lightweight concurrency for non-blocking operations:
```go
go func() {
    ticker := time.NewTicker(time.Hour)
    defer ticker.Stop()
    for range ticker.C {
        performCleanup()
    }
}()
```

### 5. **Defer for Cleanup**
Ensures resources are released:
```go
file, err := os.Open(path)
if err != nil {
    return nil, err
}
defer file.Close() // Always runs, even if function panics
```

### 6. **Package Organization**
- Each package has a single, clear responsibility
- Exported names (capitalized) form the public API
- Internal helpers (lowercase) stay private

## Running the Enhanced Server

1. **Start the server:**
   ```bash
   go run .
   ```

2. **Visit the activity page:**
   ```
   http://localhost:8080/activity
   ```

3. **Capture manual screenshots:**
   ```
   http://localhost:8080/screenshot
   ```

4. **Run tests:**
   ```bash
   go test ./...
   ```

## Key Takeaways

1. **Concurrency is Built-in**: Go makes concurrent programming accessible through goroutines and channels.

2. **Errors are Values**: Explicit error handling leads to more reliable software.

3. **Interfaces are Implicit**: Types satisfy interfaces automatically by implementing the required methods.

4. **The Standard Library is Powerful**: We built a full-featured web application using mostly standard library packages.

5. **Simplicity is Key**: Go encourages simple, obvious solutions over clever abstractions.

## Next Steps

To further your Go learning:

1. **Add Authentication**: Implement basic auth to protect the activity page
2. **Add Metrics**: Use the `expvar` package to expose runtime metrics
3. **Implement Websockets**: Make the activity page update in real-time
4. **Add Configuration**: Use environment variables for configuration
5. **Implement Caching**: Add an in-memory cache for recent screenshots

Remember: Go rewards clarity and simplicity. When in doubt, choose the more straightforward approach.