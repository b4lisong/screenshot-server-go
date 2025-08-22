// Package config provides configuration management for the screenshot server.
package config

import (
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	// Server configuration
	Port int `yaml:"port"`

	// Storage configuration
	StorageDir      string `yaml:"storage_dir"`
	CleanupInterval string `yaml:"cleanup_interval"`
	RetentionPeriod string `yaml:"retention_period"`

	// Frontend configuration
	AutoRefreshInterval string `yaml:"auto_refresh_interval"`
	MaxFailures         int    `yaml:"max_failures"`

	// Logging configuration
	LogLevel string `yaml:"log_level"`

	// Email configuration
	Email EmailConfig `yaml:"email"`
}

// EmailConfig represents SMTP email notification configuration.
type EmailConfig struct {
	// Enable/disable email notifications
	Enabled bool `yaml:"enabled"`

	// SMTP server configuration
	SMTPHost     string `yaml:"smtp_host"`
	SMTPPort     int    `yaml:"smtp_port"`
	SMTPUsername string `yaml:"smtp_username"`
	SMTPPassword string `yaml:"smtp_password"`
	SMTPSecurity string `yaml:"smtp_security"` // "none", "tls", "starttls"

	// Email addresses
	FromEmail string   `yaml:"from_email"`
	ToEmails  []string `yaml:"to_emails"`

	// Email content configuration
	SubjectPrefix string `yaml:"subject_prefix"`

	// Notification settings
	ServerStart     bool   `yaml:"server_start"`
	ServerStop      bool   `yaml:"server_stop"`
	DailySummary    bool   `yaml:"daily_summary"`
	SummaryTime     string `yaml:"summary_time"`     // "15:04" format
	SummaryTimezone string `yaml:"summary_timezone"` // IANA timezone

	// Attachment configuration
	Attachments AttachmentConfig `yaml:"attachments"`
}

// AttachmentConfig represents configuration for email attachments.
type AttachmentConfig struct {
	// Enable/disable email attachments
	Enabled bool `yaml:"enabled"`

	// Compression settings
	CompressionQuality int `yaml:"compression_quality"` // 1-100 JPEG quality

	// Size limits
	MaxAttachmentSizeMB float64 `yaml:"max_attachment_size_mb"` // Per-attachment limit
	MaxTotalSizeMB      float64 `yaml:"max_total_size_mb"`      // Total email size limit
	MaxScreenshots      int     `yaml:"max_screenshots"`        // Maximum screenshots per email

	// Image processing
	ResizeMaxWidth  int `yaml:"resize_max_width"`  // Maximum width in pixels
	ResizeMaxHeight int `yaml:"resize_max_height"` // Maximum height in pixels

	// Attachment strategy
	Strategy string `yaml:"strategy"` // "individual", "zip", "adaptive"
}

// Default returns a configuration with default values.
func Default() *Config {
	return &Config{
		Port:                8080,
		StorageDir:          "./screenshots",
		CleanupInterval:     "1h",
		RetentionPeriod:     "168h", // 7 days
		AutoRefreshInterval: "30s",
		MaxFailures:         3,
		LogLevel:            "info",
		Email: EmailConfig{
			Enabled:         false,
			SMTPPort:        587,
			SMTPSecurity:    "starttls",
			SubjectPrefix:   "[Screenshot Server]",
			ServerStart:     true,
			ServerStop:      true,
			DailySummary:    true,
			SummaryTime:     "09:00",
			SummaryTimezone: "Local",
			Attachments: AttachmentConfig{
				Enabled:             true,
				CompressionQuality:  75,
				MaxAttachmentSizeMB: 5.0,
				MaxTotalSizeMB:      20.0,
				MaxScreenshots:      10,
				ResizeMaxWidth:      1920,
				ResizeMaxHeight:     1080,
				Strategy:            "adaptive",
			},
		},
	}
}

// LoadConfig loads configuration from config.yaml file with fallback to defaults.
// Returns a configuration with default values if the file doesn't exist.
func LoadConfig(filename string) (*Config, error) {
	// Start with default configuration
	config := Default()

	// Check if config file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// File doesn't exist, return defaults
		return config, nil
	}

	// Read the config file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration values are valid.
func (c *Config) Validate() error {
	// Validate port
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}

	// Validate storage directory
	if c.StorageDir == "" {
		return fmt.Errorf("storage_dir cannot be empty")
	}

	// Validate time durations
	if _, err := time.ParseDuration(c.CleanupInterval); err != nil {
		return fmt.Errorf("invalid cleanup_interval: %w", err)
	}

	if _, err := time.ParseDuration(c.RetentionPeriod); err != nil {
		return fmt.Errorf("invalid retention_period: %w", err)
	}

	if _, err := time.ParseDuration(c.AutoRefreshInterval); err != nil {
		return fmt.Errorf("invalid auto_refresh_interval: %w", err)
	}

	// Validate max failures
	if c.MaxFailures < 1 {
		return fmt.Errorf("max_failures must be at least 1, got %d", c.MaxFailures)
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log_level: %s (must be one of: debug, info, warn, error)", c.LogLevel)
	}

	// Validate email configuration if enabled
	if c.Email.Enabled {
		if err := c.validateEmailConfig(); err != nil {
			return fmt.Errorf("invalid email configuration: %w", err)
		}

		// Validate attachment configuration if enabled
		if c.Email.Attachments.Enabled {
			if err := c.validateAttachmentConfig(); err != nil {
				return fmt.Errorf("invalid attachment configuration: %w", err)
			}
		}
	}

	return nil
}

// GetCleanupInterval returns the cleanup interval as a time.Duration.
func (c *Config) GetCleanupInterval() time.Duration {
	duration, _ := time.ParseDuration(c.CleanupInterval)
	return duration
}

// GetRetentionPeriod returns the retention period as a time.Duration.
func (c *Config) GetRetentionPeriod() time.Duration {
	duration, _ := time.ParseDuration(c.RetentionPeriod)
	return duration
}

// GetAutoRefreshInterval returns the auto-refresh interval as a time.Duration.
func (c *Config) GetAutoRefreshInterval() time.Duration {
	duration, _ := time.ParseDuration(c.AutoRefreshInterval)
	return duration
}

// GetAutoRefreshMilliseconds returns the auto-refresh interval in milliseconds for JavaScript.
func (c *Config) GetAutoRefreshMilliseconds() int {
	return int(c.GetAutoRefreshInterval().Milliseconds())
}

// validateEmailConfig validates email configuration settings.
func (c *Config) validateEmailConfig() error {
	// Validate SMTP host
	if c.Email.SMTPHost == "" {
		return fmt.Errorf("smtp_host cannot be empty when email is enabled")
	}

	// Validate SMTP port
	if c.Email.SMTPPort < 1 || c.Email.SMTPPort > 65535 {
		return fmt.Errorf("smtp_port must be between 1 and 65535, got %d", c.Email.SMTPPort)
	}

	// Validate SMTP security
	validSecurity := map[string]bool{
		"none":     true,
		"tls":      true,
		"starttls": true,
	}
	if !validSecurity[c.Email.SMTPSecurity] {
		return fmt.Errorf("invalid smtp_security: %s (must be one of: none, tls, starttls)", c.Email.SMTPSecurity)
	}

	// Validate from email
	if c.Email.FromEmail == "" {
		return fmt.Errorf("from_email cannot be empty when email is enabled")
	}
	if _, err := mail.ParseAddress(c.Email.FromEmail); err != nil {
		return fmt.Errorf("invalid from_email format: %w", err)
	}

	// Validate to emails
	if len(c.Email.ToEmails) == 0 {
		return fmt.Errorf("to_emails cannot be empty when email is enabled")
	}
	for i, email := range c.Email.ToEmails {
		if _, err := mail.ParseAddress(email); err != nil {
			return fmt.Errorf("invalid to_email[%d] format: %w", i, err)
		}
	}

	// Validate summary time format
	if c.Email.DailySummary {
		if _, err := time.Parse("15:04", c.Email.SummaryTime); err != nil {
			return fmt.Errorf("invalid summary_time format (must be HH:MM): %w", err)
		}

		// Validate timezone
		if c.Email.SummaryTimezone != "Local" {
			if _, err := time.LoadLocation(c.Email.SummaryTimezone); err != nil {
				return fmt.Errorf("invalid summary_timezone: %w", err)
			}
		}
	}

	return nil
}

// GetSummaryLocation returns the timezone location for daily summaries.
func (c *Config) GetSummaryLocation() *time.Location {
	if c.Email.SummaryTimezone == "Local" {
		return time.Local
	}
	loc, err := time.LoadLocation(c.Email.SummaryTimezone)
	if err != nil {
		return time.Local // Fallback to local time
	}
	return loc
}

// GetSMTPAddress returns the full SMTP server address.
func (c *Config) GetSMTPAddress() string {
	return c.Email.SMTPHost + ":" + strconv.Itoa(c.Email.SMTPPort)
}

// validateAttachmentConfig validates attachment configuration settings.
func (c *Config) validateAttachmentConfig() error {
	// Validate compression quality
	if c.Email.Attachments.CompressionQuality < 1 || c.Email.Attachments.CompressionQuality > 100 {
		return fmt.Errorf("compression_quality must be between 1 and 100, got %d", c.Email.Attachments.CompressionQuality)
	}

	// Validate size limits
	if c.Email.Attachments.MaxAttachmentSizeMB <= 0 {
		return fmt.Errorf("max_attachment_size_mb must be positive, got %f", c.Email.Attachments.MaxAttachmentSizeMB)
	}

	if c.Email.Attachments.MaxTotalSizeMB <= 0 {
		return fmt.Errorf("max_total_size_mb must be positive, got %f", c.Email.Attachments.MaxTotalSizeMB)
	}

	if c.Email.Attachments.MaxTotalSizeMB < c.Email.Attachments.MaxAttachmentSizeMB {
		return fmt.Errorf("max_total_size_mb (%f) cannot be less than max_attachment_size_mb (%f)",
			c.Email.Attachments.MaxTotalSizeMB, c.Email.Attachments.MaxAttachmentSizeMB)
	}

	// Validate screenshot count
	if c.Email.Attachments.MaxScreenshots <= 0 {
		return fmt.Errorf("max_screenshots must be positive, got %d", c.Email.Attachments.MaxScreenshots)
	}

	// Validate resize dimensions
	if c.Email.Attachments.ResizeMaxWidth <= 0 {
		return fmt.Errorf("resize_max_width must be positive, got %d", c.Email.Attachments.ResizeMaxWidth)
	}

	if c.Email.Attachments.ResizeMaxHeight <= 0 {
		return fmt.Errorf("resize_max_height must be positive, got %d", c.Email.Attachments.ResizeMaxHeight)
	}

	// Validate strategy
	validStrategies := map[string]bool{
		"individual": true,
		"zip":        true,
		"adaptive":   true,
	}
	if !validStrategies[c.Email.Attachments.Strategy] {
		return fmt.Errorf("invalid strategy: %s (must be one of: individual, zip, adaptive)", c.Email.Attachments.Strategy)
	}

	return nil
}
