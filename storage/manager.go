package storage

import (
	"fmt"
	"image"
	"log"
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
// The result channel is unbuffered to ensure proper synchronization between
// the worker goroutine and the calling goroutine, preventing any potential leaks.
type command struct {
	op       string        // Operation type: "save", "list", "get", "cleanup"
	img      image.Image   // For save operations
	auto     bool          // For save operations
	id       string        // For get operations
	limit    int           // For list operations
	duration time.Duration // For cleanup operations
	result   chan result   // Unbuffered channel to send the result back
}

// result encapsulates the response from a command.
type result struct {
	screenshot  *Screenshot   // For save/get operations
	screenshots []*Screenshot // For list operations
	err         error         // Any error that occurred
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
// Uses unbuffered channels for result communication to ensure proper
// synchronization and prevent any potential goroutine leaks.
func (m *Manager) worker() {
	defer m.wg.Done()

	for cmd := range m.commands {
		var res result

		// Process command based on operation type
		switch cmd.op {
		case "save":
			if cmd.img == nil {
				res = result{err: fmt.Errorf("save operation failed: image cannot be nil")}
				break
			}
			screenshot, err := m.storage.Save(cmd.img, cmd.auto)
			if err != nil {
				err = fmt.Errorf("save operation failed (auto=%t): %w", cmd.auto, err)
			}
			res = result{screenshot: screenshot, err: err}

		case "list":
			if cmd.limit < 0 {
				res = result{err: fmt.Errorf("list operation failed: limit cannot be negative (got %d)", cmd.limit)}
				break
			}
			screenshots, err := m.storage.List(cmd.limit)
			if err != nil {
				err = fmt.Errorf("list operation failed (limit=%d): %w", cmd.limit, err)
			}
			res = result{screenshots: screenshots, err: err}

		case "get":
			if cmd.id == "" {
				res = result{err: fmt.Errorf("get operation failed: screenshot ID cannot be empty")}
				break
			}
			screenshot, err := m.storage.Get(cmd.id)
			if err != nil {
				err = fmt.Errorf("get operation failed (id=%q): %w", cmd.id, err)
			}
			res = result{screenshot: screenshot, err: err}

		case "cleanup":
			if cmd.duration < 0 {
				res = result{err: fmt.Errorf("cleanup operation failed: duration cannot be negative (got %v)", cmd.duration)}
				break
			}
			err := m.storage.Cleanup(cmd.duration)
			if err != nil {
				err = fmt.Errorf("cleanup operation failed (olderThan=%v): %w", cmd.duration, err)
			}
			res = result{err: err}

		default:
			// Provide helpful context about what operations are valid
			validOps := []string{"save", "list", "get", "cleanup"}
			res = result{err: fmt.Errorf("unknown storage operation %q: valid operations are %v", cmd.op, validOps)}
			// Log this as it indicates a programming error that should be investigated
			log.Printf("ERROR: Invalid storage operation attempted: %q (valid: %v)", cmd.op, validOps)
		}

		// Send result back through the command's result channel.
		// Using unbuffered channel ensures the worker blocks until the caller
		// is ready to receive, providing proper synchronization.
		// No need to close the channel as it's only used for a single send.
		cmd.result <- res
	}
}

// Save stores a screenshot through the manager.
// This method is safe to call from multiple goroutines.
// Uses unbuffered channel for result communication to ensure proper synchronization.
func (m *Manager) Save(img image.Image, isAutomatic bool) (*Screenshot, error) {
	// Validate input parameters
	if img == nil {
		return nil, fmt.Errorf("manager save operation failed: image cannot be nil")
	}

	// Create command with unbuffered result channel
	cmd := command{
		op:     "save",
		img:    img,
		auto:   isAutomatic,
		result: make(chan result), // Unbuffered for proper synchronization
	}

	// Send command and wait for result
	// The unbuffered channel ensures the worker and caller synchronize properly
	m.commands <- cmd
	res := <-cmd.result

	// Add additional context if operation failed
	if res.err != nil {
		return nil, fmt.Errorf("manager save operation failed: %w", res.err)
	}

	return res.screenshot, nil
}

// List retrieves recent screenshots through the manager.
// Uses unbuffered channel for result communication to ensure proper synchronization.
func (m *Manager) List(limit int) ([]*Screenshot, error) {
	// Validate input parameters
	if limit < 0 {
		return nil, fmt.Errorf("manager list operation failed: limit cannot be negative (got %d)", limit)
	}
	if limit == 0 {
		return []*Screenshot{}, nil // Return empty slice for zero limit
	}

	cmd := command{
		op:     "list",
		limit:  limit,
		result: make(chan result), // Unbuffered for proper synchronization
	}

	m.commands <- cmd
	res := <-cmd.result

	// Add additional context if operation failed
	if res.err != nil {
		return nil, fmt.Errorf("manager list operation failed: %w", res.err)
	}

	return res.screenshots, nil
}

// Get retrieves a specific screenshot through the manager.
// Uses unbuffered channel for result communication to ensure proper synchronization.
func (m *Manager) Get(id string) (*Screenshot, error) {
	// Validate input parameters
	if id == "" {
		return nil, fmt.Errorf("manager get operation failed: screenshot ID cannot be empty")
	}

	cmd := command{
		op:     "get",
		id:     id,
		result: make(chan result), // Unbuffered for proper synchronization
	}

	m.commands <- cmd
	res := <-cmd.result

	// Add additional context if operation failed
	if res.err != nil {
		return nil, fmt.Errorf("manager get operation failed: %w", res.err)
	}

	return res.screenshot, nil
}

// Cleanup removes old screenshots through the manager.
// Uses unbuffered channel for result communication to ensure proper synchronization.
func (m *Manager) Cleanup(olderThan time.Duration) error {
	// Validate input parameters
	if olderThan < 0 {
		return fmt.Errorf("manager cleanup operation failed: duration cannot be negative (got %v)", olderThan)
	}
	if olderThan == 0 {
		return fmt.Errorf("manager cleanup operation failed: duration cannot be zero (would delete all screenshots)")
	}

	cmd := command{
		op:       "cleanup",
		duration: olderThan,
		result:   make(chan result), // Unbuffered for proper synchronization
	}

	m.commands <- cmd
	res := <-cmd.result

	// Add additional context if operation failed
	if res.err != nil {
		return fmt.Errorf("manager cleanup operation failed: %w", res.err)
	}

	return nil
}

// Close shuts down the manager gracefully.
// Always call this when done to prevent goroutine leaks.
func (m *Manager) Close() {
	close(m.commands)
	m.wg.Wait()
}
