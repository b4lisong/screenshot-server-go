// Package healthcheck provides production-ready health monitoring with ping functionality.
package healthcheck

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Monitor manages the lifecycle and execution of periodic health check pings.
// It provides thread-safe operations and graceful shutdown capabilities for production use.
type Monitor struct {
	client *Client
	config *Config

	// Control channels for lifecycle management
	ctx         context.Context
	cancel      context.CancelFunc
	stopChan    chan struct{}
	stoppedChan chan struct{}

	// State management with mutex protection
	mu      sync.Mutex
	running bool
	stopped bool

	// Statistics tracking
	stats MonitorStats
}

// MonitorStats tracks operational statistics for the health monitor.
type MonitorStats struct {
	// StartTime when monitoring began
	StartTime time.Time

	// TotalPings is the total number of ping attempts
	TotalPings int64

	// SuccessfulPings is the number of successful pings
	SuccessfulPings int64

	// FailedPings is the number of failed pings
	FailedPings int64

	// LastPingTime is when the most recent ping was performed
	LastPingTime time.Time

	// LastPingSuccess indicates if the most recent ping was successful
	LastPingSuccess bool

	// LastPingDuration is the response time of the most recent ping
	LastPingDuration time.Duration

	// ConsecutiveFailures tracks current consecutive failure count
	ConsecutiveFailures int64
}

// NewMonitor creates a new health check monitor with the specified configuration.
// It initializes all components but does not start monitoring until Start() is called.
func NewMonitor(config *Config) (*Monitor, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create HTTP client for ping operations
	client, err := NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	monitor := &Monitor{
		client:      client,
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		stats: MonitorStats{
			StartTime: time.Now(),
		},
	}

	return monitor, nil
}

// Start begins the periodic health check monitoring in a separate goroutine.
// It is thread-safe and can be called multiple times safely (subsequent calls are ignored).
func (m *Monitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running or stopped
	if m.running {
		return fmt.Errorf("monitor is already running")
	}
	if m.stopped {
		return fmt.Errorf("monitor has been stopped and cannot be restarted")
	}

	// Check if healthcheck is enabled
	if !m.config.IsEnabled() {
		log.Println("Healthcheck monitoring is disabled - not starting")
		return nil
	}

	// Update state
	m.running = true
	m.stats.StartTime = time.Now()

	// Start monitoring goroutine
	go m.monitorLoop()

	log.Printf("Healthcheck monitor started: %s", m.config.String())
	return nil
}

// Stop gracefully shuts down the health check monitor.
// It waits for any in-progress ping to complete before returning.
// It is thread-safe and can be called multiple times safely.
func (m *Monitor) Stop() {
	m.mu.Lock()

	// Check if not running
	if !m.running {
		m.mu.Unlock()
		return
	}

	// Mark as stopping
	m.running = false
	m.stopped = true
	m.mu.Unlock()

	// Signal shutdown and wait
	close(m.stopChan)
	<-m.stoppedChan

	// Cancel context and cleanup
	m.cancel()
	m.client.Close()

	log.Println("Healthcheck monitor stopped")
}

// monitorLoop is the main monitoring goroutine that performs periodic health checks.
func (m *Monitor) monitorLoop() {
	defer close(m.stoppedChan)

	// Create ticker for periodic pings
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	// Perform initial ping immediately
	m.performPing()

	log.Printf("Healthcheck monitoring started with %v interval", m.config.Interval)

	for {
		select {
		case <-ticker.C:
			// Perform scheduled ping
			m.performPing()

		case <-m.stopChan:
			// Graceful shutdown requested
			log.Println("Healthcheck monitor shutdown requested")
			return

		case <-m.ctx.Done():
			// Context cancelled
			log.Println("Healthcheck monitor context cancelled")
			return
		}
	}
}

// performPing executes a health check ping and updates statistics.
func (m *Monitor) performPing() {
	log.Println("Performing healthcheck ping...")

	// Create timeout context for this ping
	pingCtx, cancel := context.WithTimeout(m.ctx, m.config.Timeout)
	defer cancel()

	// Execute ping with retry logic
	result, err := m.client.Ping(pingCtx)

	// Update statistics
	m.updateStats(result, err)

	// Log results
	m.logPingResult(result, err)
}

// updateStats updates the monitor's operational statistics based on ping results.
func (m *Monitor) updateStats(result *PingResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update basic counters
	m.stats.TotalPings++
	m.stats.LastPingTime = time.Now()

	if result != nil {
		m.stats.LastPingDuration = result.ResponseTime

		if result.Success {
			m.stats.SuccessfulPings++
			m.stats.LastPingSuccess = true
			m.stats.ConsecutiveFailures = 0
		} else {
			m.stats.FailedPings++
			m.stats.LastPingSuccess = false
			m.stats.ConsecutiveFailures++
		}
	} else {
		// Ping completely failed
		m.stats.FailedPings++
		m.stats.LastPingSuccess = false
		m.stats.ConsecutiveFailures++
		m.stats.LastPingDuration = 0
	}
}

// logPingResult logs the outcome of a ping operation with appropriate detail.
func (m *Monitor) logPingResult(result *PingResult, err error) {
	if err != nil {
		log.Printf("Healthcheck ping error: %v", err)
		return
	}

	if result == nil {
		log.Printf("Healthcheck ping failed: no result")
		return
	}

	if result.Success {
		log.Printf("Healthcheck ping successful: status=%d, time=%v",
			result.StatusCode, result.ResponseTime)
	} else {
		// Check for concerning failure patterns
		m.mu.Lock()
		consecutiveFailures := m.stats.ConsecutiveFailures
		m.mu.Unlock()

		if consecutiveFailures > 3 {
			log.Printf("Healthcheck ping ALERT: %d consecutive failures, error=%v, time=%v",
				consecutiveFailures, result.Error, result.ResponseTime)
		} else {
			log.Printf("Healthcheck ping failed: error=%v, time=%v",
				result.Error, result.ResponseTime)
		}
	}
}

// GetStats returns a copy of the current monitoring statistics.
// This is thread-safe and provides insight into monitor performance.
func (m *Monitor) GetStats() MonitorStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to prevent external modification
	return m.stats
}

// IsRunning returns whether the monitor is currently active.
// This is thread-safe and useful for status checks.
func (m *Monitor) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// GetConfig returns a copy of the monitor's configuration.
// This is useful for debugging and configuration validation.
func (m *Monitor) GetConfig() *Config {
	return m.config
}

// HealthStatus represents the current health status based on recent ping results.
type HealthStatus struct {
	// Healthy indicates if the service is currently considered healthy
	Healthy bool

	// Message provides details about the health status
	Message string

	// LastCheck is when the most recent ping was performed
	LastCheck time.Time

	// ResponseTime is the most recent ping response time
	ResponseTime time.Duration

	// ConsecutiveFailures is the current count of consecutive failures
	ConsecutiveFailures int64
}

// GetHealthStatus returns the current health status based on recent ping results.
// This provides a high-level view of service health for monitoring dashboards.
func (m *Monitor) GetHealthStatus() HealthStatus {
	stats := m.GetStats()

	status := HealthStatus{
		LastCheck:           stats.LastPingTime,
		ResponseTime:        stats.LastPingDuration,
		ConsecutiveFailures: stats.ConsecutiveFailures,
	}

	// Determine health status based on recent results
	if stats.TotalPings == 0 {
		status.Healthy = false
		status.Message = "No health checks performed yet"
	} else if stats.ConsecutiveFailures == 0 {
		status.Healthy = true
		status.Message = "Service is healthy"
	} else if stats.ConsecutiveFailures < 3 {
		status.Healthy = false
		status.Message = fmt.Sprintf("Service experiencing issues (%d consecutive failures)", stats.ConsecutiveFailures)
	} else {
		status.Healthy = false
		status.Message = fmt.Sprintf("Service is unhealthy (%d consecutive failures)", stats.ConsecutiveFailures)
	}

	return status
}
