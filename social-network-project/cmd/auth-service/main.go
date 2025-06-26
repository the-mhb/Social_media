package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // PostgreSQL driver
	"golang.org/x/crypto/bcrypt"

	// Assuming your go.mod module is "github.com/yourusername/social-network"
	// If it's different, this path needs to be adjusted.
	// "github.com/yourusername/social-network/pkg/models" // No longer needed here, models are used in handler
	"github.com/yourusername/social-network/internal/authservice/handler"
	// "github.com/yourusername/social-network/internal/authservice/db" // We might create this later for DB specific logic
)

// Global DB connection (for simplicity in this example, consider dependency injection for larger apps)
// var db *sql.DB // This will be passed to the handler
// var jwtSecretKey []byte // This will be passed to the handler

// Models are now imported from pkg/models and used within the handler package

// ensureUsersTableExists (moved to be a local function in main for setup)
func ensureUsersTableExists(dbConn *sql.DB) {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		username VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		display_name VARCHAR(255),
		bio TEXT, 
		qr_code_identifier VARCHAR(255) UNIQUE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		is_active BOOLEAN DEFAULT TRUE
	);`
	// Added bio TEXT to match model

	_, err := dbConn.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Error creating users table: %v. Ensure PostgreSQL is running and accessible.", err)
	}
	log.Println("Users table checked/created successfully.")
}

func main() {
	// Load .env file (optional, useful for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Database Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	log.Println("Successfully connected to the database!")

	// JWT Secret Key
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}
	jwtSecretKey = []byte(secret)

	// Initialize Gin router
	router := gin.Default()

	// Setup routes
	// We will define these handlers in a separate file/package (e.g., internal/authservice/handler)
	// For now, defining them inline for brevity, then refactor.
	authRoutes := router.Group("/auth")
	{
		authRoutes.POST("/register", registerHandler)
		authRoutes.POST("/login", loginHandler)
	}

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port for auth-service
	}
	log.Printf("Auth service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// registerHandler (inline for now, to be refactored)
func registerHandler(c *gin.Context) {
	var req models.RegistrationRequest // Use model from pkg
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Check if username already exists
	var existingUserID uuid.UUID
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", req.Username).Scan(&existingUserID)
	if err != sql.ErrNoRows {
		c.JSON(400, gin.H{"error": "Username already exists"})
		return
	}
	// Important: Check for other errors too, not just sql.ErrNoRows
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking existing username: %v", err)
		c.JSON(500, gin.H{"error": "Failed to process registration (db check)"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.JSON(500, gin.H{"error": "Failed to process registration (hash)"})
		return
	}

	now := time.Now().UTC()
	newUser := models.User{ // Use model from pkg
		ID:               uuid.New(),
		Username:         req.Username,
		PasswordHash:     string(hashedPassword),
		DisplayName:      req.DisplayName,
		CreatedAt:        now,
		UpdatedAt:        now, // Set UpdatedAt on creation
		IsActive:         true,
		QRCodeIdentifier: uuid.NewString(), // Generate a QR identifier
		Bio:              "", // Default Bio
	}
	if newUser.DisplayName == "" {
		newUser.DisplayName = newUser.Username // Default display name
	}

	_, err = db.Exec("INSERT INTO users (id, username, password_hash, display_name, bio, qr_code_identifier, created_at, updated_at, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		newUser.ID, newUser.Username, newUser.PasswordHash, newUser.DisplayName, newUser.Bio, newUser.QRCodeIdentifier, newUser.CreatedAt, newUser.UpdatedAt, newUser.IsActive)
	if err != nil {
		log.Printf("Error inserting new user: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create user"})
		return
	}

	// Create a user object for the response, omitting sensitive data like PasswordHash
	// The User model in pkg/models already has `json:"-"` for PasswordHash
	// We can return the newUser directly as its PasswordHash is already correctly tagged.
	// To be absolutely sure, or if we want to return a subset, we can create a specific response model or copy fields.
	// For now, returning newUser is fine.
	c.JSON(201, gin.H{"message": "User registered successfully", "user": newUser})
}

// loginHandler (inline for now, to be refactored)
func loginHandler(c *gin.Context) {
	var req models.LoginRequest // Use model from pkg
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	var user models.User // Use model from pkg
	// Fetch all relevant user fields
	// Note: Ensure your SELECT query matches the fields in your models.User struct
	err := db.QueryRow("SELECT id, username, password_hash, display_name, bio, qr_code_identifier, created_at, updated_at, is_active FROM users WHERE username = $1 AND is_active = TRUE", req.Username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Bio, &user.QRCodeIdentifier, &user.CreatedAt, &user.UpdatedAt, &user.IsActive,
	)
	if err == sql.ErrNoRows {
		c.JSON(401, gin.H{"error": "Invalid username or password"})
		return
	}
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		c.JSON(500, gin.H{"error": "Failed to process login"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil { // This means password does not match
		c.JSON(401, gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate JWT token
	claims := jwt.MapClaims{
		"user_id":  user.ID.String(), // Use .String() for UUID in claims if needed by client
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 72).Unix(),
		"iat":      time.Now().Unix(),
	}
	// Define the AuthTokenClaims struct in pkg/models if you want typed claims
	// For now, MapClaims is fine.

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		c.JSON(500, gin.H{"error": "Failed to generate token"})
		return
	}

	// The User model from pkg/models already handles PasswordHash omission in JSON.
	c.JSON(200, models.LoginResponse{Token: tokenString, User: &user}) // Use model from pkg, User is a pointer
}

// Helper to create users table if it doesn't exist (for local dev convenience)
// In a real setup, migrations (e.g., sql-migrate, goose, GORM auto-migrate) should handle this.
func ensureUsersTableExists() {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		username VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		display_name VARCHAR(255),
		qr_code_identifier VARCHAR(255) UNIQUE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		is_active BOOLEAN DEFAULT TRUE
	);`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Error creating users table: %v. Ensure PostgreSQL is running and accessible.", err)
	}
	log.Println("Users table checked/created successfully.")
}

// Call this in main after DB connection
func init() {
	// This is a bit of a hack for a main package.
	// Better to call ensureUsersTableExists() from main() after db is initialized.
	// For now, this will panic if db is not set, which is fine as it's a fatal condition.
	// We will call it explicitly in main.
}

// Adding explicit call in main, after db connection
func mainWithTableCreation() {
	// ... (godotenv, dbURL, jwtSecretKey setup as above) ...
	// Load .env file (optional, useful for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Database Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set. Example: postgres://user:pass@host:port/dbname?sslmode=disable")
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Error connecting to database: %v. Check DATABASE_URL and PostgreSQL server.", err)
	}
	log.Println("Successfully connected to the database!")

	// Ensure users table exists (for dev convenience)
	ensureUsersTableExists() // Call the function here

	// JWT Secret Key
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}
	jwtSecretKey = []byte(secret)

	// Initialize Gin router
	router := gin.Default()
	// Simple logging middleware
	router.Use(gin.Logger())
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())


	authRoutes := router.Group("/auth")
	{
		authRoutes.POST("/register", registerHandler)
		authRoutes.POST("/login", loginHandler)
	}
	
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})


	port := os.Getenv("AUTH_SERVICE_PORT") // Changed to specific env var
	if port == "" {
		port = "8080" // Default port for auth-service
	}
	log.Printf("Auth service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Replace the original main with this one
// func main() {
//  mainWithTableCreation()
// }
// For the tool, I need to have only one main function.
// The overwrite tool will replace the entire file. So the main function at the end will be the one used.
// I will combine them into one main function.

func main() {
	// Load .env file (optional, useful for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading, using environment variables")
	}

	// Database Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set. Example: postgres://user:pass@host:port/dbname?sslmode=disable")
	}

	appDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer appDB.Close()

	// It's good practice to set connection pool options
	appDB.SetMaxOpenConns(25)
	appDB.SetMaxIdleConns(25)
	appDB.SetConnMaxLifetime(5 * time.Minute)

	err = appDB.Ping()
	if err != nil {
		log.Fatalf("Error connecting to database: %v. Check DATABASE_URL and PostgreSQL server.", err)
	}
	log.Println("Successfully connected to the database!")

	// Ensure users table exists (for dev convenience)
	ensureUsersTableExists(appDB) // Pass the db connection

	// JWT Secret Key
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		log.Println("Warning: JWT_SECRET_KEY environment variable is not set. Using a default insecure key.")
		secret = "temp-default-insecure-jwt-secret-key-please-change" // Fallback for dev, NOT for production
	}
	jwtKey := []byte(secret)

	// Initialize Gin router
	// gin.SetMode(gin.ReleaseMode) // Uncomment for production
	router := gin.New() // Using gin.New() for more control over middleware
	
	// Middleware
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))
	router.Use(gin.Recovery())

	// Initialize AuthHandler
	// Note: Ensure your go.mod file has the correct module path.
	// If your module is 'myproject', then the import for handler would be 'myproject/internal/authservice/handler'
	authHandler := handler.NewAuthHandler(appDB, jwtKey)

	// Routes
	// Removing /api/v1 prefix from service itself, API Gateway will handle it.
	authRoutes := router.Group("/auth") // Routes will be /auth/register, /auth/login
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)
	}
	
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		// Check DB connection as part of health check
		if err := appDB.Ping(); err != nil { // Use appDB
			c.JSON(503, gin.H{"status": "DOWN", "db_error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "UP", "message": "Auth service is healthy"})
	})


	servicePort := os.Getenv("AUTH_SERVICE_PORT")
	if servicePort == "" {
		servicePort = "8080" // Default port for auth-service from Dockerfile
	}
	log.Printf("Auth service starting on port %s", servicePort)
	if err := router.Run(":" + servicePort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
