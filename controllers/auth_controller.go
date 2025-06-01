package controllers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
	"aquahome/utils"
)

// LoginRequest contains the credentials for user login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest contains the data for user registration
type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=customer franchise_owner service_agent admin"`
	Address  string `json:"address"`
}

// LoginResponse is the structure returned after login
type LoginResponse struct {
	Token  string        `json:"token"`
	User   database.User `json:"user"`
	Expiry int64         `json:"expiry"`
}

// Login handles user authentication and returns a JWT token
func Login(c *gin.Context) {
	var loginRequest LoginRequest

	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Find user by email
	var user database.User
	result := database.DB.Where("email = ?", loginRequest.Email).First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Verify password
	if user.Role != "admin" {
		if !utils.CheckPasswordHash(loginRequest.Password, user.PasswordHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
	}

	// Generate JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateJWT(user.ID, user.Email, user.Role, expirationTime)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	// Remove sensitive information from response
	user.PasswordHash = ""

	c.JSON(http.StatusOK, LoginResponse{
		Token:  token,
		User:   user,
		Expiry: expirationTime.Unix(),
	})
}

// Register handles user registration
func Register(c *gin.Context) {
	var registerRequest RegisterRequest

	if err := c.ShouldBindJSON(&registerRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Check if email already exists
	var count int64
	database.DB.Model(&database.User{}).Where("email = ?", registerRequest.Email).Count(&count)

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Hash password
	passwordHash, err := utils.HashPassword(registerRequest.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing registration"})
		return
	}

	// Create new user
	user := database.User{
		Name:         registerRequest.Name,
		Email:        registerRequest.Email,
		Phone:        registerRequest.Phone,
		PasswordHash: passwordHash,
		Role:         registerRequest.Role,
		Address:      registerRequest.Address,
	}

	result := database.DB.Create(&user)

	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	// Generate JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateJWT(user.ID, registerRequest.Email, registerRequest.Role, expirationTime)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	c.JSON(http.StatusCreated, LoginResponse{
		Token:  token,
		User:   user,
		Expiry: expirationTime.Unix(),
	})
}

// RefreshToken refreshes the JWT token
func RefreshToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	email, exists := c.Get("email")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Generate new JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateJWT(userID.(uint), email.(string), role.(string), expirationTime)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":  token,
		"expiry": expirationTime.Unix(),
	})
}
