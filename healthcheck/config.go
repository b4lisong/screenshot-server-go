// Package healthcheck provides production-ready health monitoring with ping functionality.
package healthcheck

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/b4lisong/screenshot-server-go/config"
)

// Config represents the healthcheck monitoring configuration.
// This provides a typed interface to the application's healthcheck settings.
type Config struct {
	// Enabled determines if healthcheck monitoring is active
	Enabled bool

	// PingURL is the HTTPS endpoint to monitor
	PingURL string

	// Interval between health ping requests
	Interval time.Duration

	// Timeout for each individual ping request
	Timeout time.Duration

	// MaxRetries specifies the maximum number of retry attempts for failed pings
	MaxRetries int

	// UserAgent string sent with HTTP requests for identification
	UserAgent string
}

// NewConfig creates a new healthcheck configuration from the main application config.
// It processes environment variable substitution and applies security defaults.
func NewConfig(cfg *config.Config) (*Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("application config cannot be nil")
	}

	healthcheckConfig := &Config{
		Enabled:    cfg.Healthcheck.Enabled,
		PingURL:    cfg.Healthcheck.PingURL,
		Interval:   cfg.Healthcheck.Interval,
		Timeout:    cfg.Healthcheck.Timeout,
		MaxRetries: cfg.Healthcheck.MaxRetries,
		UserAgent:  cfg.Healthcheck.UserAgent,
	}

	// Process environment variable substitution for sensitive data
	if err := healthcheckConfig.processEnvironmentVariables(); err != nil {
		return nil, fmt.Errorf("failed to process environment variables: %w", err)
	}

	// Apply validation with security checks
	if err := healthcheckConfig.validate(); err != nil {
		return nil, fmt.Errorf("invalid healthcheck configuration: %w", err)
	}

	return healthcheckConfig, nil
}

// processEnvironmentVariables handles environment variable substitution for configuration values.
// This allows sensitive URLs to be stored in environment variables instead of config files.
func (c *Config) processEnvironmentVariables() error {
	// Process PingURL environment variable substitution
	if strings.Contains(c.PingURL, "${") && strings.Contains(c.PingURL, "}") {
		// Extract environment variable name from ${VAR_NAME} pattern
		start := strings.Index(c.PingURL, "${")
		end := strings.Index(c.PingURL, "}")
		if start >= 0 && end > start {
			envVar := c.PingURL[start+2 : end]
			envValue := os.Getenv(envVar)
			if envValue == "" {
				return fmt.Errorf("environment variable %s is not set", envVar)
			}
			// Replace the entire ${VAR_NAME} with the environment value
			c.PingURL = strings.Replace(c.PingURL, c.PingURL[start:end+1], envValue, 1)
		}
	}

	return nil
}

// validate performs comprehensive validation of the healthcheck configuration.
// This ensures all settings are secure and within acceptable operational limits.
func (c *Config) validate() error {
	// Skip validation if disabled
	if !c.Enabled {
		return nil
	}

	// Validate PingURL requirements
	if c.PingURL == "" {
		return fmt.Errorf("ping_url cannot be empty when healthcheck is enabled")
	}

	// Enforce HTTPS for security in production
	if !strings.HasPrefix(c.PingURL, "https://") {
		return fmt.Errorf("ping_url must use HTTPS protocol for security, got: %s", c.PingURL)
	}

	// Validate timing constraints
	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive, got: %v", c.Interval)
	}

	// Prevent excessive request frequency
	if c.Interval < 30*time.Second {
		return fmt.Errorf("interval must be at least 30 seconds to avoid excessive requests, got: %v", c.Interval)
	}

	// Validate timeout constraints
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", c.Timeout)
	}

	// Ensure timeout doesn't exceed interval to prevent overlapping requests
	if c.Timeout >= c.Interval {
		return fmt.Errorf("timeout (%v) must be less than interval (%v)", c.Timeout, c.Interval)
	}

	// Validate retry limits
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative, got: %d", c.MaxRetries)
	}

	// Prevent excessive retry attempts that could impact monitored services
	if c.MaxRetries > 10 {
		return fmt.Errorf("max_retries must be at most 10 to avoid excessive load, got: %d", c.MaxRetries)
	}

	// Validate user agent for proper identification
	if c.UserAgent == "" {
		return fmt.Errorf("user_agent cannot be empty")
	}

	return nil
}

// IsEnabled returns whether healthcheck monitoring is active.
// This is a convenience method for cleaner conditional logic.
func (c *Config) IsEnabled() bool {
	return c.Enabled
}

// String returns a safe string representation of the configuration.
// Sensitive information like full URLs are masked for logging.
func (c *Config) String() string {
	if !c.Enabled {
		return "Healthcheck: disabled"
	}

	// Mask the URL for security in logs
	maskedURL := c.PingURL
	if len(maskedURL) > 20 {
		maskedURL = maskedURL[:20] + "..."
	}

	return fmt.Sprintf("Healthcheck: enabled, URL=%s, interval=%v, timeout=%v, retries=%d",
		maskedURL, c.Interval, c.Timeout, c.MaxRetries)
}
