package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/b4lisong/screenshot-server-go/config"
)

// TestNewConfig tests configuration creation and validation.
func TestNewConfig(t *testing.T) {
	tests := []struct {
		name        string
		appConfig   *config.Config
		expectError bool
	}{
		{
			name:        "nil config",
			appConfig:   nil,
			expectError: true,
		},
		{
			name: "disabled healthcheck",
			appConfig: &config.Config{
				Healthcheck: config.HealthcheckConfig{
					Enabled: false,
				},
			},
			expectError: false,
		},
		{
			name: "valid enabled config",
			appConfig: &config.Config{
				Healthcheck: config.HealthcheckConfig{
					Enabled:    true,
					PingURL:    "https://example.com/health",
					Interval:   5 * time.Minute,
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					UserAgent:  "Test-Agent",
				},
			},
			expectError: false,
		},
		{
			name: "invalid URL protocol",
			appConfig: &config.Config{
				Healthcheck: config.HealthcheckConfig{
					Enabled:    true,
					PingURL:    "http://example.com/health", // HTTP not allowed
					Interval:   5 * time.Minute,
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					UserAgent:  "Test-Agent",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfig(tt.appConfig)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestClientPing tests the HTTP client ping functionality.
func TestClientPing(t *testing.T) {
	// Create test server that returns success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Test-Agent" {
			t.Errorf("expected User-Agent 'Test-Agent', got '%s'", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Use HTTP server for testing to avoid TLS certificate issues
	// In production, we enforce HTTPS, but for unit tests we use HTTP
	// We'll override the URL validation for testing
	cfg := &Config{
		Enabled:    true,
		PingURL:    server.URL, // This will be HTTP for testing
		Interval:   1 * time.Minute,
		Timeout:    10 * time.Second,
		MaxRetries: 1,
		UserAgent:  "Test-Agent",
	}

	// Create client with modified HTTP transport for testing
	client := &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		config: cfg,
	}
	defer client.Close()

	// Test ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected successful ping, got: %v", result.Error)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got: %d", result.StatusCode)
	}

	if result.ResponseTime <= 0 {
		t.Error("expected positive response time")
	}
}

// TestMonitorLifecycle tests the monitor start/stop lifecycle.
func TestMonitorLifecycle(t *testing.T) {
	// Create config with disabled healthcheck
	cfg := &Config{
		Enabled:    false,
		PingURL:    "https://example.com/health",
		Interval:   1 * time.Second,
		Timeout:    500 * time.Millisecond,
		MaxRetries: 1,
		UserAgent:  "Test-Agent",
	}

	// Create monitor
	monitor, err := NewMonitor(cfg)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	// Test initial state
	if monitor.IsRunning() {
		t.Error("monitor should not be running initially")
	}

	// Test start (should succeed but not actually start since disabled)
	err = monitor.Start()
	if err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Test stop
	monitor.Stop()

	// Test double stop (should be safe)
	monitor.Stop()

	// Verify stats
	stats := monitor.GetStats()
	if stats.TotalPings != 0 {
		t.Errorf("expected 0 pings for disabled monitor, got: %d", stats.TotalPings)
	}
}

// TestHealthStatus tests the health status reporting.
func TestHealthStatus(t *testing.T) {
	cfg := &Config{
		Enabled:    false,
		PingURL:    "https://example.com/health",
		Interval:   1 * time.Second,
		Timeout:    500 * time.Millisecond,
		MaxRetries: 1,
		UserAgent:  "Test-Agent",
	}

	monitor, err := NewMonitor(cfg)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	status := monitor.GetHealthStatus()
	if status.Healthy {
		t.Error("expected unhealthy status for new monitor")
	}

	if status.Message == "" {
		t.Error("expected non-empty status message")
	}
}
