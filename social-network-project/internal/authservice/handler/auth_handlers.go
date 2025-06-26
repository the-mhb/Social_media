package handler

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	// Adjust the import path based on your go.mod module name
	"github.com/yourusername/social-network/pkg/models"
)

// AuthHandler struct holds dependencies for authentication handlers.
type AuthHandler struct {
	DB           *sql.DB
	JwtSecretKey []byte
}

// NewAuthHandler creates a new AuthHandler with necessary dependencies.
func NewAuthHandler(db *sql.DB, jwtKey []byte) *AuthHandler {
	return &AuthHandler{
		DB:           db,
		JwtSecretKey: jwtKey,
	}
}

// Register handles user registration.
// Corresponds to the previous registerHandler function.
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Check if username already exists
	var existingUserID uuid.UUID
	err := h.DB.QueryRow("SELECT id FROM users WHERE username = $1", req.Username).Scan(&existingUserID)
	if err != sql.ErrNoRows { // Username exists or another error occurred
		if err == nil { // err is nil means user was found
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
			return
		}
		// Another DB error occurred
		log.Printf("Error checking existing username: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration (db check)"})
		return
	}
	// If err is sql.ErrNoRows, username does not exist, proceed.

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration (hash)"})
		return
	}

	now := time.Now().UTC()
	newUser := models.User{
		ID:               uuid.New(),
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		DisplayName:      req.DisplayName,
		CreatedAt:        now,
		UpdatedAt:        now,
		IsActive:         true,
		QRCodeIdentifier: uuid.NewString(),
		Bio:              "", // Default Bio
	}
	if newUser.DisplayName == "" {
		newUser.DisplayName = newUser.Username
	}

	_, err = h.DB.Exec("INSERT INTO users (id, username, password_hash, display_name, bio, qr_code_identifier, created_at, updated_at, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		newUser.ID, newUser.Username, newUser.PasswordHash, newUser.DisplayName, newUser.Bio, newUser.QRCodeIdentifier, newUser.CreatedAt, newUser.UpdatedAt, newUser.IsActive)
	if err != nil {
		log.Printf("Error inserting new user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully", "user": newUser})
}

// Login handles user login.
// Corresponds to the previous loginHandler function.
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	var user models.User
	err := h.DB.QueryRow("SELECT id, username, password_hash, display_name, bio, qr_code_identifier, created_at, updated_at, is_active FROM users WHERE username = $1 AND is_active = TRUE", req.Username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Bio, &user.QRCodeIdentifier, &user.CreatedAt, &user.UpdatedAt, &user.IsActive,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process login"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	claims := jwt.MapClaims{
		"user_id":  user.ID.String(),
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 72).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.JwtSecretKey)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{Token: tokenString, User: &user})
}
