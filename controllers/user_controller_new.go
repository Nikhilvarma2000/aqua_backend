package controllers

import (
	"aquahome/database"
	"aquahome/utils"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetUserProfileNew returns the profile of the authenticated user using GORM
func GetUserProfileNew(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert userID to uint
	var userIDUint uint
	if id, ok := userID.(uint); ok {
		userIDUint = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var user database.User
	err := database.DB.Where("id = ?", userIDUint).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Hide sensitive fields
	user.Password = ""
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// UpdateProfileRequestNew contains the data for profile update (GORM version)
type UpdateProfileRequestNew struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
}

// UpdateUserProfileNew updates the profile of the authenticated user using GORM
func UpdateUserProfileNew(c *gin.Context) {

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert userID to uint
	var userIDUint uint
	if id, ok := userID.(uint); ok {
		userIDUint = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var updateRequest UpdateProfileRequestNew
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Retrieve user first
	var user database.User
	err := database.DB.Where("id = ?", userIDUint).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Update only provided fields
	updateMap := make(map[string]interface{})
	if updateRequest.Name != "" {
		updateMap["name"] = updateRequest.Name
	}
	if updateRequest.Phone != "" {
		updateMap["phone"] = updateRequest.Phone
	}
	if updateRequest.Address != "" {
		updateMap["address"] = updateRequest.Address
	}
	if updateRequest.City != "" {
		updateMap["city"] = updateRequest.City
	}
	if updateRequest.State != "" {
		updateMap["state"] = updateRequest.State
	}
	if updateRequest.ZipCode != "" {
		updateMap["zip_code"] = updateRequest.ZipCode
	}
	updateMap["updated_at"] = time.Now()

	// Update the user
	if len(updateMap) > 0 {
		err = database.DB.Model(&user).Updates(updateMap).Error
		if err != nil {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating profile"})
			return
		}
	}

	// Get updated user profile
	err = database.DB.Where("id = ?", userIDUint).First(&user).Error
	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving updated profile"})
		return
	}

	// 🔁 Auto-create franchise if role is franchise_owner and no franchise is linked
	if user.Role == "franchise_owner" && user.FranchiseID == nil {
		var existingFranchise database.Franchise
		err := database.DB.Where("owner_id = ?", user.ID).First(&existingFranchise).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			franchise := database.Franchise{
				OwnerID:       user.ID,
				Name:          user.Name,
				Address:       user.Address,
				City:          user.City,
				State:         user.State,
				ZipCode:       user.ZipCode,
				Phone:         user.Phone,
				Email:         user.Email,
				IsActive:      true,
				ApprovalState: "approved",
			}

			if err := database.DB.Create(&franchise).Error; err != nil {
				log.Printf("❌ Failed to create franchise for user ID %d: %v", user.ID, err)
			} else {
				user.FranchiseID = &franchise.ID
				if err := database.DB.Save(&user).Error; err != nil {
					log.Printf("❌ Failed to update user with new franchise ID: %v", err)
				} else {
					log.Printf("✅ Franchise (ID %d) created and linked to user ID %d", franchise.ID, user.ID)
				}
			}
		}
	}

	// Hide sensitive fields
	user.Password = ""
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// ChangePasswordRequestNew contains the data for password change (GORM version)
type ChangePasswordRequestNew struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

// ChangePasswordNew changes the user's password using GORM
func ChangePasswordNew(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert userID to uint
	var userIDUint uint
	if id, ok := userID.(uint); ok {
		userIDUint = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var changePassRequest ChangePasswordRequestNew
	if err := c.ShouldBindJSON(&changePassRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Get user with current password hash
	var user database.User
	err := database.DB.Select("id, password_hash").Where("id = ?", userIDUint).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Verify current password
	if !utils.CheckPasswordHash(changePassRequest.CurrentPassword, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Hash new password
	newPasswordHash, err := utils.HashPassword(changePassRequest.NewPassword)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing request"})
		return
	}

	// Update password
	err = database.DB.Model(&user).Updates(map[string]interface{}{
		"password_hash": newPasswordHash,
		"updated_at":    time.Now(),
	}).Error
	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// GetUserByIDNew gets user details by ID (Admin only) using GORM
func GetUserByIDNew(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != database.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user database.User
	err = database.DB.Where("id = ?", uint(userID)).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Hide sensitive fields
	user.Password = ""
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// GetUsersByRoleNew gets users by role (Admin only) using GORM
func GetUsersByRoleNew(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != database.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userRole := c.Param("role")
	if userRole == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role parameter is required"})
		return
	}

	var users []database.User
	err := database.DB.Where("role = ?", userRole).Find(&users).Error
	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Hide sensitive fields
	for i := range users {
		users[i].Password = ""
		users[i].PasswordHash = ""
	}

	c.JSON(http.StatusOK, users)
}
func UpdateUserLocation(c *gin.Context) {
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Latitude = req.Latitude
	user.Longitude = req.Longitude

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Location updated successfully",
		"user":    user,
	})
}

// GetServiceAgentsForFranchise lists all service agents for franchise owners
func GetServiceAgentsForFranchise(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "franchise_owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	var agents []database.User
	if err := database.DB.
		Where("role = ?", "service_agent").
		Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service agents"})
		return
	}

	// Hide sensitive fields
	for i := range agents {
		agents[i].Password = ""
		agents[i].PasswordHash = ""
	}

	c.JSON(http.StatusOK, agents)
}
