package controllers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
	"aquahome/utils"
)

// LoginRequest and RegisterRequest are already defined in auth_controller.go
// Using the same structure but with different names to avoid redeclaration

// RegisterRequestNew extends the original RegisterRequest with additional fields
type RegisterRequestNew struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=customer franchise_owner service_agent admin"`
	Address  string `json:"address"`
	City     string `json:"city"`
	State    string `json:"state"`
	ZipCode  string `json:"zipCode"`
}

// LoginNew handles user authentication and returns a JWT token
// using GORM instead of raw SQL
func LoginNew(c *gin.Context) {
	var loginRequest LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	var user database.User
	err := database.DB.Where("email = ?", loginRequest.Email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Verify password
	if !utils.CheckPasswordHash(loginRequest.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT token
	expiryTime := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateJWT(user.ID, user.Email, strings.ToLower(user.Role), expiryTime)

	if err != nil {
		log.Printf("JWT error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Update last login time
	if err := database.DB.Model(&user).Update("last_login", time.Now()).Error; err != nil {
		log.Printf("Warning: Failed to update last login time: %v", err)
		// Continue despite this error
	}

	// Remove sensitive fields from response
	user.Password = ""
	user.PasswordHash = ""

	c.JSON(http.StatusOK, LoginResponse{
		Token:  token,
		User:   user,
		Expiry: expiryTime.Unix(),
	})
}

// RegisterNew handles user registration using GORM
func RegisterNew(c *gin.Context) {
	var registerRequest RegisterRequestNew
	if err := c.ShouldBindJSON(&registerRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email already exists
	var existingUser database.User
	err := database.DB.Where("email = ?", registerRequest.Email).First(&existingUser).Error
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already in use"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Hash password
	var hashedPassword string
	if registerRequest.Role == database.RoleCustomer {
		hashedPassword, err = utils.HashPassword(registerRequest.Password)
	} else {
		// Set default password for franchise_owner, service_agent, or admin
		hashedPassword, err = utils.HashPassword("12345678")
	}

	if err != nil {
		log.Printf("Password hashing error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Only allow regular users to register as customers
	if registerRequest.Role != database.RoleCustomer {
		// Check if the creating user is an admin
		adminToken := c.GetHeader("X-Admin-Token")
		if adminToken != utils.GetAdminToken() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot register as non-customer role"})
			return
		}
	}

	// Create new user
	user := database.User{
		Name:         registerRequest.Name,
		Email:        registerRequest.Email,
		Phone:        registerRequest.Phone,
		PasswordHash: hashedPassword,
		Role:         registerRequest.Role,
		Address:      registerRequest.Address,
		City:         registerRequest.City,
		State:        registerRequest.State,
		ZipCode:      registerRequest.ZipCode,
		//CreatedAt:    time.Now(),
		//UpdatedAt:    time.Now(),
	}

	// Start transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Create the user
	// Create the user
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		log.Printf("User creation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// If role is franchise_owner, also create a matching franchise
	// ✅ If role is franchise_owner, create and link franchise
	if registerRequest.Role == database.RoleFranchiseOwner {
		franchise := database.Franchise{
			OwnerID:       user.ID,
			Name:          user.Name,
			Address:       user.Address,
			City:          user.City,
			State:         user.State,
			ZipCode:       user.ZipCode,
			Phone:         user.Phone,
			Email:         user.Email,
			IsActive:      false,
			ApprovalState: "pending", // change to "approved" if you want auto-approve
		}

		if err := tx.Create(&franchise).Error; err != nil {
			tx.Rollback()
			log.Printf("Franchise creation error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create franchise"})
			return
		}

		// ✅ Link the franchise to the user
		user.FranchiseID = &franchise.ID
		if err := tx.Save(&user).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to update user with franchise ID: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link franchise to user"})
			return
		}
	}

	// Create a welcome notification
	notification := database.Notification{
		UserID:  user.ID,
		Title:   "Welcome to AquaHome",
		Message: "Thank you for registering with AquaHome! We're excited to have you with us.",
		Type:    "welcome",
		IsRead:  false,
		//CreatedAt: time.Now(),
		//UpdatedAt: time.Now(),
	}

	if err := tx.Create(&notification).Error; err != nil {
		tx.Rollback()
		log.Printf("Notification creation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create welcome notification"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Generate token for the new user
	expiryTime := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateJWT(user.ID, user.Email, strings.ToLower(user.Role), expiryTime)

	if err != nil {
		log.Printf("JWT error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User created but failed to generate token"})
		return
	}

	// Return the user with token
	user.Password = ""
	user.PasswordHash = ""

	c.JSON(http.StatusCreated, LoginResponse{
		Token:  token,
		User:   user,
		Expiry: expiryTime.Unix(),
	})
}

// RefreshTokenNew generates a new token for a logged in user using GORM
func RefreshTokenNew(c *gin.Context) {
	userID, _ := c.Get("user_id")
	email, _ := c.Get("email")
	role, _ := c.Get("role")

	// Convert userID to uint
	var userIDUint uint
	if id, ok := userID.(uint); ok {
		userIDUint = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Generate a new token
	expiryTime := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateJWT(userIDUint, email.(string), role.(string), expiryTime)
	if err != nil {
		log.Printf("JWT error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":  token,
		"expiry": expiryTime.Unix(),
	})
}

// ForgotPasswordNew sends a password reset token to user's email using GORM
func ForgotPasswordNew(c *gin.Context) {
	var request struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the user
	var user database.User
	err := database.DB.Where("email = ?", request.Email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Don't reveal if the email exists or not for security
			c.JSON(http.StatusOK, gin.H{"message": "If your email is registered, you will receive a password reset link"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Generate a reset token
	resetToken := utils.GenerateResetToken()
	expiryTime := time.Now().Add(30 * time.Minute)

	// Store the reset token in database
	resetRequest := database.PasswordReset{
		UserID:    user.ID,
		Token:     resetToken,
		ExpiresAt: expiryTime,
		//CreatedAt: time.Now(),
	}

	if err := database.DB.Create(&resetRequest).Error; err != nil {
		log.Printf("Reset token creation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// In a real application, send an email with the reset token/link
	// For now, just return the token (would be security issue in production)
	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset link has been sent to your email",
		"token":   resetToken, // In production, remove this and only send via email
	})
}

// ResetPasswordNew resets the user's password using a token with GORM
func ResetPasswordNew(c *gin.Context) {
	var request struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the reset token
	var resetRequest database.PasswordReset
	err := database.DB.Where("token = ? AND expires_at > ?", request.Token, time.Now()).First(&resetRequest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired token"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Hash the new password
	hashedPassword, err := utils.HashPassword(request.NewPassword)
	if err != nil {
		log.Printf("Password hashing error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Start transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Update the user's password
	if err := tx.Model(&database.User{}).Where("id = ?", resetRequest.UserID).Update("password_hash", hashedPassword).Error; err != nil {
		tx.Rollback()
		log.Printf("Password update error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Delete used reset token
	if err := tx.Delete(&resetRequest).Error; err != nil {
		tx.Rollback()
		log.Printf("Token deletion error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete password reset"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully"})
}
