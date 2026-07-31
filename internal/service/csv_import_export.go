package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"mikvoc/internal/database"
)

// RouterConfig represents a router configuration record for CSV import/export
type RouterConfig struct {
	ID           int       `csv:"id"`
	Name         string    `csv:"name"`
	IPAddress    string    `csv:"ip_address"`
	Port         string    `csv:"port"`
	TemplateID   int       `csv:"template_id"`
	Status       string    `csv:"status"`
	LastSeen     time.Time `csv:"last_seen"`
}

// CSVImportResult holds the results of a bulk import operation
type CSVImportResult struct {
	SuccessCount  int               `json:"success_count"`
	FailureCount  int               `json:"failure_count"`
	UpdateCount   int               `json:"update_count"`
	CreateCount   int               `json:"create_count"`
	Errors        []CSVImportError  `json:"errors"`
	BatchSize     int               `json:"batch_size"`
	TotalProcessed int              `json:"total_processed"`
}

// CSVImportError represents an error in a specific row
type CSVImportError struct {
	LineNumber int    `json:"line_number"`
	Column     string `json:"column"`
	Value      string `json:"value"`
	Message    string `json:"message"`
}

// ExportRouterConfig exports all routers to CSV format
func ExportRouterConfig(w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{"id", "name", "ip_address", "port", "template_id", "status", "last_seen"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Get all routers
	routers, err := database.GetRouters()
	if err != nil {
		return fmt.Errorf("failed to get routers: %w", err)
	}

	now := time.Now()
	for _, router := range routers {
		record := RouterConfig{
			ID:          router.ID,
			Name:        router.Name,
			IPAddress:   router.IP,
			Port:        router.Port,
			TemplateID:  0, // Template ID stored separately
			Status:      "online",
			LastSeen:    now,
		}

		// Get template settings
		settings := database.GetRouterSettings(router.ID)
		if settings["voucher_template"] != "" {
			record.TemplateID, _ = strconv.Atoi(settings["voucher_template"])
		}

		recordRow := []string{
			strconv.Itoa(record.ID),
			record.Name,
			record.IPAddress,
			record.Port,
			strconv.Itoa(record.TemplateID),
			record.Status,
			record.LastSeen.Format(time.RFC3339),
		}

		if err := writer.Write(recordRow); err != nil {
			return fmt.Errorf("failed to write router record: %w", err)
		}
	}

	return nil
}

// ImportRouterConfig imports routers from a CSV reader
// Supports batch processing (100 routers at a time) and handles duplicates
func ImportRouterConfig(r io.Reader, batchSize int) (*CSVImportResult, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Allow variable field count

	result := &CSVImportResult{
		BatchSize:   batchSize,
		Errors:      make([]CSVImportError, 0),
	}

	// Read headers first
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Validate required columns
	requiredColumns := map[string]bool{
		"id": false,
		"name": true,
		"ip_address": true,
		"port": true,
		"template_id": false,
		"status": false,
		"last_seen": false,
	}

	columnIndex := make(map[string]int)
	for i, h := range headers {
		h = strings.TrimSpace(strings.ToLower(h))
		columnIndex[h] = i
		if _, exists := requiredColumns[h]; exists {
			requiredColumns[h] = true
		}
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("invalid CSV structure")
	}

	// Process records in batches
	batch := make([]*RouterConfig, 0, batchSize)
	lineNumber := 1

	for {
		lineNumber++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, CSVImportError{
				LineNumber: lineNumber,
				Message:    fmt.Sprintf("parse error: %v", err),
			})
			continue
		}

		// Parse and validate the record
		router, parseErrs := parseCSVRecord(record, columnIndex, lineNumber)
		if parseErrs != nil {
			result.Errors = append(result.Errors, *parseErrs...)
			continue
		}

		batch = append(batch, router)

		// Process batch when full
		if len(batch) >= batchSize {
			importResult, err := processBatch(batch)
			if err != nil {
				result.Errors = append(result.Errors, CSVImportError{
					LineNumber: 0,
					Message:    fmt.Sprintf("batch error: %v", err),
				})
			}
			result.SuccessCount += importResult.SuccessCount
			result.FailureCount += importResult.FailureCount
			result.UpdateCount += importResult.UpdateCount
			result.CreateCount += importResult.CreateCount
			result.TotalProcessed += importResult.TotalProcessed
			batch = batch[:0]
		}
	}

	// Process remaining records
	if len(batch) > 0 {
		importResult, err := processBatch(batch)
		if err != nil {
			result.Errors = append(result.Errors, CSVImportError{
				LineNumber: 0,
				Message:    fmt.Sprintf("final batch error: %v", err),
			})
		}
		result.SuccessCount += importResult.SuccessCount
		result.FailureCount += importResult.FailureCount
		result.UpdateCount += importResult.UpdateCount
		result.CreateCount += importResult.CreateCount
		result.TotalProcessed += importResult.TotalProcessed
	}

	return result, nil
}

// parseCSVRecord parses a CSV record into a RouterConfig and validates it
func parseCSVRecord(record []string, columnIndex map[string]int, lineNumber int) (*RouterConfig, *[]CSVImportError) {
	var errors []CSVImportError
	
	router := &RouterConfig{}

	// Helper function to extract field value
	getField := func(columnName string) string {
		idx, ok := columnIndex[columnName]
		if !ok || idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	// ID (optional - if not provided, new record will be created)
	if idStr := getField("id"); idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			errors = append(errors, CSVImportError{
				LineNumber: lineNumber,
				Column:     "id",
				Value:      idStr,
				Message:    fmt.Sprintf("invalid integer: %v", err),
			})
		} else {
			router.ID = id
		}
	}

	// Name (required)
	if name := getField("name"); name == "" {
		errors = append(errors, CSVImportError{
			LineNumber: lineNumber,
			Column:     "name",
			Message:    "name is required",
		})
	} else {
		router.Name = name
	}

	// IP Address (required)
	if ip := getField("ip_address"); ip == "" {
		errors = append(errors, CSVImportError{
			LineNumber: lineNumber,
			Column:     "ip_address",
			Message:    "ip_address is required",
		})
	} else {
		router.IPAddress = ip
	}

	// Port (required)
	if port := getField("port"); port == "" {
		errors = append(errors, CSVImportError{
			LineNumber: lineNumber,
			Column:     "port",
			Message:    "port is required",
		})
	} else {
		router.Port = port
	}

	// Template ID (optional)
	if templateStr := getField("template_id"); templateStr != "" {
		templateID, err := strconv.Atoi(templateStr)
		if err != nil {
			errors = append(errors, CSVImportError{
				LineNumber: lineNumber,
				Column:     "template_id",
				Value:      templateStr,
				Message:    fmt.Sprintf("invalid template_id: %v", err),
			})
		} else {
			router.TemplateID = templateID
		}
	}

	// Status (optional)
	if status := getField("status"); status != "" {
		router.Status = status
	} else {
		router.Status = "unknown"
	}

	// Last Seen (optional)
	if lastSeenStr := getField("last_seen"); lastSeenStr != "" {
		lastSeen, err := time.Parse(time.RFC3339, lastSeenStr)
		if err != nil {
			lastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeenStr)
		}
		router.LastSeen = lastSeen
	}

	if len(errors) > 0 {
		return nil, &errors
	}

	return router, nil
}

// processBatch processes a batch of router configurations
func processBatch(batch []*RouterConfig) (*CSVImportResult, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	result := &CSVImportResult{
		BatchSize: len(batch),
	}

	for _, router := range batch {
		// Check if router exists
		existingRouter, err := database.GetRouter(router.ID)
		if err != nil || existingRouter == nil {
			// Create new router
			newRouter := &database.Router{
				Name:        router.Name,
				IP:          router.IPAddress,
				Port:        router.Port,
				SortOrder:   0,
				VoucherTemplate: "classic",
			}
			
			if err := database.SaveRouter(newRouter); err != nil {
				result.Errors = append(result.Errors, CSVImportError{
					LineNumber: 0,
					Message:    fmt.Sprintf("create failed for %s: %v", router.Name, err),
				})
				result.FailureCount++
				continue
			}
			result.CreateCount++
			result.SuccessCount++
			
			// Update template if specified
			if router.TemplateID > 0 {
				database.SetRouterSetting(newRouter.ID, "voucher_template", fmt.Sprintf("%d", router.TemplateID))
			}
		} else {
			// Update existing router
			existingRouter.Name = router.Name
			existingRouter.IP = router.IPAddress
			existingRouter.Port = router.Port
			
			if err := database.SaveRouter(existingRouter); err != nil {
				result.Errors = append(result.Errors, CSVImportError{
					LineNumber: 0,
					Message:    fmt.Sprintf("update failed for %s: %v", router.Name, err),
				})
				result.FailureCount++
				continue
			}
			result.UpdateCount++
			result.SuccessCount++
			
			// Update template if specified
			if router.TemplateID > 0 {
				database.SetRouterSetting(existingRouter.ID, "voucher_template", fmt.Sprintf("%d", router.TemplateID))
			}
		}
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return result, fmt.Errorf("transaction commit failed: %w", err)
	}

	return result, nil
}

// ExportRouterConfigToFile exports routers to a CSV file
func ExportRouterConfigToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	return ExportRouterConfig(file)
}

// ImportRouterConfigFromFile imports routers from a CSV file
func ImportRouterConfigFromFile(filename string) (*CSVImportResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	return ImportRouterConfig(file, 100)
}
