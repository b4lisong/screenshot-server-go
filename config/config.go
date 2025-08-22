// Package config provides configuration management for the screenshot server.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	// Server configuration
	Port int `yaml:"port"`
	
	// Storage configuration
	StorageDir        string `yaml:"storage_dir"`
	CleanupInterval   string `yaml:"cleanup_interval"`
	RetentionPeriod   string `yaml:"retention_period"`
	
	// Frontend configuration
	AutoRefreshInterval string `yaml:"auto_refresh_interval"`
	MaxFailures        int    `yaml:"max_failures"`
	
	// Logging configuration
	LogLevel string `yaml:"log_level"`
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
		LogLevel:           "info",
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