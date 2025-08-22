package main

import (
	"html/template"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/b4lisong/screenshot-server-go/config"
	"github.com/b4lisong/screenshot-server-go/email"
	"github.com/b4lisong/screenshot-server-go/scheduler"
	"github.com/b4lisong/screenshot-server-go/screenshot"
	"github.com/b4lisong/screenshot-server-go/storage"
)

// TestActivityHandler tests the activity page handler.
func TestActivityHandler(t *testing.T) {
	// Create temporary storage
	tempDir := t.TempDir()
	fileStorage, err := storage.NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	// Create manager for the test
	manager := storage.NewManager(fileStorage)
	defer manager.Close()

	// Parse templates
	templates, err := template.ParseGlob("templates/*.html")
	if err != nil {
		t.Skipf("skipping test - templates not found: %v", err)
	}

	// Create a mock scheduler (not used in this test)
	mockScheduler := scheduler.New(screenshot.Capture, func(img image.Image, isAutomatic bool) error {
		return nil
	})

	// Create test config
	cfg := config.Default()
	cfg.Email.Enabled = false // Disable email for tests

	// Create mock mailer
	mailer, err := email.New(&cfg.Email, tempDir)
	if err != nil {
		t.Fatalf("creating mailer: %v", err)
	}

	// Create mock daily scheduler
	dailyScheduler := email.NewDailySummaryScheduler(cfg, fileStorage, mailer, email.ServerInfo{
		Port:       8080,
		StorageDir: tempDir,
		Version:    "test",
	})

	// Create server instance
	server := NewServer(manager, templates, mockScheduler, cfg, mailer, dailyScheduler)

	// Save some test screenshots
	testImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for i := 0; i < 3; i++ {
		_, err := manager.Save(testImg, i%2 == 0)
		if err != nil {
			t.Fatalf("saving test screenshot: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Create test request
	req, err := http.NewRequest("GET", "/activity", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	// Record response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleActivity)
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check content type
	expected := "text/html; charset=utf-8"
	if ct := rr.Header().Get("Content-Type"); ct != expected {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ct, expected)
	}

	// Check that response body contains expected content
	body := rr.Body.String()
	if !strings.Contains(body, "Screenshot Activity") {
		t.Error("response should contain page title")
	}
}

// TestScreenshotImageHandler tests serving individual screenshots.
func TestScreenshotImageHandler(t *testing.T) {
	// Set up storage and save a test screenshot
	tempDir := t.TempDir()
	fileStorage, err := storage.NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("creating storage: %v", err)
	}

	manager := storage.NewManager(fileStorage)
	defer manager.Close()

	// Create a mock scheduler (not used in this test)
	mockScheduler := scheduler.New(screenshot.Capture, func(img image.Image, isAutomatic bool) error {
		return nil
	})

	// Create test config
	cfg := config.Default()
	cfg.Email.Enabled = false // Disable email for tests

	// Create mock mailer
	mailer, err := email.New(&cfg.Email, tempDir)
	if err != nil {
		t.Fatalf("creating mailer: %v", err)
	}

	// Create mock daily scheduler
	dailyScheduler := email.NewDailySummaryScheduler(cfg, fileStorage, mailer, email.ServerInfo{
		Port:       8080,
		StorageDir: tempDir,
		Version:    "test",
	})

	// Create server instance
	server := NewServer(manager, nil, mockScheduler, cfg, mailer, dailyScheduler)

	testImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	screenshot, err := manager.Save(testImg, false)
	if err != nil {
		t.Fatalf("saving test screenshot: %v", err)
	}

	// Test valid request
	req, err := http.NewRequest("GET", "/screenshot/"+screenshot.ID, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleScreenshotImage)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ct, "image/png")
	}

	// Test invalid request
	req, _ = http.NewRequest("GET", "/screenshot/invalid", nil)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(server.handleScreenshotImage)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler should return 404 for invalid ID: got %v", status)
	}
}
