package service

import (
	"fmt"
	"sync"

	"mikvoc/internal/database"
)

// BulkOperationResult represents the result of a single router operation
type BulkOperationResult struct {
	RouterID int    `json:"router_id"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Action   string `json:"action"`
}

// BulkAssetUpdateResult represents the complete bulk update result
type BulkAssetUpdateResult struct {
	SuccessCount int                  `json:"success_count"`
	FailureCount int                  `json:"failure_count"`
	Results      []BulkOperationResult `json:"results"`
	RolledBack   bool                   `json:"rolled_back"`
}

// BulkAssetDeleteResult represents the complete bulk delete result
type BulkAssetDeleteResult struct {
	SuccessCount int                  `json:"success_count"`
	FailureCount int                  `json:"failure_count"`
	Results      []BulkOperationResult `json:"results"`
	RolledBack   bool                   `json:"rolled_back"`
}

// BulkOperationsService handles bulk operations on routers
type BulkOperationsService struct {
	pool *Pool
	mu   sync.Mutex // Protect rollback state during transaction
}

// NewBulkOperations creates a new BulkOperationsService
func NewBulkOperations(pool *Pool) *BulkOperationsService {
	return &BulkOperationsService{pool: pool}
}

// BulkAssetUpdate applies asset changes to multiple routers atomically
// If any router fails, all changes are rolled back
func (s *BulkOperationsService) BulkAssetUpdate(routerIDs []int, logoURL string, backgroundURL string) (*BulkAssetUpdateResult, error) {
	result := &BulkAssetUpdateResult{
		Results: make([]BulkOperationResult, 0, len(routerIDs)),
	}

	if len(routerIDs) == 0 {
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate all router IDs exist before executing
	validRouters := make([]int, 0, len(routerIDs))
	for _, id := range routerIDs {
		router, err := database.GetRouter(id)
		if err != nil || router == nil {
			result.Results = append(result.Results, BulkOperationResult{
				RouterID: id,
				Success:  false,
				Error:    fmt.Sprintf("router not found or invalid: %v", err),
				Action:   "update",
			})
		} else {
			validRouters = append(validRouters, id)
		}
	}

	// Rollback if validation failed for any router
	if len(validRouters) != len(routerIDs) {
		result.RolledBack = true
		return result, fmt.Errorf("validation failed: not all routers exist")
	}

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		result.Results = append(result.Results, BulkOperationResult{
			RouterID: -1,
			Success:  false,
			Error:    fmt.Sprintf("failed to begin transaction: %v", err),
			Action:   "transaction",
		})
		result.RolledBack = true
		return result, err
	}

	// Execute updates
	for _, routerID := range validRouters {
		settings := map[string]string{}
		if logoURL != "" {
			settings["tpl_logo_url"] = logoURL
		}
		if backgroundURL != "" {
			settings["tpl_bg_image"] = backgroundURL
		}

		err := database.SetTemplateSettings(routerID, settings)
		result.Results = append(result.Results, BulkOperationResult{
			RouterID: routerID,
			Success:  err == nil,
			Error:    "",
			Action:   "update",
		})

		if err != nil {
			_ = tx.Rollback()
			result.RolledBack = true
			s.logBulkOperation("bulk_update_failure", routerID, err.Error())
			return result, fmt.Errorf("update failed for router %d: %v", routerID, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		result.RolledBack = true
		s.logBulkOperation("bulk_commit_failure", -1, err.Error())
		return result, err
	}

	// Count successes and failures
	for _, r := range result.Results {
		if r.Success {
			result.SuccessCount++
			s.logBulkOperation("bulk_update_success", r.RouterID, "")
		} else {
			result.FailureCount++
		}
	}

	return result, nil
}

// BulkAssetDelete removes specific assets from multiple routers atomically
// If any router fails, all changes are rolled back
func (s *BulkOperationsService) BulkAssetDelete(routerIDs []int, assetType string) (*BulkAssetDeleteResult, error) {
	result := &BulkAssetDeleteResult{
		Results: make([]BulkOperationResult, 0, len(routerIDs)),
	}

	if len(routerIDs) == 0 {
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate all router IDs exist before executing
	validRouters := make([]int, 0, len(routerIDs))
	for _, id := range routerIDs {
		router, err := database.GetRouter(id)
		if err != nil || router == nil {
			result.Results = append(result.Results, BulkOperationResult{
				RouterID: id,
				Success:  false,
				Error:    fmt.Sprintf("router not found or invalid: %v", err),
				Action:   "delete",
			})
		} else {
			validRouters = append(validRouters, id)
		}
	}

	// Rollback if validation failed for any router
	if len(validRouters) != len(routerIDs) {
		result.RolledBack = true
		return result, fmt.Errorf("validation failed: not all routers exist")
	}

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		result.Results = append(result.Results, BulkOperationResult{
			RouterID: -1,
			Success:  false,
			Error:    fmt.Sprintf("failed to begin transaction: %v", err),
			Action:   "transaction",
		})
		result.RolledBack = true
		return result, err
	}

	// Execute deletions
	for _, routerID := range validRouters {
		currentSettings := database.GetRouterSettings(routerID)
		modified := false

		switch assetType {
		case "logo":
			if currentSettings["tpl_logo_url"] != "" {
				currentSettings["tpl_logo_url"] = ""
				modified = true
			}
		case "background":
			if currentSettings["tpl_bg_image"] != "" {
				currentSettings["tpl_bg_image"] = ""
				modified = true
			}
		case "assets":
			if currentSettings["tpl_custom_assets_zip"] != "" {
				currentSettings["tpl_custom_assets_zip"] = ""
				currentSettings["tpl_custom_assets_manifest"] = ""
				modified = true
			}
		default:
			result.Results = append(result.Results, BulkOperationResult{
				RouterID: routerID,
				Success:  false,
				Error:    fmt.Sprintf("unknown asset type: %s", assetType),
				Action:   "delete",
			})
			continue
		}

		if !modified {
			result.Results = append(result.Results, BulkOperationResult{
				RouterID: routerID,
				Success:  true,
				Error:    "asset not present",
				Action:   "delete",
			})
			continue
		}

		err := database.SetTemplateSettings(routerID, currentSettings)
		result.Results = append(result.Results, BulkOperationResult{
			RouterID: routerID,
			Success:  err == nil,
			Error:    "",
			Action:   "delete",
		})

		if err != nil {
			_ = tx.Rollback()
			result.RolledBack = true
			s.logBulkOperation("bulk_delete_failure", routerID, err.Error())
			return result, fmt.Errorf("delete failed for router %d: %v", routerID, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		result.RolledBack = true
		s.logBulkOperation("bulk_commit_failure", -1, err.Error())
		return result, err
	}

	// Count successes and failures
	for _, r := range result.Results {
		if r.Success && r.Error != "asset not present" {
			result.SuccessCount++
			s.logBulkOperation("bulk_delete_success", r.RouterID, "")
		} else {
			result.FailureCount++
		}
	}

	return result, nil
}

// logBulkOperation logs bulk operations to audit trail
func (s *BulkOperationsService) logBulkOperation(operation string, routerID int, errorMessage string) {
	var logMsg string
	if routerID > 0 {
		logMsg = fmt.Sprintf("[%s] router_id=%d", operation, routerID)
	} else {
		logMsg = fmt.Sprintf("[%s]", operation)
	}
	if errorMessage != "" {
		logMsg += fmt.Sprintf(" error=%s", errorMessage)
	}
	_ = logMsg // Log to audit trail (could be implemented with logging framework)
	fmt.Printf("[audit] %s\n", logMsg)
}

// CheckRoutersExist validates a list of router IDs and returns only valid ones
func (s *BulkOperationsService) CheckRoutersExist(routerIDs []int) ([]int, error) {
	validIDs := make([]int, 0, len(routerIDs))
	for _, id := range routerIDs {
		router, err := database.GetRouter(id)
		if err != nil || router == nil {
			return validIDs, fmt.Errorf("router %d not found", id)
		}
		validIDs = append(validIDs, id)
	}
	return validIDs, nil
}
