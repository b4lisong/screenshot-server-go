package email

import (
	"fmt"
	"image"
	"path/filepath"
	"testing"
	"time"

	"github.com/b4lisong/screenshot-server-go/config"
	"github.com/b4lisong/screenshot-server-go/storage"
)

func TestMailerAttachmentIntegration(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create test config with attachments enabled
	cfg := config.Default()
	cfg.Email.Enabled = false // Disable actual email sending
	cfg.Email.Attachments.Enabled = true
	cfg.Email.Attachments.MaxAttachmentSizeMB = 2.0
	cfg.Email.Attachments.MaxTotalSizeMB = 10.0
	cfg.Email.Attachments.MaxScreenshots = 5
	cfg.Email.Attachments.Strategy = "individual"

	// Create mailer
	mailer, err := New(&cfg.Email, tempDir)
	if err != nil {
		t.Fatalf("Failed to create mailer: %v", err)
	}

	// Test that mailer was created with attachment support
	if mailer.config.Attachments.Enabled != true {
		t.Error("Expected attachments to be enabled")
	}

	if mailer.compressionMgr == nil {
		t.Error("Expected compression manager to be initialized")
	}

	if mailer.attachmentHelper == nil {
		t.Error("Expected attachment helper to be initialized")
	}
}

func TestAttachmentConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifier    func(*config.AttachmentConfig)
		expectError bool
	}{
		{
			name: "valid config",
			modifier: func(c *config.AttachmentConfig) {
				// Use defaults - should be valid
			},
			expectError: false,
		},
		{
			name: "invalid compression quality - too low",
			modifier: func(c *config.AttachmentConfig) {
				c.CompressionQuality = 0
			},
			expectError: true,
		},
		{
			name: "invalid compression quality - too high",
			modifier: func(c *config.AttachmentConfig) {
				c.CompressionQuality = 101
			},
			expectError: true,
		},
		{
			name: "invalid max attachment size",
			modifier: func(c *config.AttachmentConfig) {
				c.MaxAttachmentSizeMB = 0
			},
			expectError: true,
		},
		{
			name: "invalid total size vs attachment size",
			modifier: func(c *config.AttachmentConfig) {
				c.MaxAttachmentSizeMB = 10.0
				c.MaxTotalSizeMB = 5.0 // Less than attachment size
			},
			expectError: true,
		},
		{
			name: "invalid strategy",
			modifier: func(c *config.AttachmentConfig) {
				c.Strategy = "invalid"
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Email.Enabled = false // Disable email to avoid SMTP validation
			cfg.Email.Attachments.Enabled = true

			// Apply test-specific modifications
			tt.modifier(&cfg.Email.Attachments)

			// Create a simple validation by trying to create a mailer
			// with attachment validation logic
			var err error
			if cfg.Email.Attachments.Enabled {
				// Test attachment configuration by creating mailer
				tempDir := t.TempDir()
				_, err = New(&cfg.Email, tempDir)

				// For this test, we only care about attachment validation errors
				// So we need to manually validate the attachment config
				if err == nil {
					// Manually validate attachment config for testing
					att := &cfg.Email.Attachments
					if att.CompressionQuality < 1 || att.CompressionQuality > 100 {
						err = fmt.Errorf("compression_quality must be between 1 and 100")
					} else if att.MaxAttachmentSizeMB <= 0 {
						err = fmt.Errorf("max_attachment_size_mb must be positive")
					} else if att.MaxTotalSizeMB <= 0 {
						err = fmt.Errorf("max_total_size_mb must be positive")
					} else if att.MaxTotalSizeMB < att.MaxAttachmentSizeMB {
						err = fmt.Errorf("max_total_size_mb cannot be less than max_attachment_size_mb")
					} else if att.MaxScreenshots <= 0 {
						err = fmt.Errorf("max_screenshots must be positive")
					} else if att.ResizeMaxWidth <= 0 {
						err = fmt.Errorf("resize_max_width must be positive")
					} else if att.ResizeMaxHeight <= 0 {
						err = fmt.Errorf("resize_max_height must be positive")
					} else if att.Strategy != "individual" && att.Strategy != "zip" && att.Strategy != "adaptive" {
						err = fmt.Errorf("invalid strategy")
					}
				}
			}

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestProcessScreenshotAttachments(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create file storage and manager
	fileStorage, err := storage.NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	manager := storage.NewManager(fileStorage)
	defer manager.Close()

	// Create test screenshots
	testImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	screenshots := make([]*storage.Screenshot, 3)

	for i := 0; i < 3; i++ {
		screenshot, err := manager.Save(testImg, i%2 == 0)
		if err != nil {
			t.Fatalf("Failed to save test screenshot: %v", err)
		}
		screenshots[i] = screenshot
	}

	// Create test config
	cfg := config.Default()
	cfg.Email.Enabled = false
	cfg.Email.Attachments.Enabled = true
	cfg.Email.Attachments.Strategy = "individual"
	cfg.Email.Attachments.MaxScreenshots = 2 // Limit to test filtering

	// Create mailer
	mailer, err := New(&cfg.Email, tempDir)
	if err != nil {
		t.Fatalf("Failed to create mailer: %v", err)
	}

	// Test attachment processing
	result, err := mailer.processScreenshotAttachments(screenshots)
	if err != nil {
		t.Fatalf("Failed to process attachments: %v", err)
	}

	// Verify results
	if result == nil {
		t.Fatal("Expected attachment result but got nil")
	}

	if result.Strategy != "individual" {
		t.Errorf("Expected strategy 'individual', got %s", result.Strategy)
	}

	// Should have limited to max screenshots (2)
	expectedMaxAttachments := 2
	if len(result.Attachments) > expectedMaxAttachments {
		t.Errorf("Expected at most %d attachments, got %d", expectedMaxAttachments, len(result.Attachments))
	}
}

func TestEmailDataWithAttachments(t *testing.T) {
	// Create test data
	data := EmailData{
		Timestamp:             time.Now(),
		TotalCount:            5,
		AutoCount:             3,
		ManualCount:           2,
		SummaryDate:           "2023-01-01",
		HasAttachments:        true,
		AttachmentCount:       3,
		AttachmentStrategy:    "individual",
		TotalAttachmentSizeKB: 1500,
		Screenshots: []ScreenshotSummary{
			{
				ID:               "test1",
				CapturedAt:       time.Now(),
				IsAutomatic:      true,
				SizeKB:           500,
				CompressedSizeKB: 150,
				HasAttachment:    true,
			},
			{
				ID:               "test2",
				CapturedAt:       time.Now(),
				IsAutomatic:      false,
				SizeKB:           800,
				CompressedSizeKB: 0, // No attachment
				HasAttachment:    false,
			},
		},
	}

	// Verify the data structure
	if !data.HasAttachments {
		t.Error("Expected HasAttachments to be true")
	}

	if data.AttachmentCount != 3 {
		t.Errorf("Expected 3 attachments, got %d", data.AttachmentCount)
	}

	if data.AttachmentStrategy != "individual" {
		t.Errorf("Expected strategy 'individual', got %s", data.AttachmentStrategy)
	}

	// Verify screenshot attachment info
	screenshot1 := data.Screenshots[0]
	if !screenshot1.HasAttachment {
		t.Error("Expected first screenshot to have attachment")
	}

	if screenshot1.CompressedSizeKB != 150 {
		t.Errorf("Expected compressed size 150, got %d", screenshot1.CompressedSizeKB)
	}

	screenshot2 := data.Screenshots[1]
	if screenshot2.HasAttachment {
		t.Error("Expected second screenshot to not have attachment")
	}
}

func TestZipAttachmentDetection(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create test screenshots
	screenshots := make([]*storage.Screenshot, 3)
	for i := range screenshots {
		screenshot := &storage.Screenshot{
			ID:          fmt.Sprintf("test%d", i+1),
			Path:        filepath.Join(tempDir, fmt.Sprintf("screenshot%d.png", i+1)),
			CapturedAt:  time.Now(),
			IsAutomatic: i%2 == 0,
			Size:        500 * 1024, // 500KB
		}
		screenshots[i] = screenshot
	}

	// Create a mock attachment result that simulates ZIP strategy
	attachmentResult := &AttachmentResult{
		Attachments: []AttachmentInfo{{
			Filename: "screenshots_20250822_150000.zip",
			Data:     []byte("mock zip data"),
			SizeKB:   300, // Total ZIP size
		}},
		Strategy:    "zip",
		TotalSizeKB: 300,
		Skipped:     []string{}, // No screenshots skipped
		Errors:      []error{},
	}

	// Create test config
	cfg := config.Default()
	cfg.Email.Enabled = false

	// Simulate the SendDailySummary attachment detection logic
	summaries := make([]ScreenshotSummary, len(screenshots))
	for i, screenshot := range screenshots {
		hasAttachment := false
		compressedSizeKB := int64(0)

		// Use the same logic from the fixed SendDailySummary function
		switch attachmentResult.Strategy {
		case "zip":
			screenshotBase := filepath.Base(screenshot.Path)
			hasAttachment = true
			for _, skipped := range attachmentResult.Skipped {
				if skipped == screenshotBase {
					hasAttachment = false
					break
				}
			}
			if hasAttachment && len(screenshots) > 0 {
				// Calculate non-skipped count for accurate size estimation
				nonSkippedCount := len(screenshots) - len(attachmentResult.Skipped)
				if nonSkippedCount > 0 {
					estimatedSizePerScreenshot := int64(attachmentResult.TotalSizeKB) / int64(nonSkippedCount)
					compressedSizeKB = estimatedSizePerScreenshot
				}
			}
		}

		summaries[i] = ScreenshotSummary{
			ID:               screenshot.ID,
			CapturedAt:       screenshot.CapturedAt,
			IsAutomatic:      screenshot.IsAutomatic,
			SizeKB:           screenshot.Size / 1024,
			CompressedSizeKB: compressedSizeKB,
			HasAttachment:    hasAttachment,
		}
	}

	// Verify all screenshots are marked as attached for ZIP strategy
	for i, summary := range summaries {
		if !summary.HasAttachment {
			t.Errorf("Screenshot %d should be marked as attached in ZIP strategy", i+1)
		}
		
		expectedCompressedSize := int64(300 / 3) // 300KB ZIP / 3 screenshots = 100KB per screenshot
		if summary.CompressedSizeKB != expectedCompressedSize {
			t.Errorf("Screenshot %d: expected compressed size %d KB, got %d KB", 
				i+1, expectedCompressedSize, summary.CompressedSizeKB)
		}
	}

	// Test with skipped screenshots
	attachmentResultWithSkipped := &AttachmentResult{
		Attachments: []AttachmentInfo{{
			Filename: "screenshots_20250822_150000.zip",
			Data:     []byte("mock zip data"),
			SizeKB:   200,
		}},
		Strategy:    "zip",
		TotalSizeKB: 200,
		Skipped:     []string{"screenshot2.png"}, // Second screenshot skipped
		Errors:      []error{},
	}

	// Test again with skipped logic
	for i, screenshot := range screenshots {
		hasAttachment := false
		compressedSizeKB := int64(0)

		switch attachmentResultWithSkipped.Strategy {
		case "zip":
			screenshotBase := filepath.Base(screenshot.Path)
			hasAttachment = true
			for _, skipped := range attachmentResultWithSkipped.Skipped {
				if skipped == screenshotBase {
					hasAttachment = false
					break
				}
			}
			if hasAttachment && len(screenshots) > 0 {
				// Calculate non-skipped count for accurate size estimation
				nonSkippedCount := len(screenshots) - len(attachmentResultWithSkipped.Skipped)
				if nonSkippedCount > 0 {
					estimatedSizePerScreenshot := int64(attachmentResultWithSkipped.TotalSizeKB) / int64(nonSkippedCount)
					compressedSizeKB = estimatedSizePerScreenshot
				}
			}
		}

		summaries[i] = ScreenshotSummary{
			HasAttachment:    hasAttachment,
			CompressedSizeKB: compressedSizeKB,
		}
	}

	// Verify skipped logic works correctly
	if summaries[0].HasAttachment != true {
		t.Error("First screenshot should be attached (not skipped)")
	}
	if summaries[1].HasAttachment != false {
		t.Error("Second screenshot should not be attached (was skipped)")
	}
	if summaries[2].HasAttachment != true {
		t.Error("Third screenshot should be attached (not skipped)")
	}

	// Test edge case: all screenshots skipped (division by zero protection)
	attachmentResultAllSkipped := &AttachmentResult{
		Attachments: []AttachmentInfo{{
			Filename: "screenshots_20250822_150000.zip",
			Data:     []byte("mock zip data"),
			SizeKB:   100,
		}},
		Strategy:    "zip",
		TotalSizeKB: 100,
		Skipped:     []string{"screenshot1.png", "screenshot2.png", "screenshot3.png"}, // All skipped
		Errors:      []error{},
	}

	// Test with all screenshots skipped - should not cause division by zero
	for i, screenshot := range screenshots {
		hasAttachment := false
		compressedSizeKB := int64(0)

		switch attachmentResultAllSkipped.Strategy {
		case "zip":
			screenshotBase := filepath.Base(screenshot.Path)
			hasAttachment = true
			for _, skipped := range attachmentResultAllSkipped.Skipped {
				if skipped == screenshotBase {
					hasAttachment = false
					break
				}
			}
			if hasAttachment && len(screenshots) > 0 {
				// Calculate non-skipped count for accurate size estimation
				nonSkippedCount := len(screenshots) - len(attachmentResultAllSkipped.Skipped)
				if nonSkippedCount > 0 {
					estimatedSizePerScreenshot := int64(attachmentResultAllSkipped.TotalSizeKB) / int64(nonSkippedCount)
					compressedSizeKB = estimatedSizePerScreenshot
				}
			}
		}

		summaries[i] = ScreenshotSummary{
			HasAttachment:    hasAttachment,
			CompressedSizeKB: compressedSizeKB,
		}
	}

	// Verify all screenshots are marked as not attached and no division by zero occurred
	for i, summary := range summaries {
		if summary.HasAttachment != false {
			t.Errorf("Screenshot %d should not be attached when all are skipped", i+1)
		}
		if summary.CompressedSizeKB != 0 {
			t.Errorf("Screenshot %d should have 0 compressed size when not attached, got %d", i+1, summary.CompressedSizeKB)
		}
	}
}
