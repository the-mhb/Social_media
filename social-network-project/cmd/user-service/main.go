package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	// Adjust the import path based on your go.mod module name
	// "github.com/yourusername/social-network/pkg/models" // Models are used in handler
	"github.com/yourusername/social-network/internal/userservice/handler"
)

// var db *sql.DB // DB connection will be managed in main and passed to handler
// var jwtSecretKey []byte // JWT key will be managed in main and passed to handler


func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading, using environment variables")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set.")
	}

	appDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer appDB.Close()

	appDB.SetMaxOpenConns(20)
	appDB.SetMaxIdleConns(20)
	appDB.SetConnMaxLifetime(5 * time.Minute)

	if err = appDB.Ping(); err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	log.Println("User Service: Successfully connected to the database!")

	// JWT Secret Key (must be the same as in auth-service for token verification)
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		log.Println("Warning: JWT_SECRET_KEY environment variable is not set for User Service. Using a default insecure key.")
		secret = "temp-default-insecure-jwt-secret-key-please-change" // Fallback for dev
	}
	jwtKey := []byte(secret)

	// Initialize Gin router
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP, param.TimeStamp.Format(time.RFC1123), param.Method, param.Path, param.Request.Proto,
			param.StatusCode, param.Latency, param.Request.UserAgent(), param.ErrorMessage,
		)
	}))
	router.Use(gin.Recovery())
	
	// Initialize UserHandler
	// Ensure correct module path for handler import
	userHandler := handler.NewUserHandler(appDB, jwtKey)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		if err := appDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "db_error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "UP", "message": "User service is healthy"})
	})
	
	// API Routes - Removing /api/v1 prefix from service itself
	userRoutes := router.Group("/users") // Routes will be /users/me, /users/:userId
	userRoutes.Use(userHandler.AuthMiddleware())
	{
		userRoutes.GET("/me", userHandler.GetCurrentUserProfile)
		userRoutes.PUT("/me", userHandler.UpdateCurrentUserProfile)
		userRoutes.GET("/:userId", userHandler.GetUserProfile)
	}

	servicePort := os.Getenv("USER_SERVICE_PORT")
	if servicePort == "" {
		servicePort = "8081" // Default port for user-service from Dockerfile
	}
	log.Printf("User service starting on port %s", servicePort)
	if err := router.Run(":" + servicePort); err != nil {
		log.Fatalf("Failed to start User service: %v", err)
	}
}
