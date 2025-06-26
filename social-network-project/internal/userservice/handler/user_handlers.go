package handler

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	// Adjust the import path based on your go.mod module name
	"github.com/yourusername/social-network/pkg/models"
)

// UserHandler struct holds dependencies for user service handlers.
type UserHandler struct {
	DB           *sql.DB
	JwtSecretKey []byte
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(db *sql.DB, jwtKey []byte) *UserHandler {
	return &UserHandler{
		DB:           db,
		JwtSecretKey: jwtKey,
	}
}

// AuthMiddleware verifies the JWT token.
// This is a method on UserHandler now, or could be a standalone function if preferred.
// Making it a method allows it to potentially use dependencies from UserHandler if ever needed, though not currently.
func (h *UserHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return h.JwtSecretKey, nil // Use JwtSecretKey from handler
		})

		if err != nil {
			log.Printf("Token validation error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		userIDStr, okUserID := claims["user_id"].(string)
		username, okUsername := claims["username"].(string)

		if !okUserID || !okUsername {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}
		
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token claims"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Set("username", username)
		c.Next()
	}
}

// GetUserProfile handles fetching a user's profile by ID.
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	userIDParam := c.Param("userId")
	targetUserID, err := uuid.Parse(userIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	var user models.User
	err = h.DB.QueryRow("SELECT id, username, display_name, bio, qr_code_identifier, created_at, updated_at, is_active FROM users WHERE id = $1 AND is_active = TRUE", targetUserID).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.Bio, &user.QRCodeIdentifier, &user.CreatedAt, &user.UpdatedAt, &user.IsActive,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if err != nil {
		log.Printf("Error fetching user profile by ID (%s): %v", targetUserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetCurrentUserProfile handles fetching the currently authenticated user's profile.
func (h *UserHandler) GetCurrentUserProfile(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
		return
	}
	currentUserID := userIDVal.(uuid.UUID) // Type assertion
	
	var user models.User
	err := h.DB.QueryRow("SELECT id, username, display_name, bio, qr_code_identifier, created_at, updated_at, is_active FROM users WHERE id = $1 AND is_active = TRUE", currentUserID).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.Bio, &user.QRCodeIdentifier, &user.CreatedAt, &user.UpdatedAt, &user.IsActive,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Authenticated user not found"})
		return
	}
	if err != nil {
		log.Printf("Error fetching current user profile by ID (%s): %v", currentUserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch your profile"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUserProfileRequest defines the allowed fields for updating a user profile.
type UpdateUserProfileRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	// Add other updatable fields here, e.g., ProfileImageURL, etc.
	// Do NOT include Username, Password (handled separately), Email (if sensitive, handle separately)
}


// UpdateCurrentUserProfile handles updating the currently authenticated user's profile.
func (h *UserHandler) UpdateCurrentUserProfile(c *gin.Context) {
    userIDVal, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
        return
    }
    currentUserID := userIDVal.(uuid.UUID)

    var req UpdateUserProfileRequest 
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
        return
    }

    // Simple validation: at least one field must be provided for update.
    // A more robust validation would check if the provided values are actually different or meet certain criteria.
    if req.DisplayName == "" && req.Bio == "" { // This logic is too simple, if user wants to clear a field.
        // Better: check if request body is empty or if all known fields are nil/empty after parsing.
        // For now, we'll proceed and let the DB update happen.
        // Consider what happens if a user sends an empty JSON {}
    }

    // Build the query dynamically based on provided fields.
    // For simplicity, we'll update both if provided or keep existing if not.
    // This is NOT ideal. A better way is to fetch user, update fields, then save.
    // Or, use COALESCE or CASE in SQL, or build query string.
    // For now, direct update of specific fields:
    
    // Fetch current user data first to only update provided fields
    var currentUserData models.User
    err := h.DB.QueryRow("SELECT display_name, bio FROM users WHERE id = $1", currentUserID).Scan(&currentUserData.DisplayName, &currentUserData.Bio)
    if err != nil {
        log.Printf("Update User: Error fetching current user data for ID (%s): %v", currentUserID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve current profile data for update"})
        return
    }

    // If request field is empty, use current value. This is not how PATCH typically works.
    // Better to use a map[string]interface{} for updates or check explicitly.
    // For a PATCH-like behavior, only update fields that are explicitly set in the request.
    // This requires a more complex request model or checking for nil pointers for optional fields.
    // Let's simplify: if a field is in the request (even if empty string), it's updated.
    // If you want to allow clearing fields, this is okay. If not, add validation.

    finalDisplayName := req.DisplayName
    finalBio := req.Bio
    // This simplistic approach means if a field is omitted in JSON, it's not updated by this query.
    // However, if it's sent as an empty string `""`, it WILL be updated to an empty string.

    // Let's refine: we only update what is provided.
    // This is still basic. A real app might use ORM or more advanced query building.
    query := "UPDATE users SET updated_at = $1"
    args := []interface{}{time.Now().UTC()}
    argId := 2 

    if req.DisplayName != "" { // Only update if display name is provided
        query += fmt.Sprintf(", display_name = $%d", argId)
        args = append(args, req.DisplayName)
        argId++
    }
    if req.Bio != "" { // Only update if bio is provided
         query += fmt.Sprintf(", bio = $%d", argId)
        args = append(args, req.Bio)
        argId++
    }
    
    if argId == 2 { // No fields were actually added to update
        c.JSON(http.StatusBadRequest, gin.H{"error": "No updateable fields (display_name, bio) provided with non-empty values."})
        return
    }

    query += fmt.Sprintf(" WHERE id = $%d", argId)
    args = append(args, currentUserID)
    
    result, err := h.DB.Exec(query, args...)
    if err != nil {
        log.Printf("Error updating user profile for ID (%s): %v, Query: %s, Args: %v", currentUserID, err, query, args)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
        return
    }
    
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        log.Printf("Error getting rows affected for ID (%s): %v", currentUserID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile (check rows)"})
        return
    }
    if rowsAffected == 0 {
         c.JSON(http.StatusNotFound, gin.H{"error": "User not found or no changes made"}) // Or 304 Not Modified if no actual change
        return
    }

    var updatedUser models.User
    err = h.DB.QueryRow("SELECT id, username, display_name, bio, qr_code_identifier, created_at, updated_at, is_active FROM users WHERE id = $1", currentUserID).Scan(
		&updatedUser.ID, &updatedUser.Username, &updatedUser.DisplayName, &updatedUser.Bio, &updatedUser.QRCodeIdentifier, &updatedUser.CreatedAt, &updatedUser.UpdatedAt, &updatedUser.IsActive,
	)
    if err != nil {
        log.Printf("Error fetching updated user profile for ID (%s): %v", currentUserID, err)
        c.JSON(http.StatusOK, gin.H{"message": "Profile updated, but failed to fetch updated data. Please refresh."})
        return
    }
    c.JSON(http.StatusOK, updatedUser)
}
