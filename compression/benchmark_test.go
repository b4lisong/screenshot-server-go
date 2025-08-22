package compression

import (
	"context"
	"image"
	"image/color"
	"runtime"
	"testing"
	"time"
)

// createBenchmarkImage creates a test image with realistic screenshot-like content.
func createBenchmarkImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Create a more realistic pattern resembling a screenshot
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Simulate gradients and text-like patterns
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(((x + y) * 255) / (width + height))

			// Add some noise for realism
			if (x+y)%7 == 0 {
				r = 255
				g = 255
				b = 255
			} else if (x*y)%13 == 0 {
				r = 0
				g = 0
				b = 0
			}

			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	return img
}

// Benchmark single image compression at different sizes and qualities

func BenchmarkCompressImage_Small_HighQuality(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(400, 300)
	opts := CompressionOptions{
		Quality: 95,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_Small_MediumQuality(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(400, 300)
	opts := CompressionOptions{
		Quality: 75,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_Small_LowQuality(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(400, 300)
	opts := CompressionOptions{
		Quality: 50,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_Medium_HighQuality(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1024, 768)
	opts := CompressionOptions{
		Quality: 95,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_Large_HighQuality(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1920, 1080)
	opts := CompressionOptions{
		Quality: 95,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_VeryLarge_MediumQuality(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(2560, 1440)
	opts := CompressionOptions{
		Quality: 75,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

// Benchmark compression with resizing

func BenchmarkCompressImage_WithResize_PreserveAspect(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(2560, 1440)
	opts := CompressionOptions{
		Quality:             85,
		Format:              "jpeg",
		MaxWidth:            1920,
		MaxHeight:           1080,
		PreserveAspectRatio: true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_WithResize_NoAspect(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(2560, 1440)
	opts := CompressionOptions{
		Quality:             85,
		Format:              "jpeg",
		MaxWidth:            1920,
		MaxHeight:           1080,
		PreserveAspectRatio: false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

// Benchmark compression with size limits

func BenchmarkCompressImage_WithSizeLimit_Aggressive(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1920, 1080)
	opts := CompressionOptions{
		Quality:   90,
		Format:    "jpeg",
		MaxSizeKB: 100, // Very aggressive size limit
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_WithSizeLimit_Moderate(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1920, 1080)
	opts := CompressionOptions{
		Quality:   90,
		Format:    "jpeg",
		MaxSizeKB: 500, // Moderate size limit
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

// Benchmark batch compression with different worker counts

func BenchmarkCompressBatch_Workers1(b *testing.B) {
	benchmarkBatchCompressionWorkers(b, 1)
}

func BenchmarkCompressBatch_Workers2(b *testing.B) {
	benchmarkBatchCompressionWorkers(b, 2)
}

func BenchmarkCompressBatch_Workers4(b *testing.B) {
	benchmarkBatchCompressionWorkers(b, 4)
}

func BenchmarkCompressBatch_Workers8(b *testing.B) {
	benchmarkBatchCompressionWorkers(b, 8)
}

func BenchmarkCompressBatch_WorkersAuto(b *testing.B) {
	benchmarkBatchCompressionWorkers(b, runtime.NumCPU())
}

func benchmarkBatchCompressionWorkers(b *testing.B, workers int) {
	compressor := NewCompressor()

	// Create test images
	images := make([]image.Image, 10)
	for i := range images {
		images[i] = createBenchmarkImage(800, 600)
	}

	opts := CompressionOptions{
		Quality:     75,
		Format:      "jpeg",
		WorkerCount: workers,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressBatch(images, opts)
		if err != nil {
			b.Fatalf("Batch compression failed: %v", err)
		}
	}
}

// Benchmark different image formats

func BenchmarkCompressImage_JPEG(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1024, 768)
	opts := CompressionOptions{
		Quality: 85,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("JPEG compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_PNG(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1024, 768)
	opts := CompressionOptions{
		Quality: 85, // Ignored for PNG
		Format:  "png",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("PNG compression failed: %v", err)
		}
	}
}

// Benchmark context cancellation performance

func BenchmarkCompressImage_WithContext(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1024, 768)
	opts := CompressionOptions{
		Quality: 85,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := compressor.CompressImageWithContext(ctx, img, opts)
		if err != nil {
			b.Fatalf("Context compression failed: %v", err)
		}
	}
}

func BenchmarkCompressImage_WithTimeout(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1024, 768)
	opts := CompressionOptions{
		Quality: 85,
		Format:  "jpeg",
		Timeout: 10 * time.Second,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := compressor.CompressImageWithContext(ctx, img, opts)
		cancel()
		if err != nil {
			b.Fatalf("Timeout compression failed: %v", err)
		}
	}
}

// Benchmark convenience functions

func BenchmarkCompressImageFromBytes(b *testing.B) {
	// Create test image data
	testData := createTestImageBytes(800, 600)

	opts := CompressionOptions{
		Quality: 85,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := CompressImageFromBytes(testData, opts)
		if err != nil {
			b.Fatalf("Compression from bytes failed: %v", err)
		}
	}
}

// Benchmark email service

func BenchmarkEmailCompressionService(b *testing.B) {
	service := NewEmailCompressionService()
	img := createBenchmarkImage(1920, 1080)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := service.CompressForEmail(img)
		if err != nil {
			b.Fatalf("Email compression failed: %v", err)
		}
	}
}

func BenchmarkEmailCompressionService_Batch(b *testing.B) {
	service := NewEmailCompressionService()

	// Create test images
	images := make([]image.Image, 5)
	for i := range images {
		images[i] = createBenchmarkImage(1024, 768)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := service.BatchCompressForEmail(images, nil)
		if err != nil {
			b.Fatalf("Batch email compression failed: %v", err)
		}
	}
}

// Benchmark adaptive compression

func BenchmarkAdaptiveCompressionService_Small(b *testing.B) {
	service := NewAdaptiveCompressionService()
	img := createBenchmarkImage(800, 600)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := service.CompressAdaptive(img, 200)
		if err != nil {
			b.Fatalf("Adaptive compression failed: %v", err)
		}
	}
}

func BenchmarkAdaptiveCompressionService_Large(b *testing.B) {
	service := NewAdaptiveCompressionService()
	img := createBenchmarkImage(2560, 1440)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := service.CompressAdaptive(img, 1000)
		if err != nil {
			b.Fatalf("Adaptive compression failed: %v", err)
		}
	}
}

// Memory usage benchmarks

func BenchmarkMemoryUsage_Sequential(b *testing.B) {
	compressor := NewCompressor()
	opts := CompressionOptions{
		Quality: 85,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create image each time to test memory allocation patterns
		img := createBenchmarkImage(1024, 768)
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Sequential compression failed: %v", err)
		}

		// Force garbage collection periodically
		if i%10 == 0 {
			runtime.GC()
		}
	}
}

func BenchmarkMemoryUsage_LargeImages(b *testing.B) {
	compressor := NewCompressor()
	opts := CompressionOptions{
		Quality: 85,
		Format:  "jpeg",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Test with very large images to stress memory management
		img := createBenchmarkImage(3840, 2160) // 4K resolution
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Large image compression failed: %v", err)
		}

		// Force garbage collection after each iteration for large images
		runtime.GC()
	}
}

// Comparative benchmarks for different optimization levels

func BenchmarkOptimizedSettings_EmailProfile(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1920, 1080)
	opts := GetEmailOptimizedOptions()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Email profile compression failed: %v", err)
		}
	}
}

func BenchmarkOptimizedSettings_DefaultProfile(b *testing.B) {
	compressor := NewCompressor()
	img := createBenchmarkImage(1920, 1080)
	opts := GetDefaultOptions()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressImage(img, opts)
		if err != nil {
			b.Fatalf("Default profile compression failed: %v", err)
		}
	}
}
