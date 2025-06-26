package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
type User struct {
	ID               uuid.UUID `json:"id"`
	Username         string    `json:"username"`
	PasswordHash     string    `json:"-"` // Do not expose password hash in JSON responses
	DisplayName      string    `json:"display_name,omitempty"`
	Bio              string    `json:"bio,omitempty"`
	QRCodeIdentifier string    `json:"qr_code_identifier,omitempty"` // Identifier for QR code generation
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	IsActive         bool      `json:"is_active"`
}

// RegistrationRequest represents the data needed for a new user registration.
type RegistrationRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Password    string `json:"password" validate:"required,min=8,max=100"`
	DisplayName string `json:"display_name,omitempty"`
}

// LoginRequest represents the data needed for a user to log in.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the data returned after a successful login.
type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"` // Optional: for refresh token mechanism
	User         *User  `json:"user,omitempty"`          // Optional: return user details on login
}

// AuthTokenClaims represents the JWT claims.
type AuthTokenClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	// Standard claims - add more as needed (e.g., roles, permissions)
	// jwt.StandardClaims
}
