// Package compression provides example usage patterns for the image compression system.
// This file demonstrates common use cases and integration patterns for the screenshot server.
package compression

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"time"
)

// EmailCompressionService provides email-optimized compression for the screenshot server.
type EmailCompressionService struct {
	compressor Compressor
	options    CompressionOptions
}

// NewEmailCompressionService creates a new service optimized for email attachments.
func NewEmailCompressionService() *EmailCompressionService {
	return &EmailCompressionService{
		compressor: NewCompressor(),
		options:    GetEmailOptimizedOptions(),
	}
}

// CompressForEmail compresses a screenshot image for email attachment.
// It automatically applies email-optimized settings and returns the compressed data
// along with compression statistics.
func (s *EmailCompressionService) CompressForEmail(img image.Image) ([]byte, CompressionStats, error) {
	start := time.Now()

	// Compress with email-optimized settings
	data, err := s.compressor.CompressImage(img, s.options)
	if err != nil {
		return nil, CompressionStats{}, fmt.Errorf("email compression failed: %w", err)
	}

	// Calculate compression statistics
	originalBounds := img.Bounds()
	stats := CompressionStats{
		OriginalSizeKB:   estimateImageSizeKB(originalBounds),
		CompressedSizeKB: len(data) / 1024,
		CompressionRatio: float64(len(data)) / float64(estimateImageSizeKB(originalBounds)*1024),
		Quality:          s.options.Quality,
		Duration:         time.Since(start),
		Format:           s.options.Format,
	}

	return data, stats, nil
}

// BatchCompressForEmail compresses multiple images for email with progress tracking.
func (s *EmailCompressionService) BatchCompressForEmail(images []image.Image, progressFn ProgressCallback) ([][]byte, []CompressionStats, error) {
	if len(images) == 0 {
		return [][]byte{}, []CompressionStats{}, nil
	}

	start := time.Now()

	// Compress batch
	results, err := s.compressor.CompressBatch(images, s.options)
	if err != nil {
		return nil, nil, fmt.Errorf("batch email compression failed: %w", err)
	}

	// Calculate statistics for each image
	stats := make([]CompressionStats, len(images))
	for i, img := range images {
		if i < len(results) && results[i] != nil {
			originalBounds := img.Bounds()
			stats[i] = CompressionStats{
				OriginalSizeKB:   estimateImageSizeKB(originalBounds),
				CompressedSizeKB: len(results[i]) / 1024,
				CompressionRatio: float64(len(results[i])) / float64(estimateImageSizeKB(originalBounds)*1024),
				Quality:          s.options.Quality,
				Duration:         time.Since(start) / time.Duration(len(images)), // Average duration
				Format:           s.options.Format,
			}
		}

		// Report progress
		if progressFn != nil {
			progressFn(i+1, len(images))
		}
	}

	return results, stats, nil
}

// CompressionStats contains statistics about a compression operation.
type CompressionStats struct {
	OriginalSizeKB   int           `json:"original_size_kb"`
	CompressedSizeKB int           `json:"compressed_size_kb"`
	CompressionRatio float64       `json:"compression_ratio"`
	Quality          int           `json:"quality"`
	Duration         time.Duration `json:"duration"`
	Format           string        `json:"format"`
}

// SavingsPercent returns the percentage of space saved by compression.
func (s CompressionStats) SavingsPercent() float64 {
	if s.OriginalSizeKB == 0 {
		return 0
	}
	saved := s.OriginalSizeKB - s.CompressedSizeKB
	return (float64(saved) / float64(s.OriginalSizeKB)) * 100
}

// String returns a human-readable summary of compression statistics.
func (s CompressionStats) String() string {
	return fmt.Sprintf("Compressed %dKB → %dKB (%.1f%% savings, quality=%d, %v)",
		s.OriginalSizeKB, s.CompressedSizeKB, s.SavingsPercent(), s.Quality, s.Duration)
}

// FileCompressionService provides file-based compression operations.
type FileCompressionService struct {
	compressor Compressor
}

// NewFileCompressionService creates a new file-based compression service.
func NewFileCompressionService() *FileCompressionService {
	return &FileCompressionService{
		compressor: NewCompressor(),
	}
}

// CompressFile compresses an image file and saves the result to a new file.
func (s *FileCompressionService) CompressFile(inputPath, outputPath string, opts CompressionOptions) error {
	// Open and decode the input file
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file %s: %w", inputPath, err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image from %s: %w", inputPath, err)
	}

	// Compress the image
	compressedData, err := s.compressor.CompressImage(img, opts)
	if err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Write compressed data to output file
	if err := os.WriteFile(outputPath, compressedData, 0644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
	}

	return nil
}

// CompressDirectory compresses all images in a directory with progress tracking.
func (s *FileCompressionService) CompressDirectory(inputDir, outputDir string, opts CompressionOptions, progressFn ProgressCallback) error {
	// Find all image files in the input directory
	imageFiles, err := findImageFiles(inputDir)
	if err != nil {
		return fmt.Errorf("failed to scan input directory: %w", err)
	}

	if len(imageFiles) == 0 {
		return fmt.Errorf("no image files found in %s", inputDir)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Process each file
	for i, inputPath := range imageFiles {
		// Generate output path
		relPath, err := filepath.Rel(inputDir, inputPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", inputPath, err)
		}

		outputPath := filepath.Join(outputDir, relPath)

		// Change extension based on output format
		if opts.Format == "jpeg" {
			outputPath = changeExtension(outputPath, ".jpg")
		} else if opts.Format == "png" {
			outputPath = changeExtension(outputPath, ".png")
		}

		// Compress the file
		if err := s.CompressFile(inputPath, outputPath, opts); err != nil {
			log.Printf("Failed to compress %s: %v", inputPath, err)
			continue
		}

		// Report progress
		if progressFn != nil {
			progressFn(i+1, len(imageFiles))
		}
	}

	return nil
}

// AdaptiveCompressionService provides intelligent compression based on image characteristics.
type AdaptiveCompressionService struct {
	compressor Compressor
}

// NewAdaptiveCompressionService creates a new adaptive compression service.
func NewAdaptiveCompressionService() *AdaptiveCompressionService {
	return &AdaptiveCompressionService{
		compressor: NewCompressor(),
	}
}

// CompressAdaptive automatically chooses compression settings based on image characteristics.
func (s *AdaptiveCompressionService) CompressAdaptive(img image.Image, targetSizeKB int) ([]byte, CompressionStats, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Choose base options based on image size and type
	opts := s.chooseBaseOptions(width, height, targetSizeKB)

	start := time.Now()

	// Compress with chosen options
	data, err := s.compressor.CompressImage(img, opts)
	if err != nil {
		return nil, CompressionStats{}, fmt.Errorf("adaptive compression failed: %w", err)
	}

	// Calculate statistics
	stats := CompressionStats{
		OriginalSizeKB:   estimateImageSizeKB(bounds),
		CompressedSizeKB: len(data) / 1024,
		CompressionRatio: float64(len(data)) / float64(estimateImageSizeKB(bounds)*1024),
		Quality:          opts.Quality,
		Duration:         time.Since(start),
		Format:           opts.Format,
	}

	return data, stats, nil
}

// chooseBaseOptions selects optimal compression options based on image characteristics.
func (s *AdaptiveCompressionService) chooseBaseOptions(width, height, targetSizeKB int) CompressionOptions {
	opts := GetDefaultOptions()

	// Adjust quality based on target size
	if targetSizeKB > 0 {
		opts.MaxSizeKB = targetSizeKB

		// Lower quality for smaller targets
		if targetSizeKB < 100 {
			opts.Quality = 60
		} else if targetSizeKB < 500 {
			opts.Quality = 75
		}
	}

	// Resize very large images
	if width > 2560 || height > 1440 {
		opts.MaxWidth = 2560
		opts.MaxHeight = 1440
		opts.PreserveAspectRatio = true
	} else if width > 1920 || height > 1080 {
		opts.MaxWidth = 1920
		opts.MaxHeight = 1080
		opts.PreserveAspectRatio = true
	}

	return opts
}

// Example usage functions

// ExampleEmailCompression demonstrates compressing screenshots for email.
func ExampleEmailCompression() {
	// Create email compression service
	emailService := NewEmailCompressionService()

	// Load a screenshot (example)
	// img := loadScreenshotImage()

	// Compress for email
	// compressedData, stats, err := emailService.CompressForEmail(img)
	// if err != nil {
	//     log.Fatalf("Email compression failed: %v", err)
	// }

	// log.Printf("Email compression: %s", stats.String())

	// Use compressedData as email attachment
	log.Printf("Example: Email compression service created with quality %d", emailService.options.Quality)
}

// ExampleBatchCompression demonstrates batch compression with progress tracking.
func ExampleBatchCompression() {
	compressor := NewCompressor()

	// Example images (in real usage, load from files or capture screenshots)
	var images []image.Image

	opts := GetEmailOptimizedOptions()
	opts.WorkerCount = 8 // Use more workers for faster processing

	// Compress with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	results, err := compressor.CompressBatchWithContext(ctx, images, opts)
	if err != nil {
		log.Fatalf("Batch compression failed: %v", err)
	}

	log.Printf("Compressed %d images", len(results))
}

// ExampleAdaptiveCompression demonstrates intelligent compression.
func ExampleAdaptiveCompression() {
	adaptiveService := NewAdaptiveCompressionService()

	// Example: compress for different use cases
	targetSizes := []int{
		50,   // Thumbnail
		200,  // Email attachment
		1000, // Web display
		0,    // No size limit
	}

	// var img image.Image // Load your image

	for _, targetKB := range targetSizes {
		// compressedData, stats, err := adaptiveService.CompressAdaptive(img, targetKB)
		// if err != nil {
		//     log.Printf("Adaptive compression failed for target %dKB: %v", targetKB, err)
		//     continue
		// }

		log.Printf("Target %dKB: adaptive compression configured with service %p", targetKB, adaptiveService)
	}
}

// Helper functions

// estimateImageSizeKB estimates the memory size of an image in KB (4 bytes per pixel).
func estimateImageSizeKB(bounds image.Rectangle) int {
	pixels := bounds.Dx() * bounds.Dy()
	return (pixels * 4) / 1024 // 4 bytes per pixel (RGBA)
}

// findImageFiles recursively finds all image files in a directory.
func findImageFiles(dir string) ([]string, error) {
	var imageFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".png", ".jpg", ".jpeg", ".PNG", ".JPG", ".JPEG":
			imageFiles = append(imageFiles, path)
		}

		return nil
	})

	return imageFiles, err
}

// changeExtension changes the file extension while preserving the base name.
func changeExtension(filename, newExt string) string {
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)] + newExt
}
