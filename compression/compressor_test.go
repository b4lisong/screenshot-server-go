package compression

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

// createTestImage creates a simple test image for testing purposes.
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Create a simple pattern with different colors
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a checkerboard pattern
			if (x/10+y/10)%2 == 0 {
				img.Set(x, y, color.RGBA{255, 0, 0, 255}) // Red
			} else {
				img.Set(x, y, color.RGBA{0, 255, 0, 255}) // Green
			}
		}
	}

	return img
}

// createTestImageBytes creates test image data as bytes.
func createTestImageBytes(width, height int) []byte {
	img := createTestImage(width, height)
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestNewCompressor(t *testing.T) {
	compressor := NewCompressor()
	if compressor == nil {
		t.Fatal("NewCompressor returned nil")
	}

	if compressor.maxMemoryMB != MaxImageMemoryMB {
		t.Errorf("Expected maxMemoryMB to be %d, got %d", MaxImageMemoryMB, compressor.maxMemoryMB)
	}

	if compressor.defaultTimeout != DefaultTimeout {
		t.Errorf("Expected defaultTimeout to be %v, got %v", DefaultTimeout, compressor.defaultTimeout)
	}
}

func TestNewCompressorWithOptions(t *testing.T) {
	customMemory := 50
	customTimeout := 60 * time.Second

	compressor := NewCompressorWithOptions(customMemory, customTimeout)

	if compressor.maxMemoryMB != customMemory {
		t.Errorf("Expected maxMemoryMB to be %d, got %d", customMemory, compressor.maxMemoryMB)
	}

	if compressor.defaultTimeout != customTimeout {
		t.Errorf("Expected defaultTimeout to be %v, got %v", customTimeout, compressor.defaultTimeout)
	}
}

func TestCompressImage(t *testing.T) {
	compressor := NewCompressor()
	testImage := createTestImage(100, 100)

	tests := []struct {
		name    string
		opts    CompressionOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_jpeg_compression",
			opts: CompressionOptions{
				Quality: 80,
				Format:  "jpeg",
			},
			wantErr: false,
		},
		{
			name: "valid_png_compression",
			opts: CompressionOptions{
				Quality: 80,
				Format:  "png",
			},
			wantErr: false,
		},
		{
			name: "invalid_quality_too_low",
			opts: CompressionOptions{
				Quality: 0,
				Format:  "jpeg",
			},
			wantErr: true,
			errMsg:  "quality must be between",
		},
		{
			name: "invalid_quality_too_high",
			opts: CompressionOptions{
				Quality: 101,
				Format:  "jpeg",
			},
			wantErr: true,
			errMsg:  "quality must be between",
		},
		{
			name: "invalid_format",
			opts: CompressionOptions{
				Quality: 80,
				Format:  "invalid",
			},
			wantErr: true,
			errMsg:  "unsupported format",
		},
		{
			name: "with_resize",
			opts: CompressionOptions{
				Quality:             80,
				Format:              "jpeg",
				MaxWidth:            50,
				MaxHeight:           50,
				PreserveAspectRatio: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := compressor.CompressImage(testImage, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(data) == 0 {
				t.Error("Compressed data is empty")
			}
		})
	}
}

func TestCompressImageWithSizeLimit(t *testing.T) {
	compressor := NewCompressor()
	testImage := createTestImage(200, 200)

	opts := CompressionOptions{
		Quality:   90,
		Format:    "jpeg",
		MaxSizeKB: 10, // Very small limit to force quality reduction
	}

	data, err := compressor.CompressImage(testImage, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	sizeKB := len(data) / 1024
	if sizeKB > opts.MaxSizeKB {
		t.Errorf("Compressed image size %dKB exceeds limit %dKB", sizeKB, opts.MaxSizeKB)
	}
}

func TestCompressBatch(t *testing.T) {
	compressor := NewCompressor()

	// Create test images
	images := []image.Image{
		createTestImage(50, 50),
		createTestImage(100, 100),
		createTestImage(75, 75),
	}

	opts := CompressionOptions{
		Quality:     80,
		Format:      "jpeg",
		WorkerCount: 2,
	}

	results, err := compressor.CompressBatch(images, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != len(images) {
		t.Errorf("Expected %d results, got %d", len(images), len(results))
	}

	for i, data := range results {
		if len(data) == 0 {
			t.Errorf("Result %d is empty", i)
		}
	}
}

func TestCompressBatchEmpty(t *testing.T) {
	compressor := NewCompressor()

	opts := CompressionOptions{
		Quality: 80,
		Format:  "jpeg",
	}

	results, err := compressor.CompressBatch([]image.Image{}, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results for empty input, got %d results", len(results))
	}
}

func TestCompressImageWithContext(t *testing.T) {
	compressor := NewCompressor()
	testImage := createTestImage(100, 100)

	t.Run("normal_operation", func(t *testing.T) {
		ctx := context.Background()
		opts := CompressionOptions{
			Quality: 80,
			Format:  "jpeg",
		}

		data, err := compressor.CompressImageWithContext(ctx, testImage, opts)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(data) == 0 {
			t.Error("Compressed data is empty")
		}
	})

	t.Run("cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		opts := CompressionOptions{
			Quality: 80,
			Format:  "jpeg",
		}

		_, err := compressor.CompressImageWithContext(ctx, testImage, opts)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})
}

func TestValidateImage(t *testing.T) {
	compressor := NewCompressor()

	tests := []struct {
		name    string
		img     image.Image
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil_image",
			img:     nil,
			wantErr: true,
			errMsg:  "image is nil",
		},
		{
			name:    "valid_image",
			img:     createTestImage(100, 100),
			wantErr: false,
		},
		{
			name:    "too_large_image",
			img:     createTestImage(MaxImageDimension+1, 100),
			wantErr: true,
			errMsg:  "image dimensions too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compressor.validateImage(tt.img)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCalculateTargetSize(t *testing.T) {
	compressor := NewCompressor()

	tests := []struct {
		name           string
		srcWidth       int
		srcHeight      int
		maxWidth       int
		maxHeight      int
		preserveAspect bool
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "no_limits",
			srcWidth:       100,
			srcHeight:      200,
			maxWidth:       0,
			maxHeight:      0,
			preserveAspect: true,
			expectedWidth:  100,
			expectedHeight: 200,
		},
		{
			name:           "width_limit_preserve_aspect",
			srcWidth:       200,
			srcHeight:      100,
			maxWidth:       100,
			maxHeight:      0,
			preserveAspect: true,
			expectedWidth:  100,
			expectedHeight: 50,
		},
		{
			name:           "height_limit_preserve_aspect",
			srcWidth:       100,
			srcHeight:      200,
			maxWidth:       0,
			maxHeight:      100,
			preserveAspect: true,
			expectedWidth:  50,
			expectedHeight: 100,
		},
		{
			name:           "both_limits_preserve_aspect",
			srcWidth:       200,
			srcHeight:      200,
			maxWidth:       100,
			maxHeight:      150,
			preserveAspect: true,
			expectedWidth:  100,
			expectedHeight: 100,
		},
		{
			name:           "both_limits_no_preserve_aspect",
			srcWidth:       200,
			srcHeight:      200,
			maxWidth:       100,
			maxHeight:      150,
			preserveAspect: false,
			expectedWidth:  100,
			expectedHeight: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := compressor.calculateTargetSize(
				tt.srcWidth, tt.srcHeight,
				tt.maxWidth, tt.maxHeight,
				tt.preserveAspect,
			)

			if width != tt.expectedWidth || height != tt.expectedHeight {
				t.Errorf("Expected %dx%d, got %dx%d",
					tt.expectedWidth, tt.expectedHeight,
					width, height)
			}
		})
	}
}

func TestGetDefaultOptions(t *testing.T) {
	opts := GetDefaultOptions()

	if opts.Quality != DefaultQuality {
		t.Errorf("Expected quality %d, got %d", DefaultQuality, opts.Quality)
	}

	if opts.Format != "jpeg" {
		t.Errorf("Expected format 'jpeg', got '%s'", opts.Format)
	}

	if !opts.PreserveAspectRatio {
		t.Error("Expected PreserveAspectRatio to be true")
	}
}

func TestGetEmailOptimizedOptions(t *testing.T) {
	opts := GetEmailOptimizedOptions()

	if opts.Quality != 70 {
		t.Errorf("Expected quality 70, got %d", opts.Quality)
	}

	if opts.MaxSizeKB != 500 {
		t.Errorf("Expected MaxSizeKB 500, got %d", opts.MaxSizeKB)
	}

	if opts.MaxWidth != 1920 {
		t.Errorf("Expected MaxWidth 1920, got %d", opts.MaxWidth)
	}

	if opts.MaxHeight != 1080 {
		t.Errorf("Expected MaxHeight 1080, got %d", opts.MaxHeight)
	}
}

func TestCompressImageFromBytes(t *testing.T) {
	imageData := createTestImageBytes(100, 100)

	opts := CompressionOptions{
		Quality: 80,
		Format:  "jpeg",
	}

	compressedData, err := CompressImageFromBytes(imageData, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(compressedData) == 0 {
		t.Error("Compressed data is empty")
	}
}

func TestCompressImageFromBytesInvalidData(t *testing.T) {
	invalidData := []byte("not an image")

	opts := CompressionOptions{
		Quality: 80,
		Format:  "jpeg",
	}

	_, err := CompressImageFromBytes(invalidData, opts)
	if err == nil {
		t.Error("Expected error for invalid image data")
	}

	if !contains(err.Error(), "failed to decode image") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestEncodeImage(t *testing.T) {
	compressor := NewCompressor()
	testImage := createTestImage(50, 50)

	tests := []struct {
		name    string
		format  string
		quality int
		wantErr bool
	}{
		{
			name:    "jpeg_encoding",
			format:  "jpeg",
			quality: 80,
			wantErr: false,
		},
		{
			name:    "png_encoding",
			format:  "png",
			quality: 80, // Quality ignored for PNG
			wantErr: false,
		},
		{
			name:    "empty_format_defaults_to_jpeg",
			format:  "",
			quality: 80,
			wantErr: false,
		},
		{
			name:    "invalid_format",
			format:  "invalid",
			quality: 80,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := compressor.encodeImage(testImage, tt.format, tt.quality)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(data) == 0 {
				t.Error("Encoded data is empty")
			}
		})
	}
}

// Benchmarks

func BenchmarkCompressImage(b *testing.B) {
	compressor := NewCompressor()
	testImage := createTestImage(1920, 1080)
	opts := CompressionOptions{
		Quality: 80,
		Format:  "jpeg",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(testImage, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImageWithResize(b *testing.B) {
	compressor := NewCompressor()
	testImage := createTestImage(1920, 1080)
	opts := CompressionOptions{
		Quality:             80,
		Format:              "jpeg",
		MaxWidth:            800,
		MaxHeight:           600,
		PreserveAspectRatio: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(testImage, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressBatch(b *testing.B) {
	compressor := NewCompressor()
	images := make([]image.Image, 10)
	for i := range images {
		images[i] = createTestImage(400, 300)
	}

	opts := CompressionOptions{
		Quality:     80,
		Format:      "jpeg",
		WorkerCount: 4,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressBatch(images, opts)
		if err != nil {
			b.Fatalf("Batch compression failed: %v", err)
		}
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
