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
	capture CaptureFunc
	save    SaveFunc

	// Control channels for graceful shutdown
	stop    chan struct{}
	stopped chan struct{}

	// Mutex protects the entire state machine
	mu       sync.Mutex
	running  bool
	stopping bool // Prevents concurrent stops
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
// Thread-safe: can be called concurrently with Stop().
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running || s.stopping {
		return fmt.Errorf("scheduler is already running")
	}

	// Initialize fresh channels for this run
	// This handles the case where Stop() was called previously
	s.stop = make(chan struct{})
	s.stopped = make(chan struct{})
	s.running = true
	s.stopping = false

	go s.run()

	log.Println("Automatic screenshot scheduler started")
	return nil
}

// Stop gracefully shuts down the scheduler.
// It waits for any in-progress capture to complete.
// Thread-safe: can be called concurrently with Start() and multiple times safely.
func (s *Scheduler) Stop() {
	s.mu.Lock()

	// Early return if not running or already stopping
	if !s.running || s.stopping {
		s.mu.Unlock()
		return
	}

	// Mark as stopping to prevent concurrent stop operations
	s.stopping = true

	// Get channel references before releasing the mutex
	stopChan := s.stop
	stoppedChan := s.stopped
	s.mu.Unlock()

	// Signal stop and wait for goroutine to finish
	close(stopChan)
	<-stoppedChan

	// Update final state
	s.mu.Lock()
	s.running = false
	s.stopping = false
	s.mu.Unlock()

	log.Println("Automatic screenshot scheduler stopped")
}

// run is the main scheduler loop.
// It captures one screenshot per hour at random times.
func (s *Scheduler) run() {
	// Create local references to channels to avoid races with Start()/Stop()
	s.mu.Lock()
	stopChan := s.stop
	stoppedChan := s.stopped
	s.mu.Unlock()

	defer close(stoppedChan)

	// Create random number generator with modern approach
	// In production, you might use crypto/rand for better randomness
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Calculate time until next capture
	next := s.calculateNextCapture(time.Now(), rng)
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	log.Printf("Next automatic screenshot scheduled for %s", next.Format("15:04:05"))

	for {
		select {
		case <-timer.C:
			// Capture screenshot
			s.captureScreenshot()

			// Schedule next capture
			next = s.calculateNextCapture(time.Now(), rng)
			timer.Reset(time.Until(next))
			log.Printf("Next automatic screenshot scheduled for %s", next.Format("15:04:05"))

		case <-stopChan:
			// Graceful shutdown requested
			return
		}
	}
}

// calculateNextCapture determines when the next screenshot should be taken.
// It ensures one screenshot per hour at a random minute and second.
func (s *Scheduler) calculateNextCapture(now time.Time, rng *rand.Rand) time.Time {
	// Start with the beginning of the next hour
	next := now.Truncate(time.Hour).Add(time.Hour)

	// Add random minutes (0-59) and seconds (0-59)
	randomDuration := time.Duration(rng.Intn(60))*time.Minute +
		time.Duration(rng.Intn(60))*time.Second

	next = next.Add(randomDuration)

	// If we haven't had a screenshot this hour yet, schedule one soon
	thisHour := now.Truncate(time.Hour)
	if now.Sub(thisHour) < 5*time.Minute {
		// Schedule within the next 5 minutes if we just started the hour
		randomDuration = time.Duration(rng.Intn(300)) * time.Second
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
// Thread-safe: can be called concurrently with Start() and Stop().
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
