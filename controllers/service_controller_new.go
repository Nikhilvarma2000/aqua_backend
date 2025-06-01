package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
)

// Using the existing request types from original service_controller.go to avoid redeclaration
// and using the same ServiceRequestWithDetails types from the original controller

// ServiceRequestWithDetails is a combined structure for fetching service requests with related data
type ServiceRequestWithDetails struct {
	ID               uint       `json:"id"`
	Type             string     `json:"type"`
	Status           string     `json:"status"`
	Description      string     `json:"description"`
	ScheduledTime    *time.Time `json:"scheduled_time"`
	CompletionTime   *time.Time `json:"completion_time"`
	Rating           *int       `json:"rating"`
	Feedback         string     `json:"feedback"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CustomerID       uint       `json:"customer_id"`
	CustomerName     string     `json:"customer_name"`
	CustomerEmail    string     `json:"customer_email"`
	CustomerPhone    string     `json:"customer_phone"`
	ProductID        uint       `json:"product_id"`
	ProductName      string     `json:"product_name"`
	SubscriptionID   uint       `json:"subscription_id"`
	FranchiseID      *uint      `json:"franchise_id"`
	FranchiseName    string     `json:"franchise_name"`
	ServiceAgentID   *uint      `json:"service_agent_id"`
	ServiceAgentName string     `json:"service_agent_name"`
}

// GetServiceRequests returns service requests based on user role
func GetServiceRequestsNew(c *gin.Context) {
	log.Println("🔍 CONTEXT KEYS:", c.Keys)

	userIDValue, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert interface{} to uint
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	roleValue, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	role, ok := roleValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role format"})
		return
	}

	// Define the query to fetch service requests based on role
	query := database.DB.Model(&database.ServiceRequest{})

	// Setup common joins
	query = query.Joins("JOIN users as customer ON service_requests.customer_id = customer.id")
	query = query.Joins("JOIN subscriptions ON service_requests.subscription_id = subscriptions.id")
	query = query.Joins("JOIN products ON subscriptions.product_id = products.id")
	query = query.Joins("LEFT JOIN franchises ON subscriptions.franchise_id = franchises.id")
	query = query.Joins("LEFT JOIN users as service_agent ON service_requests.service_agent_id = service_agent.id")

	var userIDs []uint
	// Apply role-specific filters
	switch role {
	case database.RoleAdmin:
		// Admin can see all service requests - no additional filters
	case database.RoleFranchiseOwner:
		// Franchise owner can see service requests assigned to their franchise
		fmt.Println("userID is ", userID)
		var franchise database.Franchise
		if err := database.DB.Where("owner_id = ?", userID).First(&franchise).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch franchise"})
			return
		}

		fmt.Println("franchise is ", franchise.ID)
		var zipCodesArray []string
		if err := database.DB.Table("franchise_locations").
			Joins("JOIN locations ON franchise_locations.location_id = locations.id").
			Where("franchise_locations.franchise_id = ?", franchise.ID).
			Pluck("locations.zip_codes", &zipCodesArray).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ZIP codes"})
			return
		}
		fmt.Println("zipCodesArray is ", zipCodesArray)
		var zipCodes []string
		for _, zipArray := range zipCodesArray {
			zipArray = strings.Trim(zipArray, "{}")
			if zipArray == "" {
				continue
			}
			individualZips := strings.Split(zipArray, ",")
			for _, zip := range individualZips {
				zip = strings.TrimSpace(zip)
				if zip != "" {
					zipCodes = append(zipCodes, zip)
				}
			}
		}
		fmt.Println("zipCodes is ", zipCodes)
		var users []database.User
		if err := database.DB.Where("zip_code IN ?", zipCodes).
			Where("role = ?", "customer").
			Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}
		fmt.Println("users is ", users)

		// Extract user IDs

		for _, u := range users {
			userIDs = append(userIDs, u.ID)
		}
		fmt.Println("userIDs is ", userIDs)
		// query = query.Where("service_requests.customer_id IN ?", userIDs)

	case database.RoleServiceAgent:
		// Service agent can see service requests assigned to them
		query = query.Where("service_requests.service_agent_id = ?", userID)
	case database.RoleCustomer:
		// Customer can see their own service requests
		query = query.Where("service_requests.customer_id = ?", userID)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
		return
	}

	// Build the select statement with all required fields
	query = query.Select(`
                service_requests.id,
                service_requests.type,
                service_requests.status,
                service_requests.description,
                service_requests.scheduled_time,
                service_requests.completion_time,
                service_requests.rating,
                service_requests.feedback,
                service_requests.created_at,
                service_requests.updated_at,
                service_requests.customer_id,
                customer.name as customer_name,
                customer.email as customer_email,
                customer.phone as customer_phone,
                subscriptions.product_id,
                products.name as product_name,
                service_requests.subscription_id,
                franchises.id as franchise_id,
                franchises.name as franchise_name,
                service_requests.service_agent_id,
                service_agent.name as service_agent_name
        `)

	// Order by creation date, descending
	query = query.Order("service_requests.created_at DESC")

	// Execute the query
	var results []ServiceRequestWithDetails
	if err := query.Find(&results).Error; err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	userIDSet := make(map[uint]struct{})
	for _, id := range userIDs {
		userIDSet[id] = struct{}{}
	}

	var filteredResults []ServiceRequestWithDetails
	for _, item := range results {
		if _, ok := userIDSet[item.CustomerID]; ok {
			filteredResults = append(filteredResults, item)
		}
	}

	c.JSON(http.StatusOK, results)
}

// GetServiceRequestByIDNew returns a specific service request
func GetServiceRequestByIDNew(c *gin.Context) {
	requestID := c.Param("id")
	requestIDInt, err := strconv.ParseUint(requestID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userIDValue, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert interface{} to uint
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	roleValue, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	role, ok := roleValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role format"})
		return
	}

	// Check if the user has permission to view this service request
	var count int64
	query := database.DB.Model(&database.ServiceRequest{})

	switch role {
	case database.RoleAdmin:
		// Admin can see any service request
		query = query.Where("id = ?", requestIDInt)
	case database.RoleFranchiseOwner:
		// Check if the service request belongs to this franchise owner
		query = query.Joins("JOIN subscriptions ON service_requests.subscription_id = subscriptions.id")
		query = query.Joins("JOIN franchises ON subscriptions.franchise_id = franchises.id")
		query = query.Where("service_requests.id = ? AND franchises.owner_id = ?", requestIDInt, userID)
	case database.RoleServiceAgent:
		// Check if the service request is assigned to this service agent
		query = query.Where("id = ? AND service_agent_id = ?", requestIDInt, userID)
	case database.RoleCustomer:
		// Check if the service request belongs to this customer
		query = query.Where("id = ? AND customer_id = ?", requestIDInt, userID)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
		return
	}

	if err := query.Count(&count).Error; err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view this service request"})
		return
	}

	// Fetch the service request details
	var result ServiceRequestWithDetails

	detailQuery := database.DB.Model(&database.ServiceRequest{}).
		Joins("JOIN users as customer ON service_requests.customer_id = customer.id").
		Joins("JOIN subscriptions ON service_requests.subscription_id = subscriptions.id").
		Joins("JOIN products ON subscriptions.product_id = products.id").
		Joins("LEFT JOIN franchises ON subscriptions.franchise_id = franchises.id").
		Joins("LEFT JOIN users as service_agent ON service_requests.service_agent_id = service_agent.id").
		Where("service_requests.id = ?", requestIDInt).
		Select(`
                        service_requests.id,
                        service_requests.type,
                        service_requests.status,
                        service_requests.description,
                        service_requests.scheduled_time,
                        service_requests.completion_time,
                        service_requests.notes,
                        service_requests.rating,
                        service_requests.feedback,
                        service_requests.created_at,
                        service_requests.updated_at,
                        service_requests.customer_id,
                        customer.name as customer_name,
                        customer.email as customer_email,
                        customer.phone as customer_phone,
                        customer.address as customer_address,
                        subscriptions.product_id,
                        products.name as product_name,
                        service_requests.subscription_id,
                        franchises.id as franchise_id,
                        franchises.name as franchise_name,
                        service_requests.service_agent_id,
                        service_agent.name as service_agent_name
                `)

	if err := detailQuery.First(&result).Error; err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CreateServiceRequestNew creates a new service request
func CreateServiceRequestNew(c *gin.Context) {
	var request ServiceRequestCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDValue, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert interface{} to uint
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Check if subscription exists and belongs to the user
	var subscription database.Subscription
	if err := database.DB.
		Preload("Franchise").
		Where("id = ? AND customer_id = ?", request.SubscriptionID, userID).
		First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found or doesn't belong to you"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Check if subscription is active
	if subscription.Status != SubscriptionStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create service request for inactive subscription"})
		return
	}

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Create service request
	serviceRequest := database.ServiceRequest{
		CustomerID:     userID,
		SubscriptionID: uint(request.SubscriptionID),
		Type:           request.RequestType,
		Status:         database.ServiceStatusPending,
		Description:    request.Description,
	}

	if err := tx.Create(&serviceRequest).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating service request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service request"})
		return
	}

	// Create notification for customer
	customerNotification := database.Notification{
		UserID:      userID,
		Title:       "Service Request Created",
		Message:     "Your service request has been created and is pending assignment.",
		Type:        "service_request",
		RelatedID:   &serviceRequest.ID,
		RelatedType: "service_request",
		IsRead:      false,
	}

	if err := tx.Create(&customerNotification).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating customer notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	// If franchise exists, create notification for franchise owner
	if subscription.FranchiseID != 0 && subscription.Franchise.OwnerID != 0 {
		franchiseOwnerNotification := database.Notification{
			UserID:      subscription.Franchise.OwnerID,
			Title:       "New Service Request",
			Message:     "A new service request has been created and needs your attention.",
			Type:        "service_request",
			RelatedID:   &serviceRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&franchiseOwnerNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating franchise owner notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service request"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      serviceRequest.ID,
		"message": "Service request created successfully",
	})
}

// UpdateServiceRequestNew updates a service request
func UpdateServiceRequestNew(c *gin.Context) {
	requestID := c.Param("id")
	requestIDInt, err := strconv.ParseUint(requestID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var updateRequest ServiceRequestUpdateRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDValue, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert interface{} to uint
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	roleValue, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	role, ok := roleValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role format"})
		return
	}

	// Check if the user has permission to update this service request
	var serviceRequest database.ServiceRequest
	permissionQuery := database.DB.Model(&database.ServiceRequest{})

	fmt.Println("role is ", role)

	fmt.Println("userID is ", userID)



	switch role {
	case database.RoleAdmin:
		// Admin can update any service request
		permissionQuery = permissionQuery.Where("id = ?", requestIDInt)
	case database.RoleFranchiseOwner:
		// Check if the service request belongs to this franchise owner
		// permissionQuery = permissionQuery.
		// 	Joins("JOIN subscriptions ON service_requests.subscription_id = subscriptions.id").
		// 	Joins("JOIN franchises ON subscriptions.franchise_id = franchises.id").
		// 	Where("service_requests.id = ? AND franchises.owner_id = ?", requestIDInt, userID)
		permissionQuery = permissionQuery.Where("id = ?", requestIDInt)
	case database.RoleServiceAgent:
		// Check if the service request is assigned to this service agent
		permissionQuery = permissionQuery.Where("id = ? AND service_agent_id = ?", requestIDInt, userID)
	case database.RoleCustomer:
		// Customers can only update their own service requests with limited fields
		// Only allow cancellation before it's assigned
		if updateRequest.Status != database.ServiceStatusCancelled {
			c.JSON(http.StatusForbidden, gin.H{"error": "Customers can only cancel service requests"})
			return
		}

		if err := database.DB.Where("id = ? AND customer_id = ? AND status = ?",
			requestIDInt, userID, database.ServiceStatusPending).First(&serviceRequest).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusForbidden, gin.H{"error": "You can only cancel pending service requests"})
			} else {
				log.Printf("Database error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			}
			return
		}
		permissionQuery = nil // Skip further permission check since we already verified
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
		return
	}

	// Only perform the permission query if it wasn't already handled (customer case)
	if permissionQuery != nil {
		var count int64
		if err := permissionQuery.Count(&count).Error; err != nil {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this service request"})
			return
		}
	}

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Update service request
	updates := map[string]interface{}{}

	if updateRequest.Status != "" && (role == database.RoleAdmin ||
		role == database.RoleFranchiseOwner ||
		role == database.RoleServiceAgent ||
		(role == database.RoleCustomer && updateRequest.Status == database.ServiceStatusCancelled)) {
		updates["status"] = updateRequest.Status
	}

	if updateRequest.ScheduledDate != "" && (role == database.RoleAdmin ||
		role == database.RoleFranchiseOwner ||
		role == database.RoleServiceAgent) {
		scheduledDate, err := time.Parse(time.RFC3339, updateRequest.ScheduledDate)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduled date format"})
			return
		}
		updates["scheduled_time"] = scheduledDate
	}

	if updateRequest.CompletionDate != "" && (role == database.RoleAdmin ||
		role == database.RoleFranchiseOwner ||
		role == database.RoleServiceAgent) {
		completionDate, err := time.Parse(time.RFC3339, updateRequest.CompletionDate)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid completion date format"})
			return
		}
		updates["completion_time"] = completionDate
	}

	if updateRequest.Notes != "" && (role == database.RoleAdmin ||
		role == database.RoleFranchiseOwner ||
		role == database.RoleServiceAgent) {
		updates["notes"] = updateRequest.Notes
	}

	// Check if agent ID is provided and valid
	if updateRequest.AgentID != 0 && (role == database.RoleAdmin || role == database.RoleFranchiseOwner) {
		// Verify agent exists and is a service agent
		var agentCount int64
		agentQuery := database.DB.Model(&database.User{})

		if role == database.RoleFranchiseOwner {
			// Franchise owners can only assign agents from their franchise
			agentQuery = agentQuery.
				Joins("JOIN franchises ON franchises.id = users.franchise_id").
				Where("users.id = ? AND users.role = ? AND franchises.owner_id = ?",
					updateRequest.AgentID, database.RoleServiceAgent, userID)
		} else {
			// Admins can assign any service agent
			agentQuery = agentQuery.Where("id = ? AND role = ?", updateRequest.AgentID, database.RoleServiceAgent)
		}

		if err := agentQuery.Count(&agentCount).Error; err != nil {
			tx.Rollback()
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}

		if agentCount == 0 {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service agent ID"})
			return
		}

		updates["service_agent_id"] = updateRequest.AgentID

		// If status is not already assigned or later, set it to assigned
		var currentStatus string
		if err := tx.Model(&database.ServiceRequest{}).
			Select("status").
			Where("id = ?", requestIDInt).
			Pluck("status", &currentStatus).Error; err != nil {
			tx.Rollback()
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}

		if currentStatus == database.ServiceStatusPending {
			updates["status"] = database.ServiceStatusAssigned
		}
	}

	if len(updates) == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid updates provided"})
		return
	}

	// Perform the update
	if err := tx.Model(&database.ServiceRequest{}).Where("id = ?", requestIDInt).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating service request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update service request"})
		return
	}

	// Get the updated service request for notifications
	var updatedRequest database.ServiceRequest
	if err := tx.Preload("Customer").First(&updatedRequest, requestIDInt).Error; err != nil {
		tx.Rollback()
		log.Printf("Error retrieving updated service request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Create notifications based on changes
	if updateRequest.Status != "" {
		statusNotification := database.Notification{
			UserID:      updatedRequest.CustomerID,
			Title:       "Service Request Updated",
			Message:     fmt.Sprintf("Your service request status has been updated to %s.", updateRequest.Status),
			Type:        "service_request",
			RelatedID:   &updatedRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&statusNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating status notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	if updateRequest.AgentID != 0 {
		// Notify customer about agent assignment
		agentNotification := database.Notification{
			UserID:      updatedRequest.CustomerID,
			Title:       "Service Agent Assigned",
			Message:     "A service agent has been assigned to your service request.",
			Type:        "service_request",
			RelatedID:   &updatedRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&agentNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating agent notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}

		// Notify agent about assignment
		assignmentNotification := database.Notification{
			UserID:      updateRequest.AgentID,
			Title:       "New Service Assignment",
			Message:     fmt.Sprintf("You have been assigned to service request #%d.", updatedRequest.ID),
			Type:        "service_request",
			RelatedID:   &updatedRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&assignmentNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating assignment notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	if updateRequest.ScheduledDate != "" {
		// Notify customer about scheduled date
		scheduleNotification := database.Notification{
			UserID:      updatedRequest.CustomerID,
			Title:       "Service Visit Scheduled",
			Message:     fmt.Sprintf("Your service request has been scheduled for %s.", updateRequest.ScheduledDate),
			Type:        "service_request",
			RelatedID:   &updatedRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&scheduleNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating schedule notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update service request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Service request updated successfully",
	})
}

// CancelServiceRequestNew cancels a service request (customer endpoint)
func CancelServiceRequestNew(c *gin.Context) {
	requestID := c.Param("id")
	requestIDInt, err := strconv.ParseUint(requestID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userIDValue, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert interface{} to uint
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Check if service request exists and belongs to the user
	var serviceRequest database.ServiceRequest
	if err := database.DB.Where("id = ? AND customer_id = ?", requestIDInt, userID).First(&serviceRequest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service request not found or doesn't belong to you"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Check if the service request can be cancelled
	if serviceRequest.Status != database.ServiceStatusPending &&
		serviceRequest.Status != database.ServiceStatusAssigned &&
		serviceRequest.Status != database.ServiceStatusScheduled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Service request cannot be cancelled in its current state"})
		return
	}

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Update service request status
	if err := tx.Model(&serviceRequest).Update("status", database.ServiceStatusCancelled).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating service request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel service request"})
		return
	}

	// Create notification for customer
	customerNotification := database.Notification{
		UserID:      userID,
		Title:       "Service Request Cancelled",
		Message:     "Your service request has been cancelled.",
		Type:        "service_request",
		RelatedID:   &serviceRequest.ID,
		RelatedType: "service_request",
		IsRead:      false,
	}

	if err := tx.Create(&customerNotification).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating customer notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	// If assigned to a service agent, notify them
	if serviceRequest.ServiceAgentID != nil {
		agentNotification := database.Notification{
			UserID:      *serviceRequest.ServiceAgentID,
			Title:       "Service Request Cancelled",
			Message:     "A service request assigned to you has been cancelled by the customer.",
			Type:        "service_request",
			RelatedID:   &serviceRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&agentNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating agent notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel service request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Service request cancelled successfully",
	})
}

// SubmitServiceFeedbackNew submits customer feedback for a completed service
func SubmitServiceFeedbackNew(c *gin.Context) {
	requestID := c.Param("id")
	requestIDInt, err := strconv.ParseUint(requestID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var feedbackRequest FeedbackRequest
	if err := c.ShouldBindJSON(&feedbackRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDValue, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert interface{} to uint
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Check if service request exists, belongs to the user, and is completed
	var serviceRequest database.ServiceRequest
	if err := database.DB.Where("id = ? AND customer_id = ? AND status = ?",
		requestIDInt, userID, database.ServiceStatusCompleted).First(&serviceRequest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service request not found, doesn't belong to you, or is not completed"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		}
		return
	}

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Update service request with feedback
	rating := feedbackRequest.Rating
	updates := map[string]interface{}{
		"rating":   rating,
		"feedback": feedbackRequest.Feedback,
	}

	if err := tx.Model(&serviceRequest).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating service request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit feedback"})
		return
	}

	// If service request had a service agent, create notification
	if serviceRequest.ServiceAgentID != nil {
		agentNotification := database.Notification{
			UserID:      *serviceRequest.ServiceAgentID,
			Title:       "Service Feedback Received",
			Message:     fmt.Sprintf("You received a %d-star rating for your service.", rating),
			Type:        "service_feedback",
			RelatedID:   &serviceRequest.ID,
			RelatedType: "service_request",
			IsRead:      false,
		}

		if err := tx.Create(&agentNotification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating agent notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit feedback"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Feedback submitted successfully",
	})

}
