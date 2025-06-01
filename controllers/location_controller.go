package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"aquahome/database"
)

// FranchiseWithOwner represents a franchise with owner details
type FranchiseWithOwner struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	OwnerName     string `json:"owner_name"`
	OwnerEmail    string `json:"owner_email"`
	OwnerPhone    string `json:"owner_phone"`
	Address       string `json:"address"`
	City          string `json:"city"`
	State         string `json:"state"`
	ZipCode       string `json:"zip_code"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	IsActive      bool   `json:"is_active"`
	ApprovalState string `json:"approval_state"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// FranchiseRequest contains data for franchise creation or update
type FranchiseRequest struct {
	Name        string `json:"name" binding:"required"`
	Address     string `json:"address" binding:"required"`
	City        string `json:"city" binding:"required"`
	State       string `json:"state" binding:"required"`
	ZipCode     string `json:"zip_code" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	LocationIDs []uint `json:"location_ids"` //
}

// CreateFranchise creates a new franchise (Franchise Owner only)
func CreateFranchise(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || (role != "franchise_owner" && role != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	//userID, _ := c.Get("user_id")
	ownerIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	ownerID, ok := ownerIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}
	// Convert to uint for GORM

	var franchiseRequest FranchiseRequest
	if err := c.ShouldBindJSON(&franchiseRequest); err != nil {
		log.Printf("Invalid request data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	fmt.Printf(" Received Payload: %+v\n", franchiseRequest)

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Create franchise using GORM
	franchise := database.Franchise{
		Name:          franchiseRequest.Name,
		OwnerID:       ownerID,
		Address:       franchiseRequest.Address,
		City:          franchiseRequest.City,
		State:         franchiseRequest.State,
		ZipCode:       franchiseRequest.ZipCode,
		Phone:         franchiseRequest.Phone,
		Email:         franchiseRequest.Email,
		IsActive:      false,     // Initially inactive until approved
		ApprovalState: "pending", // Initial approval state
	}

	result := tx.Create(&franchise)
	if result.Error != nil {
		tx.Rollback()
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating franchise"})
		return
	}

	//  Link selected locations to this franchise
	if len(franchiseRequest.LocationIDs) > 0 {
		var locations []database.Location
		if err := tx.Where("id IN ?", franchiseRequest.LocationIDs).Find(&locations).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid location IDs"})
			return
		}
		fmt.Printf(" Locations: %+v\n", locations)

		if err := tx.Model(&franchise).Association("Locations").Replace(&locations); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link locations to franchise"})
			return
		}
		fmt.Printf(" Locations linked to franchise: %+v\n", locations)
		fmt.Printf(" Franchise: %+v\n", franchise)
	}

	franchiseID := franchise.ID

	// Create notification for franchise owner
	ownerNotification := database.Notification{
		UserID:      ownerID,
		Title:       "Franchise Application Submitted",
		Message:     "Your franchise application for " + franchiseRequest.Name + " has been submitted and is pending approval.",
		Type:        "franchise",
		RelatedID:   &franchise.ID,
		RelatedType: "franchise",
	}

	result = tx.Create(&ownerNotification)
	if result.Error != nil {
		tx.Rollback()
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating owner notification"})
		return
	}

	// Create notification for admin
	// First, find an admin user to notify
	var adminUser database.User
	adminResult := database.DB.Where("role = ?", database.RoleAdmin).First(&adminUser)

	if adminResult.Error == nil {
		adminNotification := database.Notification{
			UserID:      adminUser.ID,
			Title:       "New Franchise Application",
			Message:     "A new franchise application has been submitted by " + franchiseRequest.Name + " and requires your approval.",
			Type:        "franchise",
			RelatedID:   &franchise.ID,
			RelatedType: "franchise",
		}

		if err := tx.Create(&adminNotification).Error; err != nil {
			log.Printf("Error creating admin notification: %v", err)
			// Don't roll back for this error, it's not critical
		}
	}

	// Update user with franchise_id
	var user database.User
	if err := tx.First(&user, ownerID).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error finding user"})
		return
	}

	user.FranchiseID = &franchise.ID
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user with franchise ID"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Franchise application submitted successfully. It is pending approval.",
		"id":      franchiseID,
	})
}

// GetFranchises gets all franchises based on user role
func GetFranchises(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, _ := c.Get("user_id")
	var userIDUint uint

	if role != "admin" {
		userIDUint = uint(userID.(float64))
	}

	// Define the response structure
	// Using the already defined FranchiseWithOwner struct

	var franchises []FranchiseWithOwner

	query := database.DB.Table("franchises").
		Select(`
			franchises.id, 
			franchises.name, 
			franchises.address, 
			franchises.city, 
			franchises.state, 
			franchises.zip_code, 
			franchises.phone, 
			franchises.email, 
			franchises.is_active, 
			franchises.approval_state, 
			franchises.created_at, 
			franchises.updated_at, 
			users.name as owner_name, 
			users.email as owner_email, 
			users.phone as owner_phone
		`).
		Joins("JOIN users ON franchises.owner_id = users.id").
		Order("franchises.created_at DESC")

	// Apply role-based filtering
	switch role {
	case "admin":
		// Admin can see all franchises - no additional filters
	case "franchise_owner":
		// Franchise owner can only see their own franchises
		query = query.Where("franchises.owner_id = ?", userIDUint)
	default:
		// Other roles can only see active franchises
		query = query.Where("franchises.is_active = ? AND franchises.approval_state = ?", true, "approved")
	}

	// Execute the query
	result := query.Find(&franchises)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, franchises)
}

// GetFranchiseByID gets a franchise by ID
func PublicGetFranchiseByID(c *gin.Context) {
	franchiseIDStr := c.Param("id")
	franchiseID, err := strconv.ParseUint(franchiseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid franchise ID"})
		return
	}

	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDUint := uint(userID.(float64))

	// Define response structure using FranchiseWithOwner and adding missing fields
	type FranchiseDetail struct {
		database.Franchise
		OwnerName string `json:"owner_name"`
	}

	var franchise FranchiseDetail

	// Create base query
	query := database.DB.Table("franchises").
		Select("franchises.*, users.name as owner_name").
		Joins("JOIN users ON franchises.owner_id = users.id").
		Where("franchises.id = ?", franchiseID)

	// Apply role-based conditions
	switch role {
	case "admin":
		// Admin can see any franchise - no additional filters
	case "franchise_owner":
		// Franchise owner can only see their own franchises
		query = query.Where("franchises.owner_id = ?", userIDUint)
	default:
		// Other roles can only see active franchises
		query = query.Where("franchises.is_active = ? AND franchises.approval_state = ?", true, "approved")
	}

	// Execute query
	result := query.First(&franchise)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found or you don't have permission to view it"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Get statistics if admin or franchise owner
	if role == "admin" || (role == "franchise_owner" && franchise.OwnerID == userIDUint) {
		var activeSubscriptions int64
		var pendingServices int64

		// Get active subscriptions count
		database.DB.Model(&database.Subscription{}).
			Where("franchise_id = ? AND status = ?", franchiseID, database.SubscriptionStatusActive).
			Count(&activeSubscriptions)

		// Get pending service requests count
		database.DB.Model(&database.ServiceRequest{}).
			Where("franchise_id = ? AND status IN (?, ?)",
				franchiseID, database.ServiceStatusPending, database.ServiceStatusScheduled).
			Count(&pendingServices)

		// Return franchise with statistics
		c.JSON(http.StatusOK, gin.H{
			"franchise": franchise,
			"stats": gin.H{
				"active_subscriptions": activeSubscriptions,
				"pending_services":     pendingServices,
			},
		})
		return
	}

	c.JSON(http.StatusOK, franchise)
}

// UpdateFranchise updates a franchise (Franchise Owner or Admin only)
func UpdateFranchise(c *gin.Context) {
	franchiseIDStr := c.Param("id")
	franchiseID, err := strconv.ParseUint(franchiseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid franchise ID"})
		return
	}

	role, exists := c.Get("role")
	if !exists || (role != "admin" && role != "franchise_owner") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDUint := uint(userID.(float64))

	// Find franchise to check existence and ownership
	var franchise database.Franchise
	result := database.DB.First(&franchise, franchiseID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// If franchise owner, check if they own the franchise
	if role == "franchise_owner" && franchise.OwnerID != userIDUint {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this franchise"})
		return
	}

	var franchiseRequest FranchiseRequest
	if err := c.ShouldBindJSON(&franchiseRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Update franchise fields
	franchise.Name = franchiseRequest.Name
	franchise.Address = franchiseRequest.Address
	franchise.City = franchiseRequest.City
	franchise.State = franchiseRequest.State
	franchise.ZipCode = franchiseRequest.ZipCode
	franchise.Phone = franchiseRequest.Phone
	franchise.Email = franchiseRequest.Email

	//  Update linked locations if provided
	if len(franchiseRequest.LocationIDs) > 0 {
		var locations []database.Location
		if err := database.DB.Where("id IN ?", franchiseRequest.LocationIDs).Find(&locations).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid location IDs"})
			return
		}
		if err := database.DB.Model(&franchise).Association("Locations").Replace(&locations); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update linked locations"})
			return
		}
	}

	// If franchise owner is resubmitting a rejected application, update approval state
	if role == "franchise_owner" && franchise.ApprovalState == "rejected" {
		franchise.ApprovalState = "pending"
	}

	// Save changes
	result = database.DB.Save(&franchise)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating franchise"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Franchise updated successfully"})
}

// ApproveFranchise approves a franchise application (Admin only)
func ApproveFranchise(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	franchiseIDStr := c.Param("id")
	franchiseID, err := strconv.ParseUint(franchiseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid franchise ID"})
		return
	}

	// Find franchise to check existence and status
	var franchise database.Franchise
	result := database.DB.First(&franchise, franchiseID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if franchise.ApprovalState == "approved" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise is already approved"})
		return
	}

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Update franchise status
	franchise.ApprovalState = "approved"
	franchise.IsActive = true

	if err := tx.Save(&franchise).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error approving franchise"})
		return
	}

	// Create notification for franchise owner
	notification := database.Notification{
		UserID:      franchise.OwnerID,
		Title:       "Franchise Application Approved",
		Message:     "Your franchise application has been approved. You can now start serving customers.",
		Type:        "franchise",
		RelatedID:   &franchise.ID,
		RelatedType: "franchise",
	}

	if err := tx.Create(&notification).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating notification"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Franchise approved successfully"})
}

// RejectFranchise rejects a franchise application (Admin only)
func RejectFranchise(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	franchiseIDStr := c.Param("id")
	franchiseID, err := strconv.ParseUint(franchiseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid franchise ID"})
		return
	}

	type RejectRequest struct {
		Reason string `json:"reason" binding:"required"`
	}

	var rejectRequest RejectRequest
	if err := c.ShouldBindJSON(&rejectRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reason for rejection is required"})
		return
	}

	// Find franchise to check existence and status
	var franchise database.Franchise
	result := database.DB.First(&franchise, franchiseID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if franchise.ApprovalState == "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise is already rejected"})
		return
	}

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Update franchise status
	franchise.ApprovalState = "rejected"
	franchise.IsActive = false

	if err := tx.Save(&franchise).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error rejecting franchise"})
		return
	}

	// Create notification for franchise owner
	notification := database.Notification{
		UserID:      franchise.OwnerID,
		Title:       "Franchise Application Rejected",
		Message:     "Your franchise application has been rejected. Reason: " + rejectRequest.Reason,
		Type:        "franchise",
		RelatedID:   &franchise.ID,
		RelatedType: "franchise",
	}

	if err := tx.Create(&notification).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating notification"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Franchise rejected successfully"})
}

// Helper function to check if a polygon overlaps with any existing franchise

// GetFranchiseServiceAgents gets service agents associated with a franchise
func GetFranchiseServiceAgents(c *gin.Context) {
	franchiseIDStr := c.Param("id")
	franchiseID, err := strconv.ParseUint(franchiseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid franchise ID"})
		return
	}

	role, exists := c.Get("role")
	if !exists || (role != "admin" && role != "franchise_owner") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDUint := uint(userID.(float64))

	// If franchise owner, check if they own the franchise
	if role == "franchise_owner" {
		var franchise database.Franchise
		result := database.DB.Select("owner_id").First(&franchise, franchiseID)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
				return
			}
			log.Printf("Database error: %v", result.Error)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}

		if franchise.OwnerID != userIDUint {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view this franchise's service agents"})
			return
		}
	}

	// Define response structure for service agents
	type ServiceAgentInfo struct {
		ID             uint   `json:"id"`
		Name           string `json:"name"`
		Email          string `json:"email"`
		Phone          string `json:"phone"`
		ProfilePicture string `json:"profile_picture"`
	}

	var serviceAgents []ServiceAgentInfo

	// Get service agents for the franchise using GORM
	result := database.DB.Model(&database.User{}).
		Select("id, name, email, phone, profile_picture").
		Where("franchise_id = ? AND role = ?", franchiseID, database.RoleServiceAgent).
		Find(&serviceAgents)

	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, serviceAgents)
}

// SearchFranchises searches for franchises by location (Customer only)
func SearchFranchises(c *gin.Context) {
	// This is a simplified search by zip code
	// In a real app, you'd use spatial queries to find franchises serving the customer's location

	zipCode := c.Query("zip_code")
	if zipCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Zip code is required"})
		return
	}

	// Define response structure
	type FranchiseLocation struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		Address string `json:"address"`
		City    string `json:"city"`
		State   string `json:"state"`
		ZipCode string `json:"zip_code"`
	}

	var franchises []FranchiseLocation

	// Get franchises that serve this zip code using GORM
	result := database.DB.Model(&database.Franchise{}).
		Select("id, name, address, city, state, zip_code").
		Where("is_active = ? AND approval_state = ? AND zip_code = ?", true, "approved", zipCode).
		Find(&franchises)

	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, franchises)
}

// GetAllLocations returns all available service locations (Admin only)
func GetAllLocations(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	var locations []database.Location
	if err := database.DB.Find(&locations).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch locations"})
		return
	}

	c.JSON(http.StatusOK, locations)
}
func GetMyLocations(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "franchise_owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID := c.GetUint("user_id")

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	//  Auto-link franchise if not set
	if user.FranchiseID == nil {
		var franchise database.Franchise
		if err := database.DB.Where("owner_id = ?", userID).First(&franchise).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise not linked to your account"})
			return
		}
		user.FranchiseID = &franchise.ID
		_ = database.DB.Save(&user)
	}

	var locations []database.Location
	if err := database.DB.
		Joins("JOIN franchise_locations fl ON fl.location_id = locations.id").
		Where("fl.franchise_id = ?", *user.FranchiseID).
		Find(&locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service areas"})
		return
	}

	c.JSON(http.StatusOK, locations)
}

func AddFranchiseLocations(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "franchise_owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID := c.GetUint("user_id")

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	//  Auto-link franchise if not set
	if user.FranchiseID == nil {
		var franchise database.Franchise
		if err := database.DB.Where("owner_id = ?", userID).First(&franchise).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise not linked to your account"})
			return
		}
		user.FranchiseID = &franchise.ID
		_ = database.DB.Save(&user)
	}

	var req struct {
		Name     string   `json:"name"`
		ZipCodes []string `json:"zipCodes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || len(req.ZipCodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	fmt.Printf(" Received Payload: %+v\n", req)

	var created []database.Location
	for _, zip := range req.ZipCodes {
		fmt.Printf(" Processing Zip Code: %s\n", zip)
		location := database.Location{
			Name:     req.Name,
			ZipCodes: pq.StringArray{zip},
		}
		// Use a map for the WHERE condition to properly handle the array comparison
		if err := database.DB.Where("\"zip_codes\" @> ?", pq.StringArray{zip}).FirstOrCreate(&location).Error; err != nil {
			fmt.Printf(" Error creating location: %v\n", err)
			continue
		}
		fmt.Printf(" Created Location: %+v\n", location)

		link := database.FranchiseLocation{
			FranchiseID: *user.FranchiseID,
			LocationID:  location.ID,
		}
		database.DB.FirstOrCreate(&link, link)
		created = append(created, location)
		fmt.Printf(" Created Location Link: %+v\n", link)
	}

	fmt.Printf(" Created Locations: %+v\n", created)
	c.JSON(http.StatusOK, created)
}


func UpdateFranchiseLocations(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "franchise_owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID := c.GetUint("user_id")

	// Parse body
	var req struct {
		Name     string   `json:"name"`
		ZipCodes []string `json:"zip_codes"`
		IsActive bool     `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("Error binding JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	fmt.Println("userID franchise owner ", userID)

	//need to get frnachise id from franchises table using franchise owner id
	var franchise database.Franchise
	if err := database.DB.Where("owner_id = ?", userID).First(&franchise).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise not linked to your account"})
		return
	}

	// Find the location owned by this franchise owner
	var franchiseLocation database.FranchiseLocation
	if err := database.DB.
		Where("franchise_id = ?", franchise.ID).
		Joins("JOIN locations ON franchise_locations.location_id = locations.id").
		First(&franchiseLocation).Error; err != nil {

		c.JSON(http.StatusNotFound, gin.H{"error": "Location not found or unauthorized"})
		return
	}

	var location database.Location
	if err := database.DB.First(&location, franchiseLocation.LocationID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve location"})
		return
	}

	// Update fields
	location.Name = req.Name
	location.ZipCodes = req.ZipCodes
	location.IsActive = req.IsActive

	if err := database.DB.Save(&location).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	//need to return updated ass like AddFranchiseLocations
	var updatedLocation database.Location
	if err := database.DB.First(&updatedLocation, franchiseLocation.LocationID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve location"})
		return
	}

	c.JSON(http.StatusOK, updatedLocation)
}
