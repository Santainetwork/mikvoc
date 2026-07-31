package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuditRole represents user roles for access control.
type AuditRole string

const (
	RoleOwner    AuditRole = "owner"    // Full administrative access
	RoleOperator AuditRole = "operator" // Can manage most resources but not system settings
	RoleViewer   AuditRole = "viewer"   // Read-only access
)

// AuditLevel defines the severity of audit events.
type AuditLevel string

const (
	AuditInfo   AuditLevel = "INFO"
	AuditWarn   AuditLevel = "WARN"
	AuditError  AuditLevel = "ERROR"
	AuditSecret AuditLevel = "SECRET" // For sensitive operations (credentials, keys)
)

// AuditEntry represents a structured audit log entry.
type AuditEntry struct {
	ID           int                 `json:"id"`
	AdminID      int                 `json:"admin_id,omitempty"`
	AdminName    string              `json:"admin_name,omitempty"`
	Timestamp    time.Time           `json:"timestamp"`
	Level        AuditLevel          `json:"level"`
	Action       string              `json:"action"`
	ResourceType string              `json:"resource_type"`
	Target       string              `json:"target"` // Deprecated but kept for backwards compat
	ResourceID   string              `json:"resource_id,omitempty"`
	UserID       int                 `json:"user_id,omitempty"`
	IPAddress    string              `json:"ip_address,omitempty"`
	SessionID    string              `json:"session_id,omitempty"`
	Success      bool                `json:"success"`
	Message      string              `json:"message,omitempty"`
	CreatedAt    string              `json:"created_at,omitempty"`
	Metadata     map[string]any      `json:"metadata,omitempty"`
}

// Admin represents an administrator account.
type Admin struct {
	ID               int         `json:"id"`
	Username         string      `json:"username"`
	PasswordHash     string      `json:"-"` // Never log passwords
	Email            string      `json:"email,omitempty"`
	Role             AuditRole   `json:"role"`
	IsActive         bool        `json:"is_active"`
	CreatedAt        string      `json:"created_at"`
	LastLogin        string      `json:"last_login,omitempty"`
}

// AuditLogger handles structured audit logging.
type AuditLogger struct {
	mu            sync.RWMutex
	fileWriter    io.Writer
	syslogEnabled bool
	level         AuditLevel
	retentionDays int
	sanitizeFields []string
	file          *os.File // Keep file handle for cleanup
}

// AuditConfig holds audit logger configuration.
type AuditConfig struct {
	LogDir         string
	LogFilename    string
	SyslogAddress  string
	LogLevel       AuditLevel
	RetentionDays  int
	SanitizeFields []string
}

// DefaultAuditConfig returns default audit configuration.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		LogDir:         "/var/log/mikvoc",
		LogFilename:    "audit.log",
		SyslogAddress:  "",
		LogLevel:       AuditInfo,
		RetentionDays:  90,
		SanitizeFields: []string{"password", "secret", "token", "api_key"},
	}
}

// NewAuditLogger creates a new audit logger with file and optional syslog output.
func NewAuditLogger(config AuditConfig) (*AuditLogger, error) {
	if config.LogLevel == "" {
		config.LogLevel = AuditInfo
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = 90
	}

	// Ensure log directory exists
	if err := os.MkdirAll(config.LogDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(config.LogDir, config.LogFilename)
	
	// Create or open audit log file
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	logger := &AuditLogger{
		fileWriter:     file,
		file:           file,
		syslogEnabled:  config.SyslogAddress != "",
		level:          config.LogLevel,
		retentionDays:  config.RetentionDays,
		sanitizeFields: config.SanitizeFields,
	}

	// Start retention cleanup in background
	go logger.retentionCleanup()

	return logger, nil
}

// Close closes the audit logger and releases resources.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.fileWriter != nil {
		if closer, ok := l.fileWriter.(*os.File); ok {
			return closer.Close()
		}
	}
	return nil
}

// Log writes an audit entry with the specified level.
func (l *AuditLogger) Log(level AuditLevel, action string, resourceType, resourceID string, userID int, success bool, message string, metadata map[string]interface{}) {
	l.mu.RLock()
	levelFilter := l.level
	l.mu.RUnlock()

	if level < levelFilter {
		return
	}

	entry := AuditEntry{
		Timestamp:    time.Now().UTC(),
		Level:        level,
		Action:       sanitizeAction(action),
		ResourceType: sanitizeField(resourceType),
		ResourceID:   resourceID,
		UserID:       userID,
		Message:      sanitizeMessage(message),
		Success:      success,
		Metadata:     sanitizeMetadata(metadata),
	}

	l.writeEntry(entry)
}

// LogUpload logs asset upload operations.
func (l *AuditLogger) LogUpload(userID int, routerID int, kind string, filename string, size int64, success bool, ipAddr string, errMsg string) {
	metadata := map[string]interface{}{
		"router_id": routerID,
		"asset_kind": kind,
		"filename": sanitizeFilename(filename),
		"size_bytes": size,
	}

	l.Log(AuditInfo, "upload", "asset", fmt.Sprintf("r%d_%s", routerID, kind), 
		userID, success, errMsg, metadata)
}

// LogDelete logs asset deletion operations.
func (l *AuditLogger) LogDelete(userID int, routerID int, kind string, success bool, ipAddr string, errMsg string) {
	l.Log(AuditInfo, "delete", "asset", fmt.Sprintf("r%d_%s", routerID, kind), 
		userID, success, errMsg, nil)
}

// LogModifyFocal logs focal point modification operations.
func (l *AuditLogger) LogModifyFocal(userID int, routerID int, templateID string, focalX, focalY float32, success bool, ipAddr string, errMsg string) {
	metadata := map[string]interface{}{
		"router_id": routerID,
		"template_id": templateID,
		"focal_x": focalX,
		"focal_y": focalY,
	}

	action := "modify_focal"
	resourceType := "template_focal"
	resourceID := fmt.Sprintf("t%s_r%d", templateID, routerID)

	l.Log(AuditInfo, action, resourceType, resourceID, 
		userID, success, errMsg, metadata)
}

// LogAuth logs authentication operations.
func (l *AuditLogger) LogAuth(userID int, username string, success bool, ipAddr string, method string) {
	l.Log(AuditInfo, "auth", "user_session", fmt.Sprintf("user_%d", userID),
		userID, success, "", map[string]interface{}{
			"username": sanitizeField(username),
			"method":   method,
			"ip":       ipAddr,
		})
}

// LogPermissionDenied logs permission denial events.
func (l *AuditLogger) LogPermissionDenied(userID int, action string, resource string, requiredRole string, ipAddr string) {
	l.Log(AuditError, "permission_denied", "access_control", resource,
		userID, false, "", map[string]interface{}{
			"action":      action,
			"required_role": requiredRole,
			"ip":          ipAddr,
		})
}

// writeEntry writes an audit entry to all outputs.
func (l *AuditLogger) writeEntry(entry AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Write to file
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal audit entry: %v\n", err)
		return
	}
	data = append(data, '\n')
	
	if _, err := l.fileWriter.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write audit entry: %v\n", err)
		return
	}

	// Flush file if it implements Flusher
	if flusher, ok := l.fileWriter.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
}

// sanitizeAction sanitizes action names to prevent injection attacks.
func sanitizeAction(action string) string {
	// Only allow alphanumeric, underscore, hyphen
	var result strings.Builder
	for _, r := range action {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == '-' {
			result.WriteRune(r)
		}
	}
	str := result.String()
	if str == "" {
		str = "unknown_action"
	}
	return str[:min(len(str), 100)]
}

// sanitizeMessage removes potentially sensitive data from messages.
func sanitizeMessage(msg string) string {
	result := msg
	for _, field := range sensitiveFields {
		result = strings.ReplaceAll(result, field, "***")
	}
	// Truncate long messages
	if len(result) > 500 {
		result = result[:500] + "... [truncated]"
	}
	return result
}

// sanitizeMetadata removes sensitive fields from metadata.
func sanitizeMetadata(meta map[string]interface{}) map[string]interface{} {
	if meta == nil {
		return nil
	}
	
	sanitized := make(map[string]interface{})
	for k, v := range meta {
		// Sanitize key
		k = strings.ToLower(sanitizeField(k))
		
		// Check if this is a sensitive field
		isSensitive := false
		for _, sf := range sensitiveFields {
			if strings.Contains(k, sf) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			sanitized[k] = "***"
		} else {
			switch val := v.(type) {
			case string:
				sanitized[k] = sanitizeField(val)
			default:
				sanitized[k] = val
			}
		}
	}
	
	return sanitized
}

// sanitizeField removes null bytes and limits length.
func sanitizeField(s string) string {
	// Remove control characters including null bytes
	var result strings.Builder
	for _, r := range s {
		if r > 31 || r == 9 || r == 10 || r == 13 {
			result.WriteRune(r)
		}
	}
	str := result.String()
	if len(str) > 200 {
		str = str[:200]
	}
	return str
}

// sanitizeFilename prevents path traversal in filenames.
func sanitizeFilename(name string) string {
	// Extract just the filename
	_, name = filepath.Split(name)
	return sanitizeField(name)
}

// retentionCleanup periodically removes old log files.
func (l *AuditLogger) retentionCleanup() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		l.mu.RLock()
		dir := ""
		filename := ""
		if f, ok := l.fileWriter.(*os.File); ok {
			dir = filepath.Dir(f.Name())
			filename = filepath.Base(f.Name())
		}
		l.mu.RUnlock()
		
		if dir == "" {
			continue
		}
		
		cutoff := time.Now().AddDate(0, 0, -l.retentionDays)
		
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			
			if !info.IsDir() && strings.HasPrefix(info.Name(), strings.TrimPrefix(filename, ".log")) && strings.HasSuffix(info.Name(), ".log") {
				modTime := info.ModTime()
				if modTime.Before(cutoff) {
					os.Remove(path)
				}
			}
			
			return nil
		}); err != nil {
			log.Printf("Audit log retention cleanup failed: %v", err)
		}
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// global sensitive fields list for sanitization
var sensitiveFields = []string{
	"password", "passwd", "pass", "pwd",
	"secret", "api_key", "apikey", "token",
	"authorization", "bearer", "credential",
	"private_key", "privkey", "encryption_key",
}

// Global audit logger instance
var globalAuditLogger *AuditLogger
var auditLoggerMu sync.RWMutex

// GetAuditLogger returns the global audit logger instance.
func GetAuditLogger() *AuditLogger {
	auditLoggerMu.RLock()
	defer auditLoggerMu.RUnlock()
	return globalAuditLogger
}

// SetGlobalAuditLogger sets the global audit logger instance.
func SetGlobalAuditLogger(logger *AuditLogger) {
	auditLoggerMu.Lock()
	defer auditLoggerMu.Unlock()
	globalAuditLogger = logger
}
