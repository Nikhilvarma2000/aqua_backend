package controllers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
	"aquahome/utils"
)

// GetUserProfile returns the profile of the authenticated user
func GetUserProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		log.Printf("Error fetching user: %v", err)

		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfileRequest contains the data for profile update
type UpdateProfileRequest struct {
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
	ProfilePicture string `json:"profile_picture"`
}

// UpdateUserProfile updates the profile of the authenticated user
func UpdateUserProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var updateRequest UpdateProfileRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	updates := map[string]interface{}{}
	if updateRequest.Name != "" {
		updates["name"] = updateRequest.Name
	}
	if updateRequest.Phone != "" {
		updates["phone"] = updateRequest.Phone
	}
	if updateRequest.Address != "" {
		updates["address"] = updateRequest.Address
	}
	if updateRequest.ProfilePicture != "" {
		updates["profile_picture"] = updateRequest.ProfilePicture
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	if err := database.DB.Model(&database.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		log.Printf("Failed to update profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	var updatedUser database.User
	if err := database.DB.First(&updatedUser, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving updated profile"})
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// ChangePasswordRequest contains the data for password change
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword changes the user's password
func ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var changePassRequest ChangePasswordRequest
	if err := c.ShouldBindJSON(&changePassRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	if !utils.CheckPasswordHash(changePassRequest.CurrentPassword, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	newPasswordHash, err := utils.HashPassword(changePassRequest.NewPassword)
	if err != nil {
		log.Printf("Hashing error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing new password"})
		return
	}

	if err := database.DB.Model(&user).Update("password_hash", newPasswordHash).Error; err != nil {
		log.Printf("Failed to update password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// GetUserByID gets user details by ID (Admin only)
func GetUserByID(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("DB error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetUsersByRole gets users by role (Admin only)
func GetUsersByRole(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userRole := c.Param("role")
	if userRole == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role parameter is required"})
		return
	}

	var users []database.User
	if err := database.DB.Where("role = ?", userRole).Find(&users).Error; err != nil {
		log.Printf("DB error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, users)
}
