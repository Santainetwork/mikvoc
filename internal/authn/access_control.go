package authn

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"mikvoc/internal/core"
)

// Permission constants define granular access controls.
type Permission string

const (
	ViewAssets         Permission = "view_assets"         // Read-only access to view assets
	UploadAssets       Permission = "upload_assets"       // Upload new assets/templates
	DeleteAssets       Permission = "delete_assets"       // Delete assets
	ManageFocal        Permission = "manage_focal"        // Modify focal points on templates
	ManageUsers        Permission = "manage_users"        // Create/modify/delete users
	ManageSettings     Permission = "manage_settings"     // Change system settings
	ManageTemplates    Permission = "manage_templates"    // Full template management
	ViewStats          Permission = "view_stats"          // View analytics and statistics
	ManageHotspot      Permission = "manage_hotspot"      // Hotspot configuration
	BackupRestore      Permission = "backup_restore"      // Backup and restore operations
)

// Role represents a user role with associated permissions.
type Role string

const (
	RoleOwner    Role = "owner"    // Full administrative access
	RoleOperator Role = "operator" // Can manage most resources but not system settings
	RoleEditor   Role = "editor"   // Can edit owned resources only
	RoleViewer   Role = "viewer"   // Read-only access
)

// UserPermissions holds all permissions for a user.
type UserPermissions struct {
	UserID    int
	Username  string
	Role      Role
	IsGlobal  bool // If true, applies globally across all routers
	Routers   []int // Specific router IDs this user manages
	TemplateIDs []string // Template IDs owned by user
	Mutex     sync.RWMutex
}

// PermissionChecker validates access based on permissions.
type PermissionChecker struct {
	permissions map[int]*UserPermissions
	mu          sync.RWMutex
}

// NewPermissionChecker creates a new permission checker instance.
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{
		permissions: make(map[int]*UserPermissions),
	}
}

// AddPermission registers a user's permissions.
func (pc *PermissionChecker) AddPermission(up *UserPermissions) error {
	if up == nil || up.UserID <= 0 {
		return fmt.Errorf("invalid user permissions")
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.permissions[up.UserID] = up
	return nil
}

// RemovePermission removes a user's permissions.
func (pc *PermissionChecker) RemovePermission(userID int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	delete(pc.permissions, userID)
}

// GetUserPermissions returns the permissions for a specific user.
func (pc *PermissionChecker) GetUserPermissions(userID int) *UserPermissions {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if up, exists := pc.permissions[userID]; exists {
		return up
	}
	return nil
}

// CheckPermission determines if a user has a specific permission.
func (pc *PermissionChecker) CheckPermission(userID int, perm Permission) bool {
	pc.mu.RLock()
	up, exists := pc.permissions[userID]
	pc.mu.RUnlock()

	if !exists || up == nil {
		return false
	}

	switch perm {
	case ViewAssets:
		// All roles except unauthenticated can view
		return up.Role != ""

	case UploadAssets, ManageTemplates:
		// Owner and operator can upload/manage templates
		if up.Role == RoleOwner || up.Role == RoleOperator {
			return true
		}
		// Editors can upload their own templates
		return up.Role == RoleEditor && len(up.TemplateIDs) > 0

	case DeleteAssets, ManageFocal:
		// Owner can delete anything
		if up.Role == RoleOwner {
			return true
		}
		// Operator can delete within assigned routers
		if up.Role == RoleOperator {
			return pc.canAccessRouter(up, 0)
		}
		// Editor can only modify their own templates
		return up.Role == RoleEditor

	case ManageUsers, ManageSettings:
		// Only owner can manage users and settings
		return up.Role == RoleOwner

	case ViewStats:
		// Owners and operators can view stats
		return up.Role == RoleOwner || up.Role == RoleOperator

	case ManageHotspot, BackupRestore:
		// Owners and operators can manage hotspot and backups
		return up.Role == RoleOwner || up.Role == RoleOperator

	default:
		return false
	}
}

// CheckResourceAccess verifies if user can access a specific resource.
func (pc *PermissionChecker) CheckResourceAccess(userID int, resourceType string, resourceID string) bool {
	pc.mu.RLock()
	up, exists := pc.permissions[userID]
	pc.mu.RUnlock()

	if !exists || up == nil {
		return false
	}

	// Global admin bypass
	if up.Role == RoleOwner {
		return true
	}

	// Check router-specific access
	if isRouterResource(resourceType) {
		return pc.canAccessRouter(up, parseRouterID(resourceID))
	}

	// Check template ownership
	if isTemplateResource(resourceType) {
		return pc.canAccessTemplate(up, resourceID)
	}

	// Default deny
	return false
}

// canAccessRouter checks if user can access a specific router.
func (pc *PermissionChecker) canAccessRouter(up *UserPermissions, routerID int) bool {
	if up.IsGlobal {
		return true
	}

	if len(up.Routers) == 0 {
		return false
	}

	for _, id := range up.Routers {
		if id == routerID {
			return true
		}
	}

	return false
}

// canAccessTemplate checks if user owns or can edit a template.
func (pc *PermissionChecker) canAccessTemplate(up *UserPermissions, templateID string) bool {
	if up.IsGlobal {
		return true
	}

	// Quick check if template ID is in user's owned list
	for _, tid := range up.TemplateIDs {
		if tid == templateID {
			return true
		}
	}

	return false
}

// GetRequiredRoles returns the required role for each permission.
func (pc *PermissionChecker) GetRequiredRoles() map[Permission]Role {
	return map[Permission]Role{
		ViewAssets:     RoleViewer,
		UploadAssets:   RoleEditor,
		DeleteAssets:   RoleOperator,
		ManageFocal:    RoleEditor,
		ManageUsers:    RoleOwner,
		ManageSettings: RoleOwner,
		ManageTemplates: RoleEditor,
		ViewStats:      RoleOperator,
		ManageHotspot:  RoleOperator,
		BackupRestore:  RoleOperator,
	}
}

// HasAnyPermission checks if user has any of the specified permissions.
func (pc *PermissionChecker) HasAnyPermission(userID int, perms ...Permission) bool {
	for _, perm := range perms {
		if pc.CheckPermission(userID, perm) {
			return true
		}
	}
	return false
}

// Middleware wraps HTTP handlers with permission checking.
func (pc *PermissionChecker) PermissionMiddleware(permission Permission) func(http.Handler) http.Handler {
	requiredRole := pc.GetRequiredRoles()[permission]

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := getAuthUserID(r)
			
			if userID <= 0 {
				handleUnauthorized(w, r)
				return
			}

			if pc.CheckPermission(userID, permission) {
				next.ServeHTTP(w, r)
				return
			}

			handleForbidden(w, r, permission, Role(requiredRole))
		})
	}
}

// ResourceAccessMiddleware checks access to specific resources.
func (pc *PermissionChecker) ResourceAccessMiddleware(resourceType string, extractID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := getAuthUserID(r)
			
			if userID <= 0 {
				handleUnauthorized(w, r)
				return
			}

			resourceID := extractID(r)
			if resourceID == "" {
				http.Error(w, "Invalid resource", http.StatusBadRequest)
				return
			}

			if pc.CheckResourceAccess(userID, resourceType, resourceID) {
				next.ServeHTTP(w, r)
				return
			}

			handleForbidden(w, r, "", "")
		})
	}
}

// AuthContext stores authentication details in request context.
type AuthContextKey string

const AuthContextKeyVal AuthContextKey = "auth_context"

// AuthContext contains user authentication information.
type AuthContext struct {
	UserID   int
	Username string
	Role     Role
}

// WithAuthContext adds auth context to request.
func WithAuthContext(ctx context.Context, auth AuthContext) context.Context {
	return context.WithValue(ctx, AuthContextKeyVal, auth)
}

// AuthFromContext extracts auth info from request context.
func AuthFromContext(r *http.Request) (*AuthContext, bool) {
	auth, ok := r.Context().Value(AuthContextKeyVal).(AuthContext)
	return &auth, ok
}

// Helper functions

// getAuthUserID extracts user ID from request (session or JWT).
func getAuthUserID(r *http.Request) int {
	// Try context first
	if auth, ok := AuthFromContext(r); ok {
		return auth.UserID
	}

	// Fall back to session (will need middleware integration)
	// This would be implemented in middleware package
	return 0
}

// isRouterResource checks if resource type is router-related.
func isRouterResource(resourceType string) bool {
	return resourceType == "router" || resourceType == "asset" || resourceType == "focal"
}

// isTemplateResource checks if resource type is template-related.
func isTemplateResource(resourceType string) bool {
	return resourceType == "template" || resourceType == "login_template"
}

// parseRouterID extracts router ID from resource ID string.
func parseRouterID(resourceID string) int {
	var id int
	n, _ := fmt.Sscanf(resourceID, "r%d", &id)
	if n != 1 {
		return 0
	}
	return id
}

// handleUnauthorized writes an unauthorized response.
func handleUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","message":"Authentication required"}`))
}

// handleForbidden writes a forbidden response with audit logging.
func handleForbidden(w http.ResponseWriter, r *http.Request, permission Permission, requiredRole Role) {
	logPermissionDenied(getRemoteAddr(r), permission, requiredRole)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(fmt.Sprintf(`{"error":"forbidden","message":"You do not have permission to %s","required_role":"%s"}`, 
		permission, requiredRole)))
}

// logPermissionDenied logs permission denial events.
func logPermissionDenied(ipAddr string, permission Permission, requiredRole Role) {
	auditLogger := core.GetAuditLogger()
	if auditLogger != nil {
		auditLogger.LogPermissionDenied(0, string(permission), "unknown", string(requiredRole), ipAddr)
	}
}

// getRemoteAddr extracts IP from request.
func getRemoteAddr(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}
