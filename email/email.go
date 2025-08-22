// Package email provides SMTP email notification functionality for the screenshot server.
package email

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"io"
	"log"
	"path/filepath"
	"time"

	"github.com/b4lisong/screenshot-server-go/compression"
	"github.com/b4lisong/screenshot-server-go/config"
	"github.com/b4lisong/screenshot-server-go/storage"
	"gopkg.in/gomail.v2"
)

// Mailer handles SMTP email operations.
type Mailer struct {
	config           *config.EmailConfig
	templates        *template.Template
	compressionMgr   *compression.ScreenshotCompressionManager
	attachmentHelper *compression.EmailAttachmentHelper
}

// NotificationType represents the type of email notification.
type NotificationType string

const (
	ServerStartNotification  NotificationType = "server_start"
	ServerStopNotification   NotificationType = "server_stop"
	DailySummaryNotification NotificationType = "daily_summary"
)

// EmailData contains data for email templates.
type EmailData struct {
	// Common fields
	Timestamp  time.Time
	ServerInfo ServerInfo

	// Daily summary specific
	Screenshots []ScreenshotSummary
	TotalCount  int
	AutoCount   int
	ManualCount int
	SummaryDate string

	// Attachment specific
	HasAttachments        bool
	AttachmentCount       int
	AttachmentStrategy    string
	TotalAttachmentSizeKB int
}

// ServerInfo contains server information for emails.
type ServerInfo struct {
	Port       int
	StorageDir string
	Version    string
}

// ScreenshotSummary contains summary information about a screenshot.
type ScreenshotSummary struct {
	ID               string
	CapturedAt       time.Time
	IsAutomatic      bool
	SizeKB           int64
	CompressedSizeKB int64
	HasAttachment    bool
}

// AttachmentInfo contains information about email attachments.
type AttachmentInfo struct {
	Filename string
	Data     []byte
	SizeKB   int
}

// AttachmentResult contains the result of attachment processing.
type AttachmentResult struct {
	Attachments []AttachmentInfo
	Strategy    string
	TotalSizeKB int
	Skipped     []string // Files that were skipped due to size limits
	Errors      []error  // Processing errors
}

// New creates a new email mailer with the given configuration.
func New(emailConfig *config.EmailConfig, storageDir string) (*Mailer, error) {
	// Parse email templates (always needed for testing and when email is enabled)
	var templates *template.Template
	if emailConfig.Enabled {
		tmpl, err := template.New("email").Parse(getEmailTemplates())
		if err != nil {
			return nil, fmt.Errorf("failed to parse email templates: %w", err)
		}
		templates = tmpl
	}

	// Initialize compression services if attachments are enabled
	var compressionMgr *compression.ScreenshotCompressionManager
	var attachmentHelper *compression.EmailAttachmentHelper

	if emailConfig.Attachments.Enabled {
		compressionMgr = compression.NewScreenshotCompressionManager(storageDir)
		attachmentHelper = compression.NewEmailAttachmentHelper(storageDir)
	}

	return &Mailer{
		config:           emailConfig,
		templates:        templates,
		compressionMgr:   compressionMgr,
		attachmentHelper: attachmentHelper,
	}, nil
}

// SendServerStartNotification sends a server start notification email.
func (m *Mailer) SendServerStartNotification(serverInfo ServerInfo) error {
	if !m.config.Enabled || !m.config.ServerStart {
		return nil
	}

	data := EmailData{
		Timestamp:  time.Now(),
		ServerInfo: serverInfo,
	}

	subject := fmt.Sprintf("%s Server Started", m.config.SubjectPrefix)
	return m.sendEmail(ServerStartNotification, subject, data)
}

// SendServerStopNotification sends a server stop notification email.
func (m *Mailer) SendServerStopNotification(serverInfo ServerInfo) error {
	if !m.config.Enabled || !m.config.ServerStop {
		return nil
	}

	data := EmailData{
		Timestamp:  time.Now(),
		ServerInfo: serverInfo,
	}

	subject := fmt.Sprintf("%s Server Stopped", m.config.SubjectPrefix)
	return m.sendEmail(ServerStopNotification, subject, data)
}

// SendDailySummary sends a daily summary email with screenshot information.
func (m *Mailer) SendDailySummary(serverInfo ServerInfo, screenshots []*storage.Screenshot, summaryDate time.Time) error {
	if !m.config.Enabled || !m.config.DailySummary {
		return nil
	}

	// Process attachments if enabled
	var attachmentResult *AttachmentResult
	var err error

	if m.config.Attachments.Enabled && len(screenshots) > 0 {
		attachmentResult, err = m.processScreenshotAttachments(screenshots)
		if err != nil {
			log.Printf("Failed to process attachments (continuing without attachments): %v", err)
			// Continue without attachments rather than failing the entire email
			attachmentResult = &AttachmentResult{
				Attachments: []AttachmentInfo{},
				Strategy:    "none",
				TotalSizeKB: 0,
			}
		}
	} else {
		attachmentResult = &AttachmentResult{
			Attachments: []AttachmentInfo{},
			Strategy:    "disabled",
			TotalSizeKB: 0,
		}
	}

	// Convert screenshots to summary format
	summaries := make([]ScreenshotSummary, len(screenshots))
	var autoCount, manualCount int

	for i, screenshot := range screenshots {
		// Check if this screenshot has an attachment
		hasAttachment := false
		compressedSizeKB := int64(0)

		// Handle different attachment strategies
		switch attachmentResult.Strategy {
		case "zip":
			// For ZIP strategy, all screenshots that aren't in the skipped list are attached
			screenshotBase := filepath.Base(screenshot.Path)
			hasAttachment = true // Assume attached unless found in skipped list
			for _, skipped := range attachmentResult.Skipped {
				if skipped == screenshotBase {
					hasAttachment = false
					break
				}
			}
			// For ZIP, estimate compressed size per screenshot based on total ZIP size
			if hasAttachment && len(screenshots) > 0 {
				// Calculate non-skipped count for accurate size estimation
				nonSkippedCount := len(screenshots) - len(attachmentResult.Skipped)
				if nonSkippedCount > 0 {
					estimatedSizePerScreenshot := int64(attachmentResult.TotalSizeKB) / int64(nonSkippedCount)
					compressedSizeKB = estimatedSizePerScreenshot
				}
			}

		case "individual", "adaptive":
			// For individual attachments, match by filename
			for _, att := range attachmentResult.Attachments {
				if att.Filename == filepath.Base(screenshot.Path) ||
					att.Filename == m.generateAttachmentFilename(screenshot, i) {
					hasAttachment = true
					compressedSizeKB = int64(att.SizeKB)
					break
				}
			}
		}

		summaries[i] = ScreenshotSummary{
			ID:               screenshot.ID,
			CapturedAt:       screenshot.CapturedAt,
			IsAutomatic:      screenshot.IsAutomatic,
			SizeKB:           screenshot.Size / 1024, // Convert bytes to KB
			CompressedSizeKB: compressedSizeKB,
			HasAttachment:    hasAttachment,
		}

		if screenshot.IsAutomatic {
			autoCount++
		} else {
			manualCount++
		}
	}

	data := EmailData{
		Timestamp:             time.Now(),
		ServerInfo:            serverInfo,
		Screenshots:           summaries,
		TotalCount:            len(screenshots),
		AutoCount:             autoCount,
		ManualCount:           manualCount,
		SummaryDate:           summaryDate.Format("January 2, 2006"),
		HasAttachments:        len(attachmentResult.Attachments) > 0,
		AttachmentCount:       len(attachmentResult.Attachments),
		AttachmentStrategy:    attachmentResult.Strategy,
		TotalAttachmentSizeKB: attachmentResult.TotalSizeKB,
	}

	subject := fmt.Sprintf("%s Daily Summary - %s", m.config.SubjectPrefix, summaryDate.Format("2006-01-02"))
	return m.sendEmailWithAttachments(DailySummaryNotification, subject, data, attachmentResult.Attachments)
}

// sendEmail sends an email using the configured SMTP settings.
func (m *Mailer) sendEmail(notificationType NotificationType, subject string, data EmailData) error {
	return m.sendEmailWithAttachments(notificationType, subject, data, nil)
}

// sendEmailWithAttachments sends an email with optional attachments using the configured SMTP settings.
func (m *Mailer) sendEmailWithAttachments(notificationType NotificationType, subject string, data EmailData, attachments []AttachmentInfo) error {
	if !m.config.Enabled {
		return nil
	}

	// Render email body
	body, err := m.renderTemplate(notificationType, data)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	// Create message
	message := gomail.NewMessage()
	message.SetHeader("From", m.config.FromEmail)
	message.SetHeader("To", m.config.ToEmails...)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", body)

	// Add attachments if provided
	for _, attachment := range attachments {
		message.Attach(attachment.Filename, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(attachment.Data)
			return err
		}))
	}

	// Configure SMTP dialer
	dialer := gomail.NewDialer(m.config.SMTPHost, m.config.SMTPPort, m.config.SMTPUsername, m.config.SMTPPassword)

	// Configure TLS/Security
	switch m.config.SMTPSecurity {
	case "tls":
		dialer.SSL = true
	case "starttls":
		dialer.TLSConfig = &tls.Config{ServerName: m.config.SMTPHost}
	case "none":
		dialer.SSL = false
		dialer.TLSConfig = nil
	}

	// Send email with retry logic
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := dialer.DialAndSend(message); err != nil {
			lastErr = err
			log.Printf("Email send attempt %d failed: %v", attempt, err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 5 * time.Second) // Exponential backoff
			}
			continue
		}

		// Log successful send with attachment info
		if len(attachments) > 0 {
			totalSizeKB := 0
			for _, att := range attachments {
				totalSizeKB += att.SizeKB
			}
			log.Printf("Email notification sent successfully with %d attachments (%d KB): %s",
				len(attachments), totalSizeKB, subject)
		} else {
			log.Printf("Email notification sent successfully: %s", subject)
		}
		return nil
	}

	return fmt.Errorf("failed to send email after %d attempts: %w", maxRetries, lastErr)
}

// renderTemplate renders the email template for the given notification type.
func (m *Mailer) renderTemplate(notificationType NotificationType, data EmailData) (string, error) {
	var buf bytes.Buffer
	templateName := string(notificationType)

	if err := m.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// IsEnabled returns whether email notifications are enabled.
func (m *Mailer) IsEnabled() bool {
	return m.config.Enabled
}

// processScreenshotAttachments processes screenshots for email attachments based on the configured strategy.
func (m *Mailer) processScreenshotAttachments(screenshots []*storage.Screenshot) (*AttachmentResult, error) {
	if m.attachmentHelper == nil {
		return nil, fmt.Errorf("attachment helper not initialized")
	}

	// Limit the number of screenshots processed
	maxScreenshots := m.config.Attachments.MaxScreenshots
	if len(screenshots) > maxScreenshots {
		log.Printf("Limiting attachments to %d out of %d screenshots", maxScreenshots, len(screenshots))
		screenshots = screenshots[:maxScreenshots]
	}

	// Extract file paths
	screenshotPaths := make([]string, len(screenshots))
	for i, screenshot := range screenshots {
		screenshotPaths[i] = screenshot.Path
	}

	// Process based on strategy
	switch m.config.Attachments.Strategy {
	case "individual":
		return m.processIndividualAttachments(screenshotPaths)
	case "zip":
		return m.processZipAttachment(screenshotPaths)
	case "adaptive":
		return m.processAdaptiveAttachments(screenshotPaths)
	default:
		return nil, fmt.Errorf("unknown attachment strategy: %s", m.config.Attachments.Strategy)
	}
}

// processIndividualAttachments creates individual compressed attachments for each screenshot.
func (m *Mailer) processIndividualAttachments(screenshotPaths []string) (*AttachmentResult, error) {
	maxTotalSizeKB := int(m.config.Attachments.MaxTotalSizeMB * 1024)
	maxAttachmentSizeKB := int(m.config.Attachments.MaxAttachmentSizeMB * 1024)

	compressedData, _, err := m.attachmentHelper.PrepareScreenshotsForEmail(screenshotPaths, maxTotalSizeKB)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare screenshots for email: %w", err)
	}

	attachments := make([]AttachmentInfo, 0, len(compressedData))
	totalSizeKB := 0
	var skipped []string

	for i, data := range compressedData {
		if i >= len(screenshotPaths) {
			break
		}

		sizeKB := len(data) / 1024

		// Check individual size limit
		if sizeKB > maxAttachmentSizeKB {
			skipped = append(skipped, filepath.Base(screenshotPaths[i]))
			continue
		}

		// Check total size limit
		if totalSizeKB+sizeKB > maxTotalSizeKB {
			skipped = append(skipped, filepath.Base(screenshotPaths[i]))
			continue
		}

		filename := m.generateAttachmentFilename(nil, i) + ".jpg"
		if i < len(screenshotPaths) {
			base := filepath.Base(screenshotPaths[i])
			ext := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			filename = name + "_compressed.jpg"
		}

		attachments = append(attachments, AttachmentInfo{
			Filename: filename,
			Data:     data,
			SizeKB:   sizeKB,
		})

		totalSizeKB += sizeKB
	}

	// Log skipped files if any
	if len(skipped) > 0 {
		log.Printf("Skipped %d screenshots due to size limits: %v", len(skipped), skipped)
	}

	return &AttachmentResult{
		Attachments: attachments,
		Strategy:    "individual",
		TotalSizeKB: totalSizeKB,
		Skipped:     skipped,
	}, nil
}

// processZipAttachment creates a single ZIP archive containing all compressed screenshots.
func (m *Mailer) processZipAttachment(screenshotPaths []string) (*AttachmentResult, error) {
	maxTotalSizeKB := int(m.config.Attachments.MaxTotalSizeMB * 1024)

	// Prepare compressed screenshots
	compressedData, _, err := m.attachmentHelper.PrepareScreenshotsForEmail(screenshotPaths, maxTotalSizeKB)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare screenshots for email: %w", err)
	}

	// Create ZIP archive in memory
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	totalSizeKB := 0
	var skipped []string

	for i, data := range compressedData {
		if i >= len(screenshotPaths) {
			break
		}

		// Generate filename for ZIP entry
		base := filepath.Base(screenshotPaths[i])
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		filename := name + "_compressed.jpg"

		// Add file to ZIP
		fileWriter, err := zipWriter.Create(filename)
		if err != nil {
			log.Printf("Failed to create ZIP entry for %s: %v", filename, err)
			skipped = append(skipped, base)
			continue
		}

		if _, err := fileWriter.Write(data); err != nil {
			log.Printf("Failed to write ZIP entry for %s: %v", filename, err)
			skipped = append(skipped, base)
			continue
		}

		totalSizeKB += len(data) / 1024
	}

	// Close ZIP writer
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	zipData := zipBuffer.Bytes()
	zipSizeKB := len(zipData) / 1024

	// Check if ZIP exceeds size limit
	if zipSizeKB > maxTotalSizeKB {
		return nil, fmt.Errorf("ZIP archive size (%d KB) exceeds limit (%d KB)", zipSizeKB, maxTotalSizeKB)
	}

	timestamp := time.Now().Format("20060102_150405")
	zipFilename := fmt.Sprintf("screenshots_%s.zip", timestamp)

	return &AttachmentResult{
		Attachments: []AttachmentInfo{{
			Filename: zipFilename,
			Data:     zipData,
			SizeKB:   zipSizeKB,
		}},
		Strategy:    "zip",
		TotalSizeKB: zipSizeKB,
		Skipped:     skipped,
	}, nil
}

// processAdaptiveAttachments uses an adaptive strategy based on the number and size of screenshots.
func (m *Mailer) processAdaptiveAttachments(screenshotPaths []string) (*AttachmentResult, error) {
	// Decision logic for adaptive strategy
	const (
		maxIndividualFiles = 5
		zipThreshold       = 3
	)

	numScreenshots := len(screenshotPaths)

	// If few screenshots, use individual attachments
	if numScreenshots <= maxIndividualFiles {
		log.Printf("Using individual strategy for %d screenshots", numScreenshots)
		return m.processIndividualAttachments(screenshotPaths)
	}

	// If many screenshots, use ZIP
	if numScreenshots >= zipThreshold {
		log.Printf("Using ZIP strategy for %d screenshots", numScreenshots)
		result, err := m.processZipAttachment(screenshotPaths)
		if err != nil {
			// Fallback to individual if ZIP fails
			log.Printf("ZIP strategy failed, falling back to individual: %v", err)
			return m.processIndividualAttachments(screenshotPaths)
		}
		return result, nil
	}

	// Default to individual
	return m.processIndividualAttachments(screenshotPaths)
}

// generateAttachmentFilename generates a filename for an attachment.
func (m *Mailer) generateAttachmentFilename(screenshot *storage.Screenshot, index int) string {
	if screenshot != nil {
		base := filepath.Base(screenshot.Path)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		return name + "_compressed"
	}

	// Fallback to index-based naming
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("screenshot_%s_%d", timestamp, index)
}

// getEmailTemplates returns the embedded email templates.
func getEmailTemplates() string {
	return `
{{define "server_start"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Server Started</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; color: #333; }
        .header { background-color: #4CAF50; color: white; padding: 20px; border-radius: 5px; }
        .content { margin: 20px 0; }
        .info-table { border-collapse: collapse; width: 100%; }
        .info-table th, .info-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        .info-table th { background-color: #f2f2f2; }
        .footer { color: #666; font-size: 12px; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>📸 Screenshot Server Started</h2>
    </div>
    
    <div class="content">
        <p>Your screenshot server has started successfully and is ready to accept requests.</p>
        
        <table class="info-table">
            <tr><th>Started At</th><td>{{.Timestamp.Format "2006-01-02 15:04:05 MST"}}</td></tr>
            <tr><th>Server Port</th><td>{{.ServerInfo.Port}}</td></tr>
            <tr><th>Storage Directory</th><td>{{.ServerInfo.StorageDir}}</td></tr>
            <tr><th>Server URL</th><td><a href="http://localhost:{{.ServerInfo.Port}}">http://localhost:{{.ServerInfo.Port}}</a></td></tr>
            <tr><th>Activity Page</th><td><a href="http://localhost:{{.ServerInfo.Port}}/activity">http://localhost:{{.ServerInfo.Port}}/activity</a></td></tr>
        </table>
    </div>
    
    <div class="footer">
        <p>This is an automated notification from your Screenshot Server.</p>
    </div>
</body>
</html>
{{end}}

{{define "server_stop"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Server Stopped</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; color: #333; }
        .header { background-color: #f44336; color: white; padding: 20px; border-radius: 5px; }
        .content { margin: 20px 0; }
        .info-table { border-collapse: collapse; width: 100%; }
        .info-table th, .info-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        .info-table th { background-color: #f2f2f2; }
        .footer { color: #666; font-size: 12px; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>🛑 Screenshot Server Stopped</h2>
    </div>
    
    <div class="content">
        <p>Your screenshot server has been stopped.</p>
        
        <table class="info-table">
            <tr><th>Stopped At</th><td>{{.Timestamp.Format "2006-01-02 15:04:05 MST"}}</td></tr>
            <tr><th>Server Port</th><td>{{.ServerInfo.Port}}</td></tr>
            <tr><th>Storage Directory</th><td>{{.ServerInfo.StorageDir}}</td></tr>
        </table>
    </div>
    
    <div class="footer">
        <p>This is an automated notification from your Screenshot Server.</p>
    </div>
</body>
</html>
{{end}}

{{define "daily_summary"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Daily Summary</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; color: #333; }
        .header { background-color: #2196F3; color: white; padding: 20px; border-radius: 5px; }
        .content { margin: 20px 0; }
        .summary-stats { display: flex; gap: 20px; margin: 20px 0; }
        .stat-box { border: 1px solid #ddd; padding: 15px; border-radius: 5px; text-align: center; flex: 1; }
        .stat-number { font-size: 24px; font-weight: bold; color: #2196F3; }
        .stat-label { color: #666; margin-top: 5px; }
        .screenshot-table { border-collapse: collapse; width: 100%; margin: 20px 0; }
        .screenshot-table th, .screenshot-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        .screenshot-table th { background-color: #f2f2f2; }
        .auto-badge { background-color: #4CAF50; color: white; padding: 2px 6px; border-radius: 3px; font-size: 11px; }
        .manual-badge { background-color: #ff9800; color: white; padding: 2px 6px; border-radius: 3px; font-size: 11px; }
        .footer { color: #666; font-size: 12px; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>📊 Daily Screenshot Summary</h2>
        <p>{{.SummaryDate}}</p>
    </div>
    
    <div class="summary-stats">
        <div class="stat-box">
            <div class="stat-number">{{.TotalCount}}</div>
            <div class="stat-label">Total Screenshots</div>
        </div>
        <div class="stat-box">
            <div class="stat-number">{{.AutoCount}}</div>
            <div class="stat-label">Automatic</div>
        </div>
        <div class="stat-box">
            <div class="stat-number">{{.ManualCount}}</div>
            <div class="stat-label">Manual</div>
        </div>
    </div>
    
    <div class="content">
        {{if .HasAttachments}}
        <div class="attachment-info" style="background-color: #e8f5e8; padding: 15px; border-radius: 5px; margin: 20px 0;">
            <h3 style="margin-top: 0; color: #2e7d2e;">📎 Attachments Included</h3>
            <p><strong>{{.AttachmentCount}}</strong> attachment(s) attached using <strong>{{.AttachmentStrategy}}</strong> strategy</p>
            <p>Total attachment size: <strong>{{.TotalAttachmentSizeKB}} KB</strong></p>
        </div>
        {{end}}
        
        {{if .Screenshots}}
        <h3>Screenshot Details</h3>
        <table class="screenshot-table">
            <tr>
                <th>Time</th>
                <th>Type</th>
                <th>Original Size</th>
                {{if .HasAttachments}}<th>Compressed Size</th><th>Attached</th>{{end}}
                <th>ID</th>
            </tr>
            {{range .Screenshots}}
            <tr>
                <td>{{.CapturedAt.Format "15:04:05"}}</td>
                <td>
                    {{if .IsAutomatic}}
                        <span class="auto-badge">AUTO</span>
                    {{else}}
                        <span class="manual-badge">MANUAL</span>
                    {{end}}
                </td>
                <td>{{.SizeKB}} KB</td>
                {{if $.HasAttachments}}
                    <td>{{if .HasAttachment}}{{.CompressedSizeKB}} KB{{else}}-{{end}}</td>
                    <td>{{if .HasAttachment}}<span style="color: #4CAF50;">✓</span>{{else}}<span style="color: #f44336;">✗</span>{{end}}</td>
                {{end}}
                <td><code>{{.ID}}</code></td>
            </tr>
            {{end}}
        </table>
        {{else}}
        <p>No screenshots were captured on {{.SummaryDate}}.</p>
        {{end}}
    </div>
    
    <div class="footer">
        <p>Generated at {{.Timestamp.Format "2006-01-02 15:04:05 MST"}}</p>
        <p>This is an automated notification from your Screenshot Server.</p>
    </div>
</body>
</html>
{{end}}
`
}
