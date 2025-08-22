// Package compression provides production-ready image compression functionality
// for the screenshot server application. It supports JPEG compression with
// configurable quality, image resizing, concurrent batch processing, and
// comprehensive error handling with timeout controls.
package compression

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

const (
	// DefaultWorkerCount is the default number of worker goroutines for batch compression
	DefaultWorkerCount = 4

	// DefaultTimeout is the default timeout for compression operations
	DefaultTimeout = 30 * time.Second

	// MaxImageDimension is the maximum allowed dimension to prevent memory bombs
	MaxImageDimension = 8192

	// MaxImageMemoryMB is the maximum allowed memory per image in MB
	MaxImageMemoryMB = 100

	// MinQuality is the minimum JPEG quality value
	MinQuality = 1

	// MaxQuality is the maximum JPEG quality value
	MaxQuality = 100

	// DefaultQuality is the default JPEG quality value
	DefaultQuality = 85
)

// Compressor defines the interface for image compression operations.
type Compressor interface {
	// CompressImage compresses a single image with the given options
	CompressImage(src image.Image, opts CompressionOptions) ([]byte, error)

	// CompressBatch compresses multiple images concurrently with the given options
	CompressBatch(images []image.Image, opts CompressionOptions) ([][]byte, error)

	// CompressImageWithContext compresses a single image with context for cancellation
	CompressImageWithContext(ctx context.Context, src image.Image, opts CompressionOptions) ([]byte, error)

	// CompressBatchWithContext compresses multiple images with context for cancellation
	CompressBatchWithContext(ctx context.Context, images []image.Image, opts CompressionOptions) ([][]byte, error)
}

// CompressionOptions defines configuration options for image compression.
type CompressionOptions struct {
	// Quality sets JPEG compression quality (1-100, higher is better quality)
	Quality int `json:"quality" yaml:"quality"`

	// MaxWidth sets maximum pixel width for resizing (0 = no limit)
	MaxWidth int `json:"max_width" yaml:"max_width"`

	// MaxHeight sets maximum pixel height for resizing (0 = no limit)
	MaxHeight int `json:"max_height" yaml:"max_height"`

	// Format specifies output format ("jpeg", "png")
	Format string `json:"format" yaml:"format"`

	// MaxSizeKB sets target maximum size in KB (0 = no limit)
	// If set, quality will be automatically reduced to meet this target
	MaxSizeKB int `json:"max_size_kb" yaml:"max_size_kb"`

	// PreserveAspectRatio determines if aspect ratio should be maintained during resize
	PreserveAspectRatio bool `json:"preserve_aspect_ratio" yaml:"preserve_aspect_ratio"`

	// WorkerCount sets number of workers for batch operations (0 = default)
	WorkerCount int `json:"worker_count" yaml:"worker_count"`

	// Timeout sets operation timeout (0 = default)
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// CompressResult represents the result of a compression operation.
type CompressResult struct {
	Data     []byte
	SizeKB   int
	Quality  int
	Width    int
	Height   int
	Format   string
	Duration time.Duration
}

// BatchResult represents the result of a batch compression operation.
type BatchResult struct {
	Results   []CompressResult
	Errors    []error
	Duration  time.Duration
	Processed int
	Failed    int
}

// ProgressCallback is called during batch operations to report progress.
type ProgressCallback func(completed, total int)

// DefaultCompressor implements the Compressor interface with production-ready
// image compression capabilities including memory management, timeouts, and
// concurrent processing.
type DefaultCompressor struct {
	// maxMemoryMB limits memory usage per operation
	maxMemoryMB int

	// defaultTimeout is the default timeout for operations
	defaultTimeout time.Duration

	// mutex protects concurrent access to internal state
	mutex sync.RWMutex
}

// NewCompressor creates a new DefaultCompressor with production-ready defaults.
func NewCompressor() *DefaultCompressor {
	return &DefaultCompressor{
		maxMemoryMB:    MaxImageMemoryMB,
		defaultTimeout: DefaultTimeout,
	}
}

// NewCompressorWithOptions creates a new DefaultCompressor with custom options.
func NewCompressorWithOptions(maxMemoryMB int, defaultTimeout time.Duration) *DefaultCompressor {
	if maxMemoryMB <= 0 {
		maxMemoryMB = MaxImageMemoryMB
	}
	if defaultTimeout <= 0 {
		defaultTimeout = DefaultTimeout
	}

	return &DefaultCompressor{
		maxMemoryMB:    maxMemoryMB,
		defaultTimeout: defaultTimeout,
	}
}

// CompressImage compresses a single image with the given options.
func (c *DefaultCompressor) CompressImage(src image.Image, opts CompressionOptions) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout(opts))
	defer cancel()

	return c.CompressImageWithContext(ctx, src, opts)
}

// CompressBatch compresses multiple images concurrently with the given options.
func (c *DefaultCompressor) CompressBatch(images []image.Image, opts CompressionOptions) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout(opts))
	defer cancel()

	return c.CompressBatchWithContext(ctx, images, opts)
}

// CompressImageWithContext compresses a single image with context for cancellation.
func (c *DefaultCompressor) CompressImageWithContext(ctx context.Context, src image.Image, opts CompressionOptions) ([]byte, error) {
	start := time.Now()

	// Validate input parameters
	if err := c.validateImage(src); err != nil {
		return nil, fmt.Errorf("image validation failed: %w", err)
	}

	if err := c.validateOptions(opts); err != nil {
		return nil, fmt.Errorf("options validation failed: %w", err)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Resize image if needed
	processed := src
	if opts.MaxWidth > 0 || opts.MaxHeight > 0 {
		var err error
		processed, err = c.resizeImage(processed, opts.MaxWidth, opts.MaxHeight, opts.PreserveAspectRatio)
		if err != nil {
			return nil, fmt.Errorf("image resize failed: %w", err)
		}
	}

	// Check context cancellation after resize
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Compress with adaptive quality if size limit is specified
	if opts.MaxSizeKB > 0 {
		return c.compressWithSizeLimit(ctx, processed, opts)
	}

	// Standard compression
	data, err := c.encodeImage(processed, opts.Format, opts.Quality)
	if err != nil {
		return nil, fmt.Errorf("image encoding failed: %w", err)
	}

	// Log compression result for monitoring
	duration := time.Since(start)
	sizeKB := len(data) / 1024
	c.logCompression(src.Bounds(), processed.Bounds(), sizeKB, opts.Quality, duration)

	return data, nil
}

// CompressBatchWithContext compresses multiple images with context for cancellation.
func (c *DefaultCompressor) CompressBatchWithContext(ctx context.Context, images []image.Image, opts CompressionOptions) ([][]byte, error) {
	if len(images) == 0 {
		return [][]byte{}, nil
	}

	workerCount := opts.WorkerCount
	if workerCount <= 0 {
		workerCount = DefaultWorkerCount
	}

	// Limit worker count to avoid excessive goroutine creation
	if workerCount > len(images) {
		workerCount = len(images)
	}

	results := make([][]byte, len(images))
	errors := make([]error, len(images))

	// Create job channel and result channels
	jobs := make(chan int, len(images))
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for imageIndex := range jobs {
				select {
				case <-ctx.Done():
					errors[imageIndex] = ctx.Err()
					return
				default:
				}

				data, err := c.CompressImageWithContext(ctx, images[imageIndex], opts)
				if err != nil {
					errors[imageIndex] = err
				} else {
					results[imageIndex] = data
				}
			}
		}()
	}

	// Send jobs
	for i := range images {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			return nil, ctx.Err()
		}
	}
	close(jobs)

	// Wait for completion
	wg.Wait()

	// Check for errors
	var firstError error
	for i, err := range errors {
		if err != nil && firstError == nil {
			firstError = fmt.Errorf("batch compression failed at index %d: %w", i, err)
		}
	}

	return results, firstError
}

// validateImage performs security and memory validation on the input image.
func (c *DefaultCompressor) validateImage(img image.Image) error {
	if img == nil {
		return fmt.Errorf("image is nil")
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check maximum dimensions to prevent memory bombs
	if width > MaxImageDimension || height > MaxImageDimension {
		return fmt.Errorf("image dimensions too large: %dx%d (max: %d)", width, height, MaxImageDimension)
	}

	// Estimate memory usage (4 bytes per pixel for RGBA)
	estimatedMemoryMB := (width * height * 4) / (1024 * 1024)
	if estimatedMemoryMB > c.maxMemoryMB {
		return fmt.Errorf("image requires too much memory: %dMB (max: %dMB)", estimatedMemoryMB, c.maxMemoryMB)
	}

	return nil
}

// validateOptions validates compression options.
func (c *DefaultCompressor) validateOptions(opts CompressionOptions) error {
	// Validate quality
	if opts.Quality < MinQuality || opts.Quality > MaxQuality {
		return fmt.Errorf("quality must be between %d and %d, got %d", MinQuality, MaxQuality, opts.Quality)
	}

	// Validate format
	switch opts.Format {
	case "jpeg", "png":
		// Valid formats
	case "":
		// Default to JPEG
	default:
		return fmt.Errorf("unsupported format: %s (supported: jpeg, png)", opts.Format)
	}

	// Validate dimensions
	if opts.MaxWidth < 0 || opts.MaxHeight < 0 {
		return fmt.Errorf("dimensions cannot be negative")
	}

	if opts.MaxSizeKB < 0 {
		return fmt.Errorf("max size cannot be negative")
	}

	return nil
}

// resizeImage resizes an image to fit within the specified dimensions.
func (c *DefaultCompressor) resizeImage(src image.Image, maxWidth, maxHeight int, preserveAspect bool) (image.Image, error) {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// Calculate target dimensions
	targetWidth, targetHeight := c.calculateTargetSize(srcWidth, srcHeight, maxWidth, maxHeight, preserveAspect)

	// Skip resize if no change needed
	if targetWidth == srcWidth && targetHeight == srcHeight {
		return src, nil
	}

	// Create destination image
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// Use high-quality scaling
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, srcBounds, draw.Over, nil)

	return dst, nil
}

// calculateTargetSize calculates the target dimensions for resizing.
func (c *DefaultCompressor) calculateTargetSize(srcWidth, srcHeight, maxWidth, maxHeight int, preserveAspect bool) (int, int) {
	if maxWidth <= 0 && maxHeight <= 0 {
		return srcWidth, srcHeight
	}

	if !preserveAspect {
		width := srcWidth
		height := srcHeight

		if maxWidth > 0 && width > maxWidth {
			width = maxWidth
		}
		if maxHeight > 0 && height > maxHeight {
			height = maxHeight
		}

		return width, height
	}

	// Preserve aspect ratio
	scaleX := float64(maxWidth) / float64(srcWidth)
	scaleY := float64(maxHeight) / float64(srcHeight)

	// Handle unlimited dimensions
	if maxWidth <= 0 {
		scaleX = scaleY
	}
	if maxHeight <= 0 {
		scaleY = scaleX
	}

	// Use the smaller scale to fit within both constraints
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	// Don't upscale
	if scale > 1.0 {
		scale = 1.0
	}

	targetWidth := int(float64(srcWidth) * scale)
	targetHeight := int(float64(srcHeight) * scale)

	// Ensure minimum size of 1x1
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}

	return targetWidth, targetHeight
}

// compressWithSizeLimit compresses an image with adaptive quality to meet size constraints.
func (c *DefaultCompressor) compressWithSizeLimit(ctx context.Context, img image.Image, opts CompressionOptions) ([]byte, error) {
	targetSizeBytes := opts.MaxSizeKB * 1024
	quality := opts.Quality

	// Binary search for optimal quality
	minQuality := MinQuality
	maxQuality := quality
	var bestData []byte

	for attempts := 0; attempts < 10 && minQuality <= maxQuality; attempts++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		testQuality := (minQuality + maxQuality) / 2
		data, err := c.encodeImage(img, opts.Format, testQuality)
		if err != nil {
			return nil, fmt.Errorf("encoding failed at quality %d: %w", testQuality, err)
		}

		if len(data) <= targetSizeBytes {
			bestData = data
			minQuality = testQuality + 1
		} else {
			maxQuality = testQuality - 1
		}
	}

	if bestData == nil {
		// If we can't meet the size limit, try minimum quality
		data, err := c.encodeImage(img, opts.Format, MinQuality)
		if err != nil {
			return nil, fmt.Errorf("encoding failed at minimum quality: %w", err)
		}
		bestData = data
	}

	return bestData, nil
}

// encodeImage encodes an image to the specified format with the given quality.
func (c *DefaultCompressor) encodeImage(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer

	// Default to JPEG if format is empty
	if format == "" {
		format = "jpeg"
	}

	switch format {
	case "jpeg":
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		if err != nil {
			return nil, fmt.Errorf("JPEG encoding failed: %w", err)
		}
	case "png":
		err := png.Encode(&buf, img)
		if err != nil {
			return nil, fmt.Errorf("PNG encoding failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return buf.Bytes(), nil
}

// getTimeout returns the timeout for operations.
func (c *DefaultCompressor) getTimeout(opts CompressionOptions) time.Duration {
	if opts.Timeout > 0 {
		return opts.Timeout
	}
	return c.defaultTimeout
}

// logCompression logs compression results for monitoring and debugging.
func (c *DefaultCompressor) logCompression(originalBounds, finalBounds image.Rectangle, sizeKB, quality int, duration time.Duration) {
	// In production, this would integrate with your logging system
	// For now, this is a placeholder for monitoring integration
	_ = originalBounds
	_ = finalBounds
	_ = sizeKB
	_ = quality
	_ = duration
}

// GetDefaultOptions returns default compression options optimized for screenshot compression.
func GetDefaultOptions() CompressionOptions {
	return CompressionOptions{
		Quality:             DefaultQuality,
		MaxWidth:            0, // No resize by default
		MaxHeight:           0, // No resize by default
		Format:              "jpeg",
		MaxSizeKB:           0, // No size limit by default
		PreserveAspectRatio: true,
		WorkerCount:         DefaultWorkerCount,
		Timeout:             DefaultTimeout,
	}
}

// GetEmailOptimizedOptions returns compression options optimized for email attachments.
func GetEmailOptimizedOptions() CompressionOptions {
	return CompressionOptions{
		Quality:             70,   // Balanced quality for email
		MaxWidth:            1920, // Reasonable max width
		MaxHeight:           1080, // Reasonable max height
		Format:              "jpeg",
		MaxSizeKB:           500, // 500KB limit for email attachments
		PreserveAspectRatio: true,
		WorkerCount:         DefaultWorkerCount,
		Timeout:             DefaultTimeout,
	}
}

// CompressImageFromBytes is a convenience function to compress image data directly.
func CompressImageFromBytes(data []byte, opts CompressionOptions) ([]byte, error) {
	// Decode the image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Compress using default compressor
	compressor := NewCompressor()
	return compressor.CompressImage(img, opts)
}

// CompressImageFromReader is a convenience function to compress image data from a reader.
func CompressImageFromReader(reader io.Reader, opts CompressionOptions) ([]byte, error) {
	// Decode the image
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Compress using default compressor
	compressor := NewCompressor()
	return compressor.CompressImage(img, opts)
}
