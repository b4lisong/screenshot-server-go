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
	op       string        // Operation type: "save", "list", "get", "cleanup"
	img      image.Image   // For save operations
	auto     bool          // For save operations
	id       string        // For get operations
	limit    int           // For list operations
	duration time.Duration // For cleanup operations
	result   chan result   // Channel to send the result back
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
