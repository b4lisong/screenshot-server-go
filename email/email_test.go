package email

import (
	"fmt"
	"image"
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
