package main

import (
	"bytes"
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

// Global manager instance - in production, consider dependency injection
var manager *storage.Manager

// Templates are parsed once at startup for efficiency
var templates *template.Template

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
	manager = storage.NewManager(fileStorage)
	defer manager.Close()

	// Parse templates
	templates, err = template.ParseGlob("templates/*.html")
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

	// Start cleanup routine
	startCleanupRoutine()

	// Set up routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/screenshot", handleScreenshot)
	http.HandleFunc("/activity", handleActivity)
	http.HandleFunc("/screenshot/", handleScreenshotImage)

	log.Printf("Server started at http://localhost:%d", *port)
	log.Printf("View activity at http://localhost:%d/activity", *port)

	err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleHome redirects to the activity page.
func handleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/activity", http.StatusFound)
}

// handleScreenshot captures and returns a screenshot (existing functionality).
func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received screenshot request from %s", r.RemoteAddr)

	// Capture screenshot
	img, err := screenshot.Capture()
	if err != nil {
		log.Printf("Capture failed: %v", err)
		http.Error(w, "Failed to capture screenshot", http.StatusInternalServerError)
		return
	}

	// Save to storage (manual screenshot)
	_, err = manager.Save(img, false)
	if err != nil {
		log.Printf("Failed to save screenshot: %v", err)
		// Continue to serve the image even if save fails
	}

	log.Printf("Screenshot captured successfully for %s", r.RemoteAddr)

	// Encode and send response
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		log.Printf("Encoding failed: %v", err)
		http.Error(w, "Failed to encode image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// handleActivity serves the activity overview page.
// This demonstrates template rendering and data preparation.
func handleActivity(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Retrieve recent screenshots
	screenshots, err := manager.List(24)
	if err != nil {
		log.Printf("Failed to list screenshots: %v", err)
		http.Error(w, "Failed to retrieve screenshots", http.StatusInternalServerError)
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
	if err := templates.ExecuteTemplate(w, "activity.html", data); err != nil {
		log.Printf("Failed to render template: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// handleScreenshotImage serves individual screenshot images.
// URL pattern: /screenshot/{id}
func handleScreenshotImage(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	// Example: /screenshot/20240115_143052.000000000
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 || parts[2] == "" {
		http.NotFound(w, r)
		return
	}

	id := parts[2]

	// Retrieve screenshot metadata
	screenshot, err := manager.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Read image from disk
	img, err := storage.ReadScreenshot(screenshot.Path)
	if err != nil {
		log.Printf("Failed to read screenshot: %v", err)
		http.Error(w, "Failed to load screenshot", http.StatusInternalServerError)
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
func startCleanupRoutine() {
	go func() {
		// Run cleanup every hour
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		// Also run immediately on startup
		performCleanup()

		for range ticker.C {
			performCleanup()
		}
	}()
}

// performCleanup removes screenshots older than 1 week.
func performCleanup() {
	log.Println("Running screenshot cleanup...")

	if err := manager.Cleanup(7 * 24 * time.Hour); err != nil {
		log.Printf("Cleanup failed: %v", err)
	} else {
		log.Println("Cleanup completed")
	}
}
