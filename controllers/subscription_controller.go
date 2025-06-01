package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
)

// SubscriptionWithProduct represents a subscription with product details
type SubscriptionWithProduct struct {
	ID                uint      `json:"id"`
	OrderID           uint      `json:"order_id"`
	CustomerID        uint      `json:"customer_id"`
	ProductID         uint      `json:"product_id"`
	FranchiseID       uint      `json:"franchise_id"`
	Status            string    `json:"status"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	NextBillingDate   time.Time `json:"next_billing_date"`
	MonthlyRent       float64   `json:"monthly_rent"`
	RentalDuration    int       `json:"rental_duration,omitempty"`
	RemainingDuration int       `json:"remaining_duration,omitempty"`
	AutoRenew         bool      `json:"auto_renew,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ProductName       string    `json:"product_name"`
	ProductImage      string    `json:"product_image"`
	FranchiseName     string    `json:"franchise_name,omitempty"`
	IsActive          bool      `json:"is_active"`
	NextService       time.Time `json:"next_service,omitempty"`
}

// SubscriptionDetail represents detailed subscription information
type SubscriptionDetail struct {
	ID                uint             `json:"id"`
	OrderID           uint             `json:"order_id"`
	CustomerID        uint             `json:"customer_id"`
	ProductID         uint             `json:"product_id"`
	FranchiseID       uint             `json:"franchise_id"`
	Status            string           `json:"status"`
	StartDate         time.Time        `json:"start_date"`
	EndDate           time.Time        `json:"end_date"`
	NextBillingDate   time.Time        `json:"next_billing_date"`
	MonthlyRent       float64          `json:"monthly_rent"`
	RentalDuration    int              `json:"rental_duration,omitempty"`
	RemainingDuration int              `json:"remaining_duration,omitempty"`
	AutoRenew         bool             `json:"auto_renew,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
	ProductName       string           `json:"product_name"`
	ProductImage      string           `json:"product_image"`
	ProductDesc       string           `json:"product_description"`
	FranchiseName     string           `json:"franchise_name,omitempty"`
	FranchisePhone    string           `json:"franchise_phone,omitempty"`
	FranchiseEmail    string           `json:"franchise_email,omitempty"`
	IsActive          bool             `json:"is_active"`
	NextService       time.Time        `json:"next_service,omitempty"`
	LastService       time.Time        `json:"last_service,omitempty"`
	PendingPayment    float64          `json:"pending_payment,omitempty"`
	LastPaymentDate   time.Time        `json:"last_payment_date,omitempty"`
	CustomerName      string           `json:"customer_name,omitempty"`
	CustomerEmail     string           `json:"customer_email,omitempty"`
	CustomerPhone     string           `json:"customer_phone,omitempty"`
	ServiceHistory    []ServiceHistory `json:"service_history,omitempty"`
	PaymentHistory    []PaymentHistory `json:"payment_history,omitempty"`
}

// ServiceHistory represents a service record for a subscription
type ServiceHistory struct {
	ID             uint      `json:"id"`
	Date           time.Time `json:"date"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	AgentName      string    `json:"agent_name,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	CustomerRating int       `json:"customer_rating,omitempty"`
}

// PaymentHistory represents a payment record for a subscription
type PaymentHistory struct {
	ID            uint      `json:"id"`
	Date          time.Time `json:"date"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	Method        string    `json:"method,omitempty"`
	TransactionID string    `json:"transaction_id,omitempty"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
}

// SubscriptionUpdateRequest contains data for updating a subscription
type SubscriptionUpdateRequest struct {
	Status       string `json:"status,omitempty"`
	AutoRenew    *bool  `json:"auto_renew,omitempty"`
	PauseEndDate string `json:"pause_end_date,omitempty"`
}

func GetAllSubscriptions(c *gin.Context) {
	role := c.GetString("role")
	fmt.Println("🔥 Token lo vachina role:", role)

	if role != database.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	var subscriptions []SubscriptionWithProduct

	// Use GORM to fetch subscriptions with related product information
	err := database.DB.Table("subscriptions").
		Select(`
                        subscriptions.id, 
                        subscriptions.order_id, 
                        subscriptions.customer_id, 
                        subscriptions.product_id, 
                        subscriptions.franchise_id, 
                        subscriptions.status, 
                        subscriptions.start_date, 
                        subscriptions.end_date, 
                        subscriptions.next_billing_date, 
                        subscriptions.monthly_rent,
                        subscriptions.created_at, 
                        subscriptions.updated_at,
                        products.name as product_name, 
                        products.image_url as product_image,
                        franchises.name as franchise_name,
                        CASE WHEN subscriptions.status = ? THEN true ELSE false END as is_active,
                        subscriptions.next_maintenance as next_service
                `, database.SubscriptionStatusActive).
		Joins("JOIN products ON subscriptions.product_id = products.id").
		Joins("LEFT JOIN franchises ON subscriptions.franchise_id = franchises.id").
		Order("subscriptions.created_at DESC").
		Find(&subscriptions).Error

	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscriptions"})
		return
	}

	// Add calculated fields
	for i := range subscriptions {
		// Calculate rental duration based on start and end dates
		duration := int(subscriptions[i].EndDate.Sub(subscriptions[i].StartDate).Hours() / 24 / 30)
		subscriptions[i].RentalDuration = duration

		// Calculate remaining duration
		now := time.Now()
		if subscriptions[i].EndDate.After(now) {
			remaining := int(subscriptions[i].EndDate.Sub(now).Hours() / 24 / 30)
			subscriptions[i].RemainingDuration = remaining
		} else {
			subscriptions[i].RemainingDuration = 0
		}

		// Set default auto-renew for now (this would normally come from the database)
		subscriptions[i].AutoRenew = false
	}

	c.JSON(http.StatusOK, subscriptions)
}

// GetCustomerSubscriptions gets subscriptions for the authenticated customer
func GetMySubscriptions(c *gin.Context) {
	role := c.GetString("role")
	if role != database.RoleCustomer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")

	// Convert userID to uint
	var customerID uint
	if id, ok := userID.(uint); ok {
		customerID = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var subscriptions []SubscriptionWithProduct

	// Use GORM to fetch subscriptions with related product information
	err := database.DB.Table("subscriptions").
		Select(`
                        subscriptions.id, 
                        subscriptions.order_id, 
                        subscriptions.customer_id, 
                        subscriptions.product_id, 
                        subscriptions.franchise_id, 
                        subscriptions.status, 
                        subscriptions.start_date, 
                        subscriptions.end_date, 
                        subscriptions.next_billing_date, 
                        subscriptions.monthly_rent,
                        subscriptions.created_at, 
                        subscriptions.updated_at,
                        products.name as product_name, 
                        products.image_url as product_image,
                        franchises.name as franchise_name,
                        CASE WHEN subscriptions.status = ? THEN true ELSE false END as is_active,
                        subscriptions.next_maintenance as next_service
                `, database.SubscriptionStatusActive).
		Joins("JOIN products ON subscriptions.product_id = products.id").
		Joins("LEFT JOIN franchises ON subscriptions.franchise_id = franchises.id").
		Where("subscriptions.customer_id = ?", customerID).
		Order("subscriptions.created_at DESC").
		Find(&subscriptions).Error

	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscriptions"})
		return
	}

	// Add calculated fields
	for i := range subscriptions {
		// Calculate rental duration based on start and end dates
		duration := int(subscriptions[i].EndDate.Sub(subscriptions[i].StartDate).Hours() / 24 / 30)
		subscriptions[i].RentalDuration = duration

		// Calculate remaining duration
		now := time.Now()
		if subscriptions[i].EndDate.After(now) {
			remaining := int(subscriptions[i].EndDate.Sub(now).Hours() / 24 / 30)
			subscriptions[i].RemainingDuration = remaining
		} else {
			subscriptions[i].RemainingDuration = 0
		}

		// Set default auto-renew for now (this would normally come from the database)
		subscriptions[i].AutoRenew = false
	}

	c.JSON(http.StatusOK, subscriptions)
}

// GetSubscriptionDetails gets detailed information for a specific subscription
func GetSubscriptionDetails(c *gin.Context) {
	subscriptionID := c.Param("id")
	subscriptionIDUint, err := strconv.ParseUint(subscriptionID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID"})
		return
	}

	userID := c.GetString("user_id")
	userIDUint, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		log.Printf("Invalid user ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	role := c.GetString("role")

	// Check if the user has permission to view this subscription
	var count int64
	switch role {
	case database.RoleAdmin:
		// Admin can view any subscription
		database.DB.Model(&database.Subscription{}).Where("id = ?", subscriptionIDUint).Count(&count)
	case database.RoleFranchiseOwner:
		// Check if subscription belongs to this franchise owner
		database.DB.Model(&database.Subscription{}).
			Joins("JOIN franchises ON subscriptions.franchise_id = franchises.id").
			Where("subscriptions.id = ? AND franchises.owner_id = ?", subscriptionIDUint, userIDUint).
			Count(&count)
	case database.RoleServiceAgent:
		// Service agents can view subscriptions they're assigned to
		database.DB.Model(&database.Subscription{}).
			Where("id = ? AND service_agent_id = ?", subscriptionIDUint, userIDUint).
			Count(&count)
	case database.RoleCustomer:
		// Customer can only view their own subscriptions
		database.DB.Model(&database.Subscription{}).
			Where("id = ? AND customer_id = ?", subscriptionIDUint, userIDUint).
			Count(&count)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view this subscription"})
		return
	}

	// Fetch detailed subscription information
	var subscriptionDetail SubscriptionDetail

	err = database.DB.Table("subscriptions").
		Select(`
                        subscriptions.id, 
                        subscriptions.order_id, 
                        subscriptions.customer_id, 
                        subscriptions.product_id, 
                        subscriptions.franchise_id, 
                        subscriptions.status, 
                        subscriptions.start_date, 
                        subscriptions.end_date, 
                        subscriptions.next_billing_date, 
                        subscriptions.monthly_rent,
                        subscriptions.created_at, 
                        subscriptions.updated_at,
                        products.name as product_name, 
                        products.image_url as product_image,
                        products.description as product_desc,
                        franchises.name as franchise_name,
                        franchises.phone as franchise_phone,
                        franchises.email as franchise_email,
                        CASE WHEN subscriptions.status = ? THEN true ELSE false END as is_active,
                        subscriptions.next_maintenance as next_service,
                        subscriptions.last_maintenance as last_service,
                        users.name as customer_name,
                        users.email as customer_email,
                        users.phone as customer_phone
                `, database.SubscriptionStatusActive).
		Joins("JOIN products ON subscriptions.product_id = products.id").
		Joins("LEFT JOIN franchises ON subscriptions.franchise_id = franchises.id").
		Joins("JOIN users ON subscriptions.customer_id = users.id").
		Where("subscriptions.id = ?", subscriptionIDUint).
		First(&subscriptionDetail).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscription details"})
		}
		return
	}

	// Calculate rental duration based on start and end dates
	duration := int(subscriptionDetail.EndDate.Sub(subscriptionDetail.StartDate).Hours() / 24 / 30)
	subscriptionDetail.RentalDuration = duration

	// Calculate remaining duration
	now := time.Now()
	if subscriptionDetail.EndDate.After(now) {
		remaining := int(subscriptionDetail.EndDate.Sub(now).Hours() / 24 / 30)
		subscriptionDetail.RemainingDuration = remaining
	} else {
		subscriptionDetail.RemainingDuration = 0
	}

	// Set default auto-renew for now (this would normally come from the database)
	subscriptionDetail.AutoRenew = false

	// Fetch service history
	var serviceHistory []ServiceHistory
	err = database.DB.Table("service_requests").
		Select(`
                        service_requests.id, 
                        service_requests.scheduled_time as date, 
                        service_requests.type, 
                        service_requests.status,
                        service_requests.notes,
                        service_requests.rating as customer_rating,
                        service_agent.name as agent_name
                `).
		Joins("LEFT JOIN users as service_agent ON service_requests.service_agent_id = service_agent.id").
		Where("service_requests.subscription_id = ?", subscriptionIDUint).
		Order("service_requests.scheduled_time DESC").
		Find(&serviceHistory).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Error fetching service history: %v", err)
	} else {
		subscriptionDetail.ServiceHistory = serviceHistory
	}

	// Fetch payment history
	var paymentHistory []PaymentHistory
	err = database.DB.Table("payments").
		Select(`
                        payments.id, 
                        payments.created_at as date, 
                        payments.amount, 
                        payments.status,
                        payments.payment_method as method,
                        payments.transaction_id,
                        payments.invoice_number
                `).
		Where("payments.subscription_id = ?", subscriptionIDUint).
		Order("payments.created_at DESC").
		Find(&paymentHistory).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Error fetching payment history: %v", err)
	} else {
		subscriptionDetail.PaymentHistory = paymentHistory
	}

	// Calculate pending payment amount if any
	var pendingPayment float64
	err = database.DB.Table("payments").
		Select("COALESCE(SUM(amount), 0)").
		Where("subscription_id = ? AND status = ?", subscriptionIDUint, database.PaymentStatusPending).
		Row().Scan(&pendingPayment)

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Error calculating pending payment: %v", err)
	} else {
		subscriptionDetail.PendingPayment = pendingPayment
	}

	// Get last payment date
	var lastPaymentDate time.Time
	err = database.DB.Table("payments").
		Select("created_at").
		Where("subscription_id = ? AND status = ?", subscriptionIDUint, database.PaymentStatusSuccess).
		Order("created_at DESC").
		Limit(1).
		Row().Scan(&lastPaymentDate)

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Error getting last payment date: %v", err)
	} else if !lastPaymentDate.IsZero() {
		subscriptionDetail.LastPaymentDate = lastPaymentDate
	}

	c.JSON(http.StatusOK, subscriptionDetail)
}

// GetFranchiseSubscriptions gets subscriptions for a franchise owner
func GetFranchiseSubscriptions(c *gin.Context) {
	role := c.GetString("role")
	if role != database.RoleFranchiseOwner && role != database.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		log.Println("user_id not found in context")
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID missing"})
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		log.Println("user_id is not of type uint")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	var subscriptions []SubscriptionWithProduct
	query := database.DB.Table("subscriptions").
		Select(`
                        subscriptions.id, 
                        subscriptions.order_id, 
                        subscriptions.customer_id, 
                        subscriptions.product_id, 
                        subscriptions.franchise_id, 
                        subscriptions.status, 
                        subscriptions.start_date, 
                        subscriptions.end_date, 
                        subscriptions.next_billing_date, 
                        subscriptions.monthly_rent,
                        subscriptions.created_at, 
                        subscriptions.updated_at,
                        products.name as product_name, 
                        products.image_url as product_image,
                        users.name as customer_name,
                        users.email as customer_email,
                        CASE WHEN subscriptions.status = ? THEN true ELSE false END as is_active,
                        subscriptions.next_maintenance as next_service
                `, database.SubscriptionStatusActive).
		Joins("JOIN products ON subscriptions.product_id = products.id").
		Joins("JOIN users ON subscriptions.customer_id = users.id")

	if role == database.RoleFranchiseOwner {
		// Franchise owner can only see subscriptions for their franchise
		query = query.Joins("JOIN franchises ON subscriptions.franchise_id = franchises.id").
			Where("franchises.owner_id = ?", userID)

	}

	err := query.
		Order("subscriptions.created_at DESC").
		Find(&subscriptions).Error

	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscriptions"})
		return
	}

	// Add calculated fields
	for i := range subscriptions {
		// Calculate rental duration based on start and end dates
		duration := int(subscriptions[i].EndDate.Sub(subscriptions[i].StartDate).Hours() / 24 / 30)
		subscriptions[i].RentalDuration = duration

		// Calculate remaining duration
		now := time.Now()
		if subscriptions[i].EndDate.After(now) {
			remaining := int(subscriptions[i].EndDate.Sub(now).Hours() / 24 / 30)
			subscriptions[i].RemainingDuration = remaining
		} else {
			subscriptions[i].RemainingDuration = 0
		}

		// Set default auto-renew
		subscriptions[i].AutoRenew = false
	}

	c.JSON(http.StatusOK, subscriptions)
}

// UpdateSubscription updates a subscription
func UpdateSubscription(c *gin.Context) {
	subscriptionID := c.Param("id")
	subscriptionIDUint, err := strconv.ParseUint(subscriptionID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID"})
		return
	}

	var updateRequest SubscriptionUpdateRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	userIDUint, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		log.Printf("Invalid user ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	role := c.GetString("role")

	// Find subscription
	var subscription database.Subscription
	var findErr error

	switch role {
	case database.RoleAdmin:
		// Admin can update any subscription
		findErr = database.DB.First(&subscription, subscriptionIDUint).Error
	case database.RoleFranchiseOwner:
		// Check if subscription belongs to this franchise owner
		findErr = database.DB.
			Joins("JOIN franchises ON subscriptions.franchise_id = franchises.id").
			Where("subscriptions.id = ? AND franchises.owner_id = ?", subscriptionIDUint, userIDUint).
			First(&subscription).Error
	case database.RoleCustomer:
		// Customer can only update their own subscription and only certain fields
		findErr = database.DB.
			Where("id = ? AND customer_id = ?", subscriptionIDUint, userIDUint).
			First(&subscription).Error
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
		return
	}

	if findErr != nil {
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found or you don't have permission"})
		} else {
			log.Printf("Database error: %v", findErr)
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

	// Update subscription fields
	updates := map[string]interface{}{}

	// Status can be updated by admin or franchise owner
	if updateRequest.Status != "" && (role == database.RoleAdmin || role == database.RoleFranchiseOwner) {
		if updateRequest.Status == database.SubscriptionStatusPaused {
			// If pausing, require a pause end date
			if updateRequest.PauseEndDate == "" {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Pause end date is required when pausing a subscription"})
				return
			}

			pauseEndDate, err := time.Parse(time.RFC3339, updateRequest.PauseEndDate)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pause end date format"})
				return
			}

			// Update end date to extend by pause duration
			now := time.Now()
			pauseDuration := pauseEndDate.Sub(now)
			newEndDate := subscription.EndDate.Add(pauseDuration)

			updates["end_date"] = newEndDate
		} else if updateRequest.Status == database.SubscriptionStatusActive &&
			subscription.Status == database.SubscriptionStatusPaused {
			// If resuming from pause, recalculate end date
			// This would normally consider how long it was paused
		}

		updates["status"] = updateRequest.Status
	}

	// Auto renew can be updated by any role
	if updateRequest.AutoRenew != nil {
		updates["auto_renew"] = *updateRequest.AutoRenew
	}

	if len(updates) == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid updates provided"})
		return
	}

	// Apply updates
	if err := tx.Model(&subscription).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription"})
		return
	}

	// Create notification for customer
	if subscription.CustomerID != 0 {
		var message string
		if updateRequest.Status != "" {
			message = "Your subscription status has been updated to " + updateRequest.Status
		} else if updateRequest.AutoRenew != nil {
			if *updateRequest.AutoRenew {
				message = "Auto-renewal has been enabled for your subscription"
			} else {
				message = "Auto-renewal has been disabled for your subscription"
			}
		}

		notification := database.Notification{
			UserID:      subscription.CustomerID,
			Title:       "Subscription Updated",
			Message:     message,
			Type:        "subscription",
			RelatedID:   &subscription.ID,
			RelatedType: "subscription",
			IsRead:      false,
		}

		if err := tx.Create(&notification).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription updated successfully",
	})
}

// CancelSubscription cancels a subscription (customer endpoint)
func CancelSubscription(c *gin.Context) {
	subscriptionID := c.Param("id")
	subscriptionIDUint, err := strconv.ParseUint(subscriptionID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID"})
		return
	}

	userID := c.GetString("user_id")
	userIDUint, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		log.Printf("Invalid user ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Check if subscription exists and belongs to the user
	var subscription database.Subscription
	err = database.DB.Where("id = ? AND customer_id = ?", subscriptionIDUint, userIDUint).First(&subscription).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found or doesn't belong to you"})
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

	// Update subscription status
	if err := tx.Model(&subscription).Update("status", database.SubscriptionStatusCancelled).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel subscription"})
		return
	}

	// Create notification for customer
	customerNotification := database.Notification{
		UserID:      uint(userIDUint),
		Title:       "Subscription Cancelled",
		Message:     "Your subscription has been cancelled.",
		Type:        "subscription",
		RelatedID:   &subscription.ID,
		RelatedType: "subscription",
		IsRead:      false,
	}

	if err := tx.Create(&customerNotification).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating customer notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	// Create notification for franchise if applicable
	if subscription.FranchiseID != 0 {
		// Find franchise owner
		var franchise database.Franchise
		if err := tx.First(&franchise, subscription.FranchiseID).Error; err == nil && franchise.OwnerID != 0 {
			franchiseNotification := database.Notification{
				UserID:      franchise.OwnerID,
				Title:       "Subscription Cancelled",
				Message:     "A customer has cancelled their subscription.",
				Type:        "subscription",
				RelatedID:   &subscription.ID,
				RelatedType: "subscription",
				IsRead:      false,
			}

			if err := tx.Create(&franchiseNotification).Error; err != nil {
				tx.Rollback()
				log.Printf("Error creating franchise notification: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
				return
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription cancelled successfully",
	})
}

// CreateSubscription creates a new subscription (Customer only)
func CreateSubscription(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var subscription database.Subscription
	if err := c.ShouldBindJSON(&subscription); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	subscription.CustomerID = userID.(uint)
	if err := database.DB.Create(&subscription).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription"})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// DeleteSubscription deletes a subscription (Admin only)
func DeleteSubscription(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	subscriptionID := c.Param("id")
	if err := database.DB.Delete(&database.Subscription{}, subscriptionID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription deleted successfully"})
}
func GetCustomerSubscriptionsByAdmin(c *gin.Context) {
	if c.GetString("role") != database.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	customerIDParam := c.Param("id")
	customerID, err := strconv.ParseUint(customerIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var subscriptions []SubscriptionWithProduct

	err = database.DB.Table("subscriptions").
		Select(`
			subscriptions.id, 
			subscriptions.order_id, 
			subscriptions.customer_id, 
			subscriptions.product_id, 
			subscriptions.franchise_id, 
			subscriptions.status, 
			subscriptions.start_date, 
			subscriptions.end_date, 
			subscriptions.next_billing_date, 
			subscriptions.monthly_rent,
			subscriptions.created_at, 
			subscriptions.updated_at,
			products.name as product_name, 
			products.image_url as product_image,
			franchises.name as franchise_name,
			CASE WHEN subscriptions.status = ? THEN true ELSE false END as is_active,
			subscriptions.next_maintenance as next_service
		`, database.SubscriptionStatusActive).
		Joins("JOIN products ON subscriptions.product_id = products.id").
		Joins("LEFT JOIN franchises ON subscriptions.franchise_id = franchises.id").
		Where("subscriptions.customer_id = ?", customerID).
		Order("subscriptions.created_at DESC").
		Find(&subscriptions).Error

	if err != nil {
		log.Printf("Error fetching customer subscriptions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscriptions"})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}
