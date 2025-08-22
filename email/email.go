// Package email provides SMTP email notification functionality for the screenshot server.
package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/b4lisong/screenshot-server-go/config"
	"github.com/b4lisong/screenshot-server-go/storage"
	"gopkg.in/gomail.v2"
)

// Mailer handles SMTP email operations.
type Mailer struct {
	config    *config.EmailConfig
	templates *template.Template
}

// NotificationType represents the type of email notification.
type NotificationType string

const (
	ServerStartNotification NotificationType = "server_start"
	ServerStopNotification  NotificationType = "server_stop"
	DailySummaryNotification NotificationType = "daily_summary"
)

// EmailData contains data for email templates.
type EmailData struct {
	// Common fields
	Timestamp   time.Time
	ServerInfo  ServerInfo
	
	// Daily summary specific
	Screenshots []ScreenshotSummary
	TotalCount  int
	AutoCount   int
	ManualCount int
	SummaryDate string
}

// ServerInfo contains server information for emails.
type ServerInfo struct {
	Port       int
	StorageDir string
	Version    string
}

// ScreenshotSummary contains summary information about a screenshot.
type ScreenshotSummary struct {
	ID          string
	CapturedAt  time.Time
	IsAutomatic bool
	SizeKB      int64
}

// New creates a new email mailer with the given configuration.
func New(emailConfig *config.EmailConfig) (*Mailer, error) {
	if !emailConfig.Enabled {
		return &Mailer{config: emailConfig}, nil
	}
	
	// Parse email templates
	templates, err := template.New("email").Parse(getEmailTemplates())
	if err != nil {
		return nil, fmt.Errorf("failed to parse email templates: %w", err)
	}
	
	return &Mailer{
		config:    emailConfig,
		templates: templates,
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
	
	// Convert screenshots to summary format
	summaries := make([]ScreenshotSummary, len(screenshots))
	var autoCount, manualCount int
	
	for i, screenshot := range screenshots {
		summaries[i] = ScreenshotSummary{
			ID:          screenshot.ID,
			CapturedAt:  screenshot.CapturedAt,
			IsAutomatic: screenshot.IsAutomatic,
			SizeKB:      screenshot.Size / 1024, // Convert bytes to KB
		}
		
		if screenshot.IsAutomatic {
			autoCount++
		} else {
			manualCount++
		}
	}
	
	data := EmailData{
		Timestamp:   time.Now(),
		ServerInfo:  serverInfo,
		Screenshots: summaries,
		TotalCount:  len(screenshots),
		AutoCount:   autoCount,
		ManualCount: manualCount,
		SummaryDate: summaryDate.Format("January 2, 2006"),
	}
	
	subject := fmt.Sprintf("%s Daily Summary - %s", m.config.SubjectPrefix, summaryDate.Format("2006-01-02"))
	return m.sendEmail(DailySummaryNotification, subject, data)
}

// sendEmail sends an email using the configured SMTP settings.
func (m *Mailer) sendEmail(notificationType NotificationType, subject string, data EmailData) error {
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
		
		log.Printf("Email notification sent successfully: %s", subject)
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
        {{if .Screenshots}}
        <h3>Screenshot Details</h3>
        <table class="screenshot-table">
            <tr>
                <th>Time</th>
                <th>Type</th>
                <th>Size</th>
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