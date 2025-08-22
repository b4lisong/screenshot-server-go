// Package email provides daily summary scheduling functionality.
package email

import (
	"log"
	"sync"
	"time"

	"github.com/b4lisong/screenshot-server-go/config"
	"github.com/b4lisong/screenshot-server-go/storage"
)

// DailySummaryScheduler manages scheduled daily summary emails.
type DailySummaryScheduler struct {
	config    *config.Config
	storage   storage.Storage
	mailer    *Mailer
	serverInfo ServerInfo

	// Control channels for graceful shutdown
	stop    chan struct{}
	stopped chan struct{}

	// Mutex protects the state
	mu      sync.Mutex
	running bool
}

// NewDailySummaryScheduler creates a new daily summary scheduler.
func NewDailySummaryScheduler(config *config.Config, storage storage.Storage, mailer *Mailer, serverInfo ServerInfo) *DailySummaryScheduler {
	return &DailySummaryScheduler{
		config:     config,
		storage:    storage,
		mailer:     mailer,
		serverInfo: serverInfo,
		stop:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}
}

// Start begins the daily summary scheduling.
func (s *DailySummaryScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil // Already running
	}

	if !s.config.Email.Enabled || !s.config.Email.DailySummary {
		log.Println("Daily summary email scheduler disabled")
		return nil
	}

	// Initialize fresh channels for this run
	s.stop = make(chan struct{})
	s.stopped = make(chan struct{})
	s.running = true

	go s.run()

	log.Printf("Daily summary email scheduler started (sends at %s %s)", 
		s.config.Email.SummaryTime, s.config.Email.SummaryTimezone)
	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *DailySummaryScheduler) Stop() {
	s.mu.Lock()
	
	if !s.running {
		s.mu.Unlock()
		return
	}

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
	s.mu.Unlock()

	log.Println("Daily summary email scheduler stopped")
}

// run is the main scheduler loop.
func (s *DailySummaryScheduler) run() {
	// Create local references to channels to avoid races
	s.mu.Lock()
	stopChan := s.stop
	stoppedChan := s.stopped
	s.mu.Unlock()

	defer close(stoppedChan)

	// Calculate time until next summary
	next := s.calculateNextSummaryTime(time.Now())
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	log.Printf("Next daily summary scheduled for %s", next.Format("2006-01-02 15:04:05 MST"))

	for {
		select {
		case <-timer.C:
			// Send daily summary
			s.sendDailySummary(time.Now().Add(-24 * time.Hour)) // Summary for yesterday

			// Schedule next summary
			next = s.calculateNextSummaryTime(time.Now())
			timer.Reset(time.Until(next))
			log.Printf("Next daily summary scheduled for %s", next.Format("2006-01-02 15:04:05 MST"))

		case <-stopChan:
			// Graceful shutdown requested
			return
		}
	}
}

// calculateNextSummaryTime determines when the next summary should be sent.
func (s *DailySummaryScheduler) calculateNextSummaryTime(now time.Time) time.Time {
	// Parse the summary time
	summaryTime, err := time.Parse("15:04", s.config.Email.SummaryTime)
	if err != nil {
		// Fallback to 9:00 AM if parsing fails
		summaryTime, _ = time.Parse("15:04", "09:00")
		log.Printf("Invalid summary time format, using 09:00 as fallback: %v", err)
	}

	// Get the configured timezone
	location := s.config.GetSummaryLocation()

	// Calculate next occurrence
	now = now.In(location)
	next := time.Date(now.Year(), now.Month(), now.Day(),
		summaryTime.Hour(), summaryTime.Minute(), 0, 0, location)

	// If the time has already passed today, schedule for tomorrow
	if next.Before(now) || next.Equal(now) {
		next = next.Add(24 * time.Hour)
	}

	return next
}

// sendDailySummary sends a daily summary email for the specified date.
func (s *DailySummaryScheduler) sendDailySummary(summaryDate time.Time) {
	log.Printf("Generating daily summary for %s", summaryDate.Format("2006-01-02"))

	// Calculate date range for the summary (start of day to start of next day)
	location := s.config.GetSummaryLocation()
	startOfDay := time.Date(summaryDate.Year(), summaryDate.Month(), summaryDate.Day(),
		0, 0, 0, 0, location)
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Get screenshots for the day
	screenshots, err := s.storage.ListByDateRange(startOfDay, endOfDay)
	if err != nil {
		log.Printf("Failed to retrieve screenshots for daily summary: %v", err)
		return
	}

	// Send the summary email
	if err := s.mailer.SendDailySummary(s.serverInfo, screenshots, summaryDate); err != nil {
		log.Printf("Failed to send daily summary email: %v", err)
		return
	}

	log.Printf("Daily summary sent successfully for %s (%d screenshots)", 
		summaryDate.Format("2006-01-02"), len(screenshots))
}

// IsRunning returns whether the scheduler is currently active.
func (s *DailySummaryScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}