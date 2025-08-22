package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/b4lisong/screenshot-server-go/scheduler"
	"github.com/b4lisong/screenshot-server-go/screenshot"
	"github.com/b4lisong/screenshot-server-go/storage"
)

// Server holds all dependencies for the HTTP server.
// This eliminates global variables and enables dependency injection.
type Server struct {
	manager   *storage.Manager
	templates *template.Template
	scheduler *scheduler.Scheduler
}

// ScreenshotResponse represents the JSON response for screenshot API endpoints
type ScreenshotResponse struct {
	ID          string    `json:"id"`
	CapturedAt  time.Time `json:"captured_at"`
	IsAutomatic bool      `json:"is_automatic"`
	URL         string    `json:"url"`
}

// ErrorResponse represents error responses for API endpoints
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// NewServer creates a new Server instance with all dependencies.
func NewServer(manager *storage.Manager, templates *template.Template, scheduler *scheduler.Scheduler) *Server {
	return &Server{
		manager:   manager,
		templates: templates,
		scheduler: scheduler,
	}
}

// toScreenshotResponse converts a storage.Screenshot to a ScreenshotResponse.
// This helper function eliminates duplication between API handlers.
func toScreenshotResponse(screenshot *storage.Screenshot) ScreenshotResponse {
	return ScreenshotResponse{
		ID:          screenshot.ID,
		CapturedAt:  screenshot.CapturedAt,
		IsAutomatic: screenshot.IsAutomatic,
		URL:         "/screenshot/" + screenshot.ID,
	}
}

// captureAndSave captures a screenshot and saves it to storage.
// This helper function eliminates duplication between screenshot handlers.
func (s *Server) captureAndSave() (*storage.Screenshot, error) {
	img, err := screenshot.Capture()
	if err != nil {
		return nil, fmt.Errorf("capture failed: %w", err)
	}

	screenshot, err := s.manager.Save(img, false)
	if err != nil {
		return nil, fmt.Errorf("save failed: %w", err)
	}

	return screenshot, nil
}

func main() {
	// Parse command-line flags
	port := flag.Int("p", 8080, "port to run the server on")
	storageDir := flag.String("storage", "./screenshots", "directory to store screenshots")
	flag.Parse()

	// Initialize storage
	fileStorage, err := storage.NewFileStorage(*storageDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create manager for thread-safe operations
	manager := storage.NewManager(fileStorage)
	defer manager.Close()

	// Parse templates
	templates, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Start automatic screenshot scheduler
	sched := scheduler.New(screenshot.Capture, func(img image.Image, isAutomatic bool) error {
		_, err := manager.Save(img, isAutomatic)
		return err
	})
	if err := sched.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// Create server with dependencies
	server := NewServer(manager, templates, sched)

	// Start cleanup routine
	server.startCleanupRoutine()

	// Set up routes with server methods
	http.HandleFunc("/", server.handleHome)
	http.HandleFunc("/screenshot", server.handleScreenshot)
	http.HandleFunc("/activity", server.handleActivity)
	http.HandleFunc("/screenshot/", server.handleScreenshotImage)

	// API routes for asynchronous frontend functionality
	http.HandleFunc("/api/screenshot", server.handleAPIScreenshot)
	http.HandleFunc("/api/screenshots", server.handleAPIScreenshots)

	log.Printf("Server started at http://localhost:%d", *port)
	log.Printf("View activity at http://localhost:%d/activity", *port)

	err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleHome redirects to the activity page.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/activity", http.StatusFound)
}

// handleScreenshot captures and returns a screenshot (existing functionality).
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received screenshot request from %s", r.RemoteAddr)

	screenshot, err := s.captureAndSave()
	if err != nil {
		log.Printf("Screenshot operation failed: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "capture_failed", "Failed to capture screenshot")
		return
	}

	// Load image for serving
	img, err := storage.ReadScreenshot(screenshot.Path)
	if err != nil {
		log.Printf("Failed to read saved screenshot: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "load_failed", "Failed to load screenshot")
		return
	}

	log.Printf("Screenshot captured successfully for %s", r.RemoteAddr)

	// Set headers before encoding (required for streaming)
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)

	// Encode directly to ResponseWriter for better resource efficiency
	err = png.Encode(w, img)
	if err != nil {
		log.Printf("Failed to encode image to response: %v", err)
	}
}

// handleActivity serves the activity overview page.
// This demonstrates template rendering and data preparation.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are allowed")
		return
	}

	// Retrieve recent screenshots
	screenshots, err := s.manager.List(24)
	if err != nil {
		log.Printf("Failed to list screenshots: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "list_failed", "Failed to retrieve screenshots")
		return
	}

	// Prepare template data
	// In Go, we create a struct to pass data to templates
	data := struct {
		Title       string
		Screenshots []*storage.Screenshot
		Now         time.Time
	}{
		Title:       "Screenshot Activity",
		Screenshots: screenshots,
		Now:         time.Now(),
	}

	// Execute template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "activity.html", data); err != nil {
		log.Printf("Failed to render template: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "template_render_failed", "Failed to render page")
	}
}

// handleScreenshotImage serves individual screenshot images.
// URL pattern: /screenshot/{id}
func (s *Server) handleScreenshotImage(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	// Example: /screenshot/20240115_143052.000000000
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 || parts[2] == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "invalid_id_format", "Invalid screenshot ID format")
		return
	}

	id := parts[2]

	// Retrieve screenshot metadata
	screenshot, err := s.manager.Get(id)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "screenshot_not_found", "Screenshot not found")
		return
	}

	// Read image from disk
	img, err := storage.ReadScreenshot(screenshot.Path)
	if err != nil {
		log.Printf("Failed to read screenshot: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "load_failed", "Failed to load screenshot")
		return
	}

	// Encode and serve
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	if err := png.Encode(w, img); err != nil {
		log.Printf("Failed to encode screenshot: %v", err)
	}
}

// startCleanupRoutine starts a goroutine that periodically removes old screenshots.
// This demonstrates long-running background tasks in Go.
func (s *Server) startCleanupRoutine() {
	go func() {
		// Run cleanup every hour
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		// Also run immediately on startup
		s.performCleanup()

		for range ticker.C {
			s.performCleanup()
		}
	}()
}

// performCleanup removes screenshots older than 1 week.
func (s *Server) performCleanup() {
	log.Println("Running screenshot cleanup...")

	if err := s.manager.Cleanup(7 * 24 * time.Hour); err != nil {
		log.Printf("Cleanup failed: %v", err)
	} else {
		log.Println("Cleanup completed")
	}
}

// handleAPIScreenshot captures a screenshot and returns JSON metadata.
// This endpoint is designed for fetch API calls from the frontend.
func (s *Server) handleAPIScreenshot(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests for this API endpoint
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST requests are allowed")
		return
	}

	log.Printf("Received API screenshot request from %s", r.RemoteAddr)

	screenshot, err := s.captureAndSave()
	if err != nil {
		log.Printf("Screenshot operation failed: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "capture_failed", "Failed to capture screenshot")
		return
	}

	log.Printf("Screenshot captured successfully for %s", r.RemoteAddr)

	// Create response using helper function
	response := toScreenshotResponse(screenshot)

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleAPIScreenshots returns recent screenshots as JSON.
// This endpoint supports the gallery refresh functionality.
func (s *Server) handleAPIScreenshots(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are allowed")
		return
	}

	// Retrieve recent screenshots
	screenshots, err := s.manager.List(24)
	if err != nil {
		log.Printf("Failed to list screenshots: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "list_failed", "Failed to retrieve screenshots")
		return
	}

	// Convert to API response format using helper function
	var response []ScreenshotResponse
	for _, screenshot := range screenshots {
		response = append(response, toScreenshotResponse(screenshot))
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response with proper headers.
func (s *Server) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// writeErrorResponse writes a standardized JSON error response.
func (s *Server) writeErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}
	s.writeJSONResponse(w, statusCode, response)
}
