# Image Compression Package

This package provides production-ready image compression functionality for the Go screenshot server application. It supports JPEG compression with configurable quality, image resizing, concurrent batch processing, and comprehensive error handling.

## Features

- **JPEG/PNG Compression**: High-quality image compression with configurable quality settings
- **Image Resizing**: Resize images with aspect ratio preservation
- **Batch Processing**: Concurrent compression using worker pools
- **Size-Aware Compression**: Automatically adjust quality to meet size constraints
- **Context Support**: Cancellation and timeout support for all operations
- **Memory Management**: Built-in protection against memory bombs and resource limits
- **Email Optimization**: Specialized settings for email attachments
- **Performance Monitoring**: Built-in statistics and benchmarking support

## Quick Start

### Basic Usage

```go
import "github.com/b4lisong/screenshot-server-go/compression"

// Create a compressor
compressor := compression.NewCompressor()

// Basic compression
opts := compression.CompressionOptions{
    Quality: 85,
    Format:  "jpeg",
}

// Compress from file
data, err := compression.CompressImageFromBytes(imageData, opts)
if err != nil {
    log.Fatal(err)
}
```

### Email-Optimized Compression

```go
// Use email-optimized settings
emailService := compression.NewEmailCompressionService()

// Compress for email attachment
compressedData, stats, err := emailService.CompressForEmail(img)
if err != nil {
    log.Fatal(err)
}

log.Printf("Compression: %s", stats.String())
// Output: Compressed 2048KB → 487KB (76.2% savings, quality=70, 15ms)
```

### Batch Processing

```go
// Compress multiple images concurrently
images := []image.Image{img1, img2, img3}

opts := compression.GetEmailOptimizedOptions()
opts.WorkerCount = 8

results, err := compressor.CompressBatch(images, opts)
if err != nil {
    log.Fatal(err)
}
```

### Advanced Usage with Context

```go
// Compression with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

data, err := compressor.CompressImageWithContext(ctx, img, opts)
if err != nil {
    log.Fatal(err)
}
```

## Compression Options

```go
type CompressionOptions struct {
    Quality             int           // JPEG quality (1-100)
    MaxWidth            int           // Maximum width in pixels
    MaxHeight           int           // Maximum height in pixels
    Format              string        // Output format ("jpeg", "png")
    MaxSizeKB           int           // Target maximum size in KB
    PreserveAspectRatio bool          // Maintain aspect ratio during resize
    WorkerCount         int           // Number of workers for batch operations
    Timeout             time.Duration // Operation timeout
}
```

## Predefined Profiles

### Email Optimized
```go
opts := compression.GetEmailOptimizedOptions()
// Quality: 70, MaxWidth: 1920, MaxHeight: 1080, MaxSizeKB: 500KB
```

### Default Settings
```go
opts := compression.GetDefaultOptions()
// Quality: 85, Format: "jpeg", PreserveAspectRatio: true
```

## Integration with Screenshot Server

### Compress Screenshots for Email

```go
// Create compression manager
manager := compression.NewScreenshotCompressionManager("./screenshots")

// Compress screenshot for email
compressed, data, err := manager.CompressScreenshotForEmail("screenshot.png")
if err != nil {
    log.Fatal(err)
}

// Use data as email attachment
```

### Batch Compress for Web Display

```go
screenshotPaths := []string{"shot1.png", "shot2.png", "shot3.png"}

// Compress for web display
results, err := manager.BatchCompressScreenshots(screenshotPaths, "web")
if err != nil {
    log.Fatal(err)
}

for _, result := range results {
    log.Printf("Compressed: %s", result.CompressionStats.String())
}
```

## Compression Profiles

| Profile   | Quality | Max Size | Max Dimensions | Use Case                |
|-----------|---------|----------|----------------|-------------------------|
| email     | 70      | 500KB    | 1920x1080      | Email attachments       |
| web       | 85      | 800KB    | 1920x1080      | Web display             |
| thumbnail | 75      | 50KB     | 300x200        | Thumbnail generation    |
| archive   | 60      | None     | Original       | Long-term storage       |

## Security Features

- **Memory Bomb Protection**: Validates image dimensions and memory usage
- **Input Validation**: Comprehensive parameter validation
- **Timeout Controls**: Configurable timeouts for all operations
- **Resource Limits**: Configurable memory limits per operation

## Performance Characteristics

- **Concurrent Processing**: Configurable worker pools for batch operations
- **Memory Efficient**: Optimized memory usage with proper cleanup
- **Size Prediction**: Adaptive quality reduction for size constraints
- **Context Cancellation**: Immediate cancellation support

## Error Handling

The package provides comprehensive error handling with context:

```go
data, err := compressor.CompressImage(img, opts)
if err != nil {
    // Errors include validation, encoding, and timeout failures
    log.Printf("Compression failed: %v", err)
}
```

Common error types:
- Image validation errors (nil, too large, invalid format)
- Options validation errors (invalid quality, dimensions)
- Encoding errors (JPEG/PNG encoding failures)
- Context errors (timeout, cancellation)

## Testing and Benchmarks

Run tests:
```bash
go test ./compression/... -v
```

Run benchmarks:
```bash
go test ./compression/... -bench=. -benchmem
```

## Integration Examples

See the `examples.go` file for complete integration examples including:
- Email compression service
- File compression service
- Adaptive compression service
- Progress tracking
- Error handling patterns

## Performance Tuning

### For Email Attachments
- Use `GetEmailOptimizedOptions()` for balanced quality/size
- Set `MaxSizeKB` to enforce size limits
- Use lower quality (60-70) for aggressive compression

### For Batch Processing
- Increase `WorkerCount` for faster processing (default: 4)
- Use context with timeout for large batches
- Consider memory limits for very large images

### For Large Images
- Enable resizing with `MaxWidth`/`MaxHeight`
- Use `PreserveAspectRatio: true` for proper scaling
- Set memory limits with `NewCompressorWithOptions()`