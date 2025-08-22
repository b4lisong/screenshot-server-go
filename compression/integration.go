// Package compression provides integration utilities for the screenshot server application.
// This file contains integration helpers for seamless compression integration with
// the existing screenshot capture, storage, and email systems.
package compression

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"
)

// ScreenshotCompressionManager integrates compression with the screenshot server's
// storage and email systems. It provides production-ready compression for various
// use cases including email attachments, web display, and archival storage.
type ScreenshotCompressionManager struct {
	compressor    Compressor
	emailService  *EmailCompressionService
	fileService   *FileCompressionService
	storageDir    string
	tempDir       string
	enableLogging bool
}

// NewScreenshotCompressionManager creates a new compression manager for the screenshot server.
func NewScreenshotCompressionManager(storageDir string) *ScreenshotCompressionManager {
	return &ScreenshotCompressionManager{
		compressor:    NewCompressor(),
		emailService:  NewEmailCompressionService(),
		fileService:   NewFileCompressionService(),
		storageDir:    storageDir,
		tempDir:       filepath.Join(storageDir, "temp"),
		enableLogging: true,
	}
}

// CompressedScreenshot represents a screenshot with compression metadata.
type CompressedScreenshot struct {
	ID               string             `json:"id"`
	OriginalPath     string             `json:"original_path"`
	CompressedPath   string             `json:"compressed_path,omitempty"`
	CompressionStats CompressionStats   `json:"compression_stats"`
	CreatedAt        time.Time          `json:"created_at"`
	Options          CompressionOptions `json:"options"`
}

// CompressScreenshotForEmail compresses a screenshot file optimized for email attachment.
// This method handles the complete workflow from file loading to compressed output.
func (m *ScreenshotCompressionManager) CompressScreenshotForEmail(screenshotPath string) (*CompressedScreenshot, []byte, error) {
	start := time.Now()

	// Load the screenshot image
	img, err := m.loadImageFromFile(screenshotPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load screenshot %s: %w", screenshotPath, err)
	}

	// Compress for email
	compressedData, stats, err := m.emailService.CompressForEmail(img)
	if err != nil {
		return nil, nil, fmt.Errorf("email compression failed: %w", err)
	}

	// Create compressed screenshot metadata
	compressed := &CompressedScreenshot{
		ID:               m.generateCompressionID(screenshotPath),
		OriginalPath:     screenshotPath,
		CompressionStats: stats,
		CreatedAt:        start,
		Options:          m.emailService.options,
	}

	if m.enableLogging {
		m.logCompression("email", screenshotPath, stats)
	}

	return compressed, compressedData, nil
}

// CompressScreenshotForWeb compresses a screenshot optimized for web display.
func (m *ScreenshotCompressionManager) CompressScreenshotForWeb(screenshotPath string) (*CompressedScreenshot, error) {
	start := time.Now()

	// Web-optimized compression options
	webOpts := CompressionOptions{
		Quality:             85,
		Format:              "jpeg",
		MaxWidth:            1920,
		MaxHeight:           1080,
		PreserveAspectRatio: true,
		MaxSizeKB:           800, // Reasonable for web
	}

	// Load the screenshot image
	img, err := m.loadImageFromFile(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load screenshot %s: %w", screenshotPath, err)
	}

	// Compress for web
	compressedData, err := m.compressor.CompressImage(img, webOpts)
	if err != nil {
		return nil, fmt.Errorf("web compression failed: %w", err)
	}

	// Save compressed version
	compressedPath := m.generateCompressedPath(screenshotPath, "web")
	if err := m.saveCompressedData(compressedData, compressedPath); err != nil {
		return nil, fmt.Errorf("failed to save web compressed image: %w", err)
	}

	// Calculate statistics
	originalBounds := img.Bounds()
	stats := CompressionStats{
		OriginalSizeKB:   estimateImageSizeKB(originalBounds),
		CompressedSizeKB: len(compressedData) / 1024,
		CompressionRatio: float64(len(compressedData)) / float64(estimateImageSizeKB(originalBounds)*1024),
		Quality:          webOpts.Quality,
		Duration:         time.Since(start),
		Format:           webOpts.Format,
	}

	compressed := &CompressedScreenshot{
		ID:               m.generateCompressionID(screenshotPath),
		OriginalPath:     screenshotPath,
		CompressedPath:   compressedPath,
		CompressionStats: stats,
		CreatedAt:        start,
		Options:          webOpts,
	}

	if m.enableLogging {
		m.logCompression("web", screenshotPath, stats)
	}

	return compressed, nil
}

// BatchCompressScreenshots compresses multiple screenshots with different optimization profiles.
func (m *ScreenshotCompressionManager) BatchCompressScreenshots(screenshotPaths []string, profile string) ([]*CompressedScreenshot, error) {
	if len(screenshotPaths) == 0 {
		return []*CompressedScreenshot{}, nil
	}

	// Choose compression options based on profile
	opts, err := m.getProfileOptions(profile)
	if err != nil {
		return nil, fmt.Errorf("invalid compression profile %s: %w", profile, err)
	}

	results := make([]*CompressedScreenshot, 0, len(screenshotPaths))

	// Process each screenshot
	for i, path := range screenshotPaths {
		// Load image
		img, err := m.loadImageFromFile(path)
		if err != nil {
			m.logError("batch", path, err)
			continue
		}

		// Compress
		compressedData, err := m.compressor.CompressImage(img, opts)
		if err != nil {
			m.logError("batch", path, err)
			continue
		}

		// Save if not email profile (email data is returned directly)
		var compressedPath string
		if profile != "email" {
			compressedPath = m.generateCompressedPath(path, profile)
			if err := m.saveCompressedData(compressedData, compressedPath); err != nil {
				m.logError("batch", path, err)
				continue
			}
		}

		// Calculate statistics
		originalBounds := img.Bounds()
		stats := CompressionStats{
			OriginalSizeKB:   estimateImageSizeKB(originalBounds),
			CompressedSizeKB: len(compressedData) / 1024,
			CompressionRatio: float64(len(compressedData)) / float64(estimateImageSizeKB(originalBounds)*1024),
			Quality:          opts.Quality,
			Duration:         time.Duration(0), // Will be calculated per batch
			Format:           opts.Format,
		}

		compressed := &CompressedScreenshot{
			ID:               m.generateCompressionID(path),
			OriginalPath:     path,
			CompressedPath:   compressedPath,
			CompressionStats: stats,
			CreatedAt:        time.Now(),
			Options:          opts,
		}

		results = append(results, compressed)

		if m.enableLogging {
			m.logCompression(profile, path, stats)
		}

		// Progress logging
		if i%10 == 0 || i == len(screenshotPaths)-1 {
			m.logProgress("batch", i+1, len(screenshotPaths))
		}
	}

	return results, nil
}

// BatchCompressWithContext compresses screenshots with context for cancellation.
func (m *ScreenshotCompressionManager) BatchCompressWithContext(ctx context.Context, screenshotPaths []string, profile string) ([]*CompressedScreenshot, error) {
	if len(screenshotPaths) == 0 {
		return []*CompressedScreenshot{}, nil
	}

	// Load all images first
	images := make([]image.Image, 0, len(screenshotPaths))
	validPaths := make([]string, 0, len(screenshotPaths))

	for _, path := range screenshotPaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		img, err := m.loadImageFromFile(path)
		if err != nil {
			m.logError("batch-load", path, err)
			continue
		}

		images = append(images, img)
		validPaths = append(validPaths, path)
	}

	// Get compression options
	opts, err := m.getProfileOptions(profile)
	if err != nil {
		return nil, fmt.Errorf("invalid compression profile %s: %w", profile, err)
	}

	// Compress batch with context
	compressedDataList, err := m.compressor.CompressBatchWithContext(ctx, images, opts)
	if err != nil {
		return nil, fmt.Errorf("batch compression failed: %w", err)
	}

	// Process results
	results := make([]*CompressedScreenshot, 0, len(compressedDataList))

	for i, compressedData := range compressedDataList {
		if compressedData == nil || i >= len(validPaths) {
			continue
		}

		path := validPaths[i]

		// Save compressed data if not email profile
		var compressedPath string
		if profile != "email" {
			compressedPath = m.generateCompressedPath(path, profile)
			if err := m.saveCompressedData(compressedData, compressedPath); err != nil {
				m.logError("batch-save", path, err)
				continue
			}
		}

		// Calculate statistics
		originalBounds := images[i].Bounds()
		stats := CompressionStats{
			OriginalSizeKB:   estimateImageSizeKB(originalBounds),
			CompressedSizeKB: len(compressedData) / 1024,
			CompressionRatio: float64(len(compressedData)) / float64(estimateImageSizeKB(originalBounds)*1024),
			Quality:          opts.Quality,
			Duration:         time.Duration(0),
			Format:           opts.Format,
		}

		compressed := &CompressedScreenshot{
			ID:               m.generateCompressionID(path),
			OriginalPath:     path,
			CompressedPath:   compressedPath,
			CompressionStats: stats,
			CreatedAt:        time.Now(),
			Options:          opts,
		}

		results = append(results, compressed)
	}

	return results, nil
}

// CleanupTempFiles removes temporary compression files older than the specified duration.
func (m *ScreenshotCompressionManager) CleanupTempFiles(olderThan time.Duration) error {
	if _, err := os.Stat(m.tempDir); os.IsNotExist(err) {
		return nil // Nothing to clean
	}

	cutoff := time.Now().Add(-olderThan)

	return filepath.Walk(m.tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				m.logError("cleanup", path, err)
			} else if m.enableLogging {
				m.logCleanup(path)
			}
		}

		return nil
	})
}

// Helper methods

// loadImageFromFile loads an image from a file path.
func (m *ScreenshotCompressionManager) loadImageFromFile(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}

// saveCompressedData saves compressed image data to a file.
func (m *ScreenshotCompressionManager) saveCompressedData(data []byte, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// generateCompressionID generates a unique ID for compression operations.
func (m *ScreenshotCompressionManager) generateCompressionID(originalPath string) string {
	base := filepath.Base(originalPath)
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("comp_%s_%s", timestamp, base)
}

// generateCompressedPath generates a path for compressed images.
func (m *ScreenshotCompressionManager) generateCompressedPath(originalPath, profile string) string {
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	// Create compressed subdirectory
	compressedDir := filepath.Join(dir, "compressed", profile)

	// Use .jpg for JPEG format
	newExt := ".jpg"
	if profile == "png" {
		newExt = ".png"
	}

	return filepath.Join(compressedDir, name+"_"+profile+newExt)
}

// getProfileOptions returns compression options for a given profile.
func (m *ScreenshotCompressionManager) getProfileOptions(profile string) (CompressionOptions, error) {
	switch profile {
	case "email":
		return GetEmailOptimizedOptions(), nil
	case "web":
		return CompressionOptions{
			Quality:             85,
			Format:              "jpeg",
			MaxWidth:            1920,
			MaxHeight:           1080,
			PreserveAspectRatio: true,
			MaxSizeKB:           800,
		}, nil
	case "thumbnail":
		return CompressionOptions{
			Quality:             75,
			Format:              "jpeg",
			MaxWidth:            300,
			MaxHeight:           200,
			PreserveAspectRatio: true,
			MaxSizeKB:           50,
		}, nil
	case "archive":
		return CompressionOptions{
			Quality:             60,
			Format:              "jpeg",
			MaxWidth:            0, // No resizing
			MaxHeight:           0, // No resizing
			PreserveAspectRatio: true,
			MaxSizeKB:           0, // No size limit
		}, nil
	default:
		return CompressionOptions{}, fmt.Errorf("unknown profile: %s", profile)
	}
}

// Logging methods

func (m *ScreenshotCompressionManager) logCompression(profile, path string, stats CompressionStats) {
	if !m.enableLogging {
		return
	}
	// In production, integrate with your logging system
	fmt.Printf("[COMPRESSION] %s: %s - %s\n", profile, filepath.Base(path), stats.String())
}

func (m *ScreenshotCompressionManager) logError(operation, path string, err error) {
	if !m.enableLogging {
		return
	}
	// In production, integrate with your logging system
	fmt.Printf("[COMPRESSION-ERROR] %s: %s - %v\n", operation, filepath.Base(path), err)
}

func (m *ScreenshotCompressionManager) logProgress(operation string, current, total int) {
	if !m.enableLogging {
		return
	}
	// In production, integrate with your logging system
	fmt.Printf("[COMPRESSION-PROGRESS] %s: %d/%d (%.1f%%)\n",
		operation, current, total, float64(current)/float64(total)*100)
}

func (m *ScreenshotCompressionManager) logCleanup(path string) {
	if !m.enableLogging {
		return
	}
	// In production, integrate with your logging system
	fmt.Printf("[COMPRESSION-CLEANUP] Removed: %s\n", filepath.Base(path))
}

// EmailAttachmentHelper provides utilities for email attachment compression.
type EmailAttachmentHelper struct {
	manager *ScreenshotCompressionManager
}

// NewEmailAttachmentHelper creates a new email attachment helper.
func NewEmailAttachmentHelper(storageDir string) *EmailAttachmentHelper {
	return &EmailAttachmentHelper{
		manager: NewScreenshotCompressionManager(storageDir),
	}
}

// PrepareScreenshotsForEmail compresses multiple screenshots for email attachment.
// It returns the compressed data and total size information.
func (h *EmailAttachmentHelper) PrepareScreenshotsForEmail(screenshotPaths []string, maxTotalSizeKB int) ([][]byte, []CompressionStats, error) {
	if len(screenshotPaths) == 0 {
		return [][]byte{}, []CompressionStats{}, nil
	}

	compressedData := make([][]byte, 0, len(screenshotPaths))
	allStats := make([]CompressionStats, 0, len(screenshotPaths))
	totalSizeKB := 0

	for _, path := range screenshotPaths {
		// Compress for email
		_, data, err := h.manager.CompressScreenshotForEmail(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to compress %s for email: %w", path, err)
		}

		sizeKB := len(data) / 1024

		// Check if adding this image would exceed the limit
		if maxTotalSizeKB > 0 && totalSizeKB+sizeKB > maxTotalSizeKB {
			// Try with more aggressive compression
			aggressiveData, err := h.compressAggressively(path, maxTotalSizeKB-totalSizeKB)
			if err != nil || len(aggressiveData) == 0 {
				break // Skip this image
			}
			data = aggressiveData
			sizeKB = len(data) / 1024
		}

		compressedData = append(compressedData, data)

		// Create stats
		stats := CompressionStats{
			CompressedSizeKB: sizeKB,
			Format:           "jpeg",
		}
		allStats = append(allStats, stats)

		totalSizeKB += sizeKB
	}

	return compressedData, allStats, nil
}

// compressAggressively applies very aggressive compression to fit within size limits.
func (h *EmailAttachmentHelper) compressAggressively(path string, maxSizeKB int) ([]byte, error) {
	img, err := h.manager.loadImageFromFile(path)
	if err != nil {
		return nil, err
	}

	opts := CompressionOptions{
		Quality:             30, // Very low quality
		Format:              "jpeg",
		MaxWidth:            800, // Smaller size
		MaxHeight:           600,
		PreserveAspectRatio: true,
		MaxSizeKB:           maxSizeKB,
	}

	return h.manager.compressor.CompressImage(img, opts)
}
