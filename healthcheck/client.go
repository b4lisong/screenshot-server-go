// Package healthcheck provides production-ready health monitoring with ping functionality.
package healthcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

// Client handles HTTP health check requests with retry logic and security features.
// It provides robust error handling and exponential backoff for production environments.
type Client struct {
	httpClient *http.Client
	config     *Config
}

// PingResult represents the result of a health check ping attempt.
type PingResult struct {
	// Success indicates whether the ping was successful
	Success bool

	// StatusCode is the HTTP response status code (0 if request failed)
	StatusCode int

	// ResponseTime is the duration of the HTTP request
	ResponseTime time.Duration

	// Error contains any error that occurred during the ping
	Error error

	// Attempt is the attempt number (1-based) for this ping
	Attempt int

	// Timestamp when the ping was performed
	Timestamp time.Time
}

// NewClient creates a new healthcheck client with production-ready defaults.
// It configures secure HTTP settings including TLS verification and timeouts.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create HTTP client with production security settings
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			// Enable TLS verification for security
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Always verify certificates in production
				MinVersion:         tls.VersionTLS12,
			},
			// Connection pool settings for efficiency
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
			// Timeouts for various phases
			DisableKeepAlives:     false,
			DisableCompression:    false,
			ResponseHeaderTimeout: config.Timeout / 2, // Half of total timeout
		},
		// Disable automatic redirects for security
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Client{
		httpClient: httpClient,
		config:     config,
	}, nil
}

// Ping performs a health check ping with retry logic and exponential backoff.
// It attempts the request up to MaxRetries times with increasing delays between attempts.
func (c *Client) Ping(ctx context.Context) (*PingResult, error) {
	if !c.config.IsEnabled() {
		return nil, fmt.Errorf("healthcheck is disabled")
	}

	var lastResult *PingResult
	maxAttempts := c.config.MaxRetries + 1 // Include initial attempt

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Perform the ping attempt
		result := c.performPing(ctx, attempt)
		lastResult = result

		// Log the attempt result
		c.logPingResult(result)

		// Return immediately on success
		if result.Success {
			return result, nil
		}

		// Don't delay after the final attempt
		if attempt < maxAttempts {
			// Calculate exponential backoff delay
			delay := c.calculateBackoffDelay(attempt)

			log.Printf("Healthcheck ping failed (attempt %d/%d), retrying in %v",
				attempt, maxAttempts, delay)

			// Wait for backoff delay or context cancellation
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	// All attempts failed
	return lastResult, fmt.Errorf("all ping attempts failed after %d tries", maxAttempts)
}

// performPing executes a single ping attempt and returns the result.
func (c *Client) performPing(ctx context.Context, attempt int) *PingResult {
	result := &PingResult{
		Success:   false,
		Attempt:   attempt,
		Timestamp: time.Now(),
	}

	// Create request with context for cancellation
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.PingURL, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		return result
	}

	// Set user agent for identification
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Set additional headers for monitoring
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "close") // Prevent connection reuse for cleaner monitoring

	// Perform the request with timing
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Error = fmt.Errorf("HTTP request failed: %w", err)
		return result
	}
	defer resp.Body.Close()

	// Record status code
	result.StatusCode = resp.StatusCode

	// Check if status code indicates success (2xx range)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
		result.Error = nil
	} else {
		result.Error = fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	return result
}

// calculateBackoffDelay computes the exponential backoff delay for retry attempts.
// It uses a base delay of 30 seconds with exponential growth capped at 2 minutes.
func (c *Client) calculateBackoffDelay(attempt int) time.Duration {
	// Base delay of 30 seconds
	baseDelay := 30 * time.Second

	// Exponential backoff: 30s, 60s, 120s (capped)
	backoffMultiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(baseDelay) * backoffMultiplier)

	// Cap at 2 minutes to prevent excessive delays
	maxDelay := 2 * time.Minute
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// logPingResult logs the result of a ping attempt with appropriate log levels.
func (c *Client) logPingResult(result *PingResult) {
	if result.Success {
		log.Printf("Healthcheck ping successful: status=%d, time=%v, attempt=%d",
			result.StatusCode, result.ResponseTime, result.Attempt)
	} else {
		if result.Attempt == 1 {
			// Log as warning on first failure
			log.Printf("Healthcheck ping failed: %v, time=%v, attempt=%d",
				result.Error, result.ResponseTime, result.Attempt)
		} else {
			// Log as warning on retry failures
			log.Printf("Healthcheck ping retry failed: %v, time=%v, attempt=%d",
				result.Error, result.ResponseTime, result.Attempt)
		}
	}
}

// Close performs cleanup of the HTTP client resources.
// This ensures proper resource management in production environments.
func (c *Client) Close() error {
	// Close idle connections
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}

// GetConfig returns a copy of the client's configuration.
// This is useful for debugging and configuration validation.
func (c *Client) GetConfig() *Config {
	// Return a copy to prevent external modification
	configCopy := *c.config
	return &configCopy
}
