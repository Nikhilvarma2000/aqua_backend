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

// OrderRequest contains the data for order creation
type OrderRequest struct {
	ProductID       int64  `json:"product_id" binding:"required"`
	FranchiseID     int64  `json:"franchise_id" binding:"required"`
	ShippingAddress string `json:"shipping_address" binding:"required"`
	BillingAddress  string `json:"billing_address" binding:"required"`
	RentalDuration  int    `json:"rental_duration" binding:"required,min=1"`
	Notes           string `json:"notes"`
}

// CreateOrder creates a new order (Customer only)
func CreateOrder(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userIDUint, ok := userIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}
	customerID := uint64(userIDUint) // Use this below for storing order

	var orderRequest OrderRequest
	if err := c.ShouldBindJSON(&orderRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	fmt.Printf(" Received Payload: %+v\n", orderRequest)

	fmt.Println("Incoming Product ID:", orderRequest.ProductID)
	fmt.Println("Incoming Franchise ID:", orderRequest.FranchiseID)

	// Get product details
	var product database.Product
	result := database.DB.First(&product, orderRequest.ProductID)
	err := result.Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if !product.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product is not available"})
		return
	}

	// Verify franchise exists and is active
	var franchise database.Franchise
	franchiseResult := database.DB.First(&franchise, orderRequest.FranchiseID)
	err = franchiseResult.Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if !franchise.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise is not active"})
		return
	}

	// Calculate total initial amount
	totalInitialAmount := product.SecurityDeposit + product.InstallationFee + product.MonthlyRent

	// Begin transaction
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Create order
	franchiseIDUint := uint(orderRequest.FranchiseID)
	order := database.Order{
		CustomerID:         uint(customerID),
		ProductID:          uint(orderRequest.ProductID),
		FranchiseID:        franchiseIDUint,
		OrderType:          "rental",
		Status:             database.OrderStatusPending,
		ShippingAddress:    orderRequest.ShippingAddress,
		BillingAddress:     orderRequest.BillingAddress,
		RentalStartDate:    time.Now(), // rental_start_date will be confirmed after approval
		RentalDuration:     orderRequest.RentalDuration,
		MonthlyRent:        product.MonthlyRent,
		SecurityDeposit:    product.SecurityDeposit,
		InstallationFee:    product.InstallationFee,
		TotalInitialAmount: totalInitialAmount,
		Notes:              orderRequest.Notes,
	}

	result = tx.Create(&order)
	if result.Error != nil {
		if err := tx.Rollback().Error; err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating order"})
		return
	}

	orderID := int64(order.ID)

	// Create pending payment
	invoiceNumber := generateInvoiceNumber(orderID)

	orderIDUint := uint(orderID)
	payment := database.Payment{
		CustomerID:    uint(customerID),
		OrderID:       &orderIDUint,
		Amount:        totalInitialAmount,
		PaymentType:   "initial",
		Status:        database.PaymentStatusPending,
		InvoiceNumber: invoiceNumber,
		Notes:         "Initial payment for order",
	}

	result = tx.Create(&payment)
	if result.Error != nil {
		if err := tx.Rollback().Error; err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating payment"})
		return
	}

	// Create notification for customer
	relatedID := uint(orderID)
	notification := database.Notification{
		UserID:      uint(customerID),
		Title:       "Order Placed Successfully",
		Message:     "Your order for " + product.Name + " has been placed and is pending approval.",
		Type:        "order",
		RelatedID:   &relatedID,
		RelatedType: "order",
	}

	result = tx.Create(&notification)
	if result.Error != nil {
		if err := tx.Rollback().Error; err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating notification"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// Get the created order
	var createdOrder database.Order
	result = database.DB.First(&createdOrder, orderID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving order"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Order created successfully",
		"order":          createdOrder,
		"invoice_number": invoiceNumber,
	})
}

func CancelOrder(c *gin.Context) {
	fmt.Println(" CancelOrder hit!")

	role, exists := c.Get("role")
	fmt.Println("Role:", role)
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	if role != "customer" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var order database.Order
	if err := database.DB.First(&order, orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// FIX: correct key and strict validation
	userIDInterface, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}
	fmt.Println("userID: ", userID)

	// if order.CustomerID != userID && role != "admin" {
	// 	c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to cancel this order"})
	// 	return
	// }

	order.Status = database.OrderStatusCancelled

	if err := database.DB.Save(&order).Error; err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order cancelled successfully"})
}

// GetCustomerOrders gets orders for the authenticated customer
func GetCustomerOrders(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")
	fmt.Printf("userID: %+v\n", userID)

	var customerID uint
	if id, ok := userID.(uint); ok {
		customerID = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	type OrderWithProduct struct {
		ID           uint       `json:"id"`
		Status       string     `json:"status"`
		CreatedAt    time.Time  `json:"created_at"`
		TotalAmount  float64    `json:"total_amount"`
		DeliveryDate *time.Time `json:"delivery_date"`
		ProductName  string     `json:"product_name"`
		ProductImage string     `json:"product_image"`
	}

	var orders []OrderWithProduct

	// Use GORM's joins and select capabilities to get orders with product info
	result := database.DB.Table("orders").
		Select(`orders.id as id, 
          orders.status, 
          orders.created_at, 
          orders.delivery_date, 
          orders.total_initial_amount as total_amount, 
          products.name as product_name, 
          products.image_url as product_image`).
		Joins("JOIN products ON orders.product_id = products.id").
		Where("orders.customer_id = ?", customerID).
		Order("orders.created_at DESC").
		Find(&orders)

	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, orders)
}
func GetAllOrders(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || (role != "admin" && role != "franchise_owner") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID := c.MustGet("user_id").(uint)

	var orders []database.Order
	var result *gorm.DB

	if role == "admin" {
		// Admin sees all orders
		result = database.DB.Preload("Product").Order("created_at DESC").Find(&orders)
	} else if role == "franchise_owner" {
		// Franchise owner sees only their franchise's orders
		var user database.User
		if err := database.DB.First(&user, userID).Error; err != nil || user.FranchiseID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Franchise not linked to your account"})
			return
		}
		result = database.DB.
			Where("franchise_id = ?", *user.FranchiseID).
			Preload("Product").
			Order("created_at DESC").
			Find(&orders)
	}

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	// Optional: Format as response struct if needed
	type AdminOrderResponse struct {
		ID              uint             `json:"id"`
		Status          string           `json:"status"`
		OrderType       string           `json:"order_type"`
		FranchiseID     uint             `json:"franchise_id"`
		ShippingAddress string           `json:"shipping_address"`
		CreatedAt       time.Time        `json:"created_at"`
		TotalAmount     float64          `json:"total_amount"`
		Product         database.Product `json:"product"`
	}

	var response []AdminOrderResponse
	for _, o := range orders {
		response = append(response, AdminOrderResponse{
			ID:              o.ID,
			Status:          o.Status,
			OrderType:       o.OrderType,
			FranchiseID:     o.FranchiseID,
			ShippingAddress: o.ShippingAddress,
			CreatedAt:       o.CreatedAt,
			TotalAmount:     o.TotalInitialAmount,
			Product:         o.Product,
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetOrderByID gets an order by ID
func GetOrderByID(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Get user role and ID
	role, _ := c.Get("role")
	userID, _ := c.Get("user_id")

	var userIDInt uint

	if id, ok := userID.(uint); ok {
		userIDInt = id
	} else {
		log.Printf("Failed to convert user_id to uint: %v", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Define order detail struct with joined fields
	type OrderDetail struct {
		database.Order
		ProductName       string `json:"product_name"`
		ProductImage      string `json:"product_image"`
		CustomerName      string `json:"customer_name"`
		CustomerEmail     string `json:"customer_email"`
		CustomerPhone     string `json:"customer_phone"`
		ServiceAgentName  string `json:"service_agent_name"`
		ServiceAgentPhone string `json:"service_agent_phone"`
	}

	// Start building the query with GORM
	var orderDetail OrderDetail

	// Base query with joins
	query := database.DB.Table("orders").
		Select("orders.*, products.name as product_name, products.image_url as product_image, users.name as customer_name, users.email as customer_email, users.phone as customer_phone").
		Joins("JOIN products ON orders.product_id = products.id").
		Joins("JOIN users ON orders.customer_id = users.id").
		Where("orders.id = ?", orderID)

	// Add role-specific conditions
	switch role {
	case "admin":
		// Admin can view any order, no additional conditions needed
	case "franchise_owner":
		// Franchise owner can only view orders for their franchise
		// query = query.Joins("JOIN franchises ON orders.franchise_id = franchises.id").
		// 	Where("franchises.owner_id = ?", userIDInt)
	case "service_agent":
		// Service agent can only view orders assigned to them
		query = query.Where("orders.service_agent_id = ?", userIDInt)
	case "customer":
		// Customer can only view their own orders
		query = query.Where("orders.customer_id = ?", userIDInt)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Execute the query
	result := query.First(&orderDetail)

	// adding service agent details if orderid has serviceagentid
	if orderDetail.ServiceAgentID != nil {
		var serviceAgent database.User
		if err := database.DB.First(&serviceAgent, *orderDetail.ServiceAgentID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service agent details"})
			
		}
		orderDetail.ServiceAgentName = serviceAgent.Name
		orderDetail.ServiceAgentPhone = serviceAgent.Phone
	}

	fmt.Println("Result:", result, "\nOrder Detail:", orderDetail)
	err = result.Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or you don't have permission to view it"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, orderDetail)
}

// UpdateOrderStatusRequest contains data for updating an order status
type UpdateOrderStatusRequest struct {
	Status         string `json:"status" binding:"required"`
	ServiceAgentID *int64 `json:"service_agent_id"`
	Notes          string `json:"notes"`
}

// UpdateOrderStatus updates an order status (Admin or Franchise Owner only)
func UpdateOrderStatus(c *gin.Context) {
	role, exists := c.Get("role")
	fmt.Println("Role:", role)
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}
	fmt.Println("✅ Role check passed")

	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	fmt.Println("✅ Order ID parsed successfully")
	var statusRequest UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&statusRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	fmt.Println("✅ Request data parsed successfully")
	if role == "service_agent" && statusRequest.Status == "cancelled" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Check if order exists and get current status
	var currentStatus string
	var franchiseID int64
	var customerID int64
	var productID int64

	var order database.Order
	err = database.DB.Where("id = ?", orderID).
		Select("status, franchise_id, customer_id, product_id").
		First(&order).Error
	if err == nil {
		currentStatus = order.Status
		franchiseID = int64(order.FranchiseID)
		customerID = int64(order.CustomerID)
		productID = int64(order.ProductID)
	}
	fmt.Println("✅ Order details retrieved successfully ", orderID, franchiseID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// If franchise owner, check if they own the franchise
	if role == "franchise_owner" {
		userID, _ := c.Get("user_id")

		var user struct {
			FranchiseID uint
		}
		err := database.DB.Table("users").
			Select("franchise_id").
			Where("id = ? AND role = ?", userID, "franchise_owner").
			Scan(&user).Error
		if err != nil {
			log.Printf("Database error fetching franchise_id: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}
	
		// Step 2: Use franchise_id to get franchise details
		var franchise database.Franchise
		err = database.DB.Where("id = ?", user.FranchiseID).First(&franchise).Error
		if err != nil {
			log.Printf("Database error fetching franchise: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}
	
		fmt.Println("✅ Franchise ID retrieved successfully", franchise.ID)


	
		if err != nil {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
			return
		}
		ownerID := uint(franchise.OwnerID)
		fmt.Println("✅ Owner ID retrieved successfully", ownerID)
		if ownerID != userID.(uint) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this order"})
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

	// First get the order in the transaction
	// Already have the order variable from earlier, reuse it
	if err := tx.First(&order, orderID).Error; err != nil {
		if err := tx.Rollback().Error; err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error finding order"})
		return
	}

	// Update order status
	order.Status = statusRequest.Status

	// Only update serviceAgentID if provided
	if statusRequest.ServiceAgentID != nil && *statusRequest.ServiceAgentID > 0 {
		agentID := uint(*statusRequest.ServiceAgentID)
		order.ServiceAgentID = &agentID
	}

	// Append notes if provided
	if statusRequest.Notes != "" {
		if order.Notes != "" {
			order.Notes = order.Notes + " | " + statusRequest.Notes
		} else {
			order.Notes = statusRequest.Notes
		}
	}

	if err := tx.Save(&order).Error; err != nil {
		if err := tx.Rollback().Error; err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating order status"})
		return
	}

	// If status changed to "approved", create subscription
	if statusRequest.Status == database.OrderStatusDelivered && currentStatus != database.OrderStatusDelivered {
		// We already have the order from earlier, but we need to reload to get all fields
		if err := tx.First(&order, orderID).Error; err != nil {
			if err := tx.Rollback().Error; err != nil {
				log.Printf("Failed to rollback transaction: %v", err)
			}
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving order details"})
			return
		}

		// Calculate end date and next billing date
		startDate := time.Now() // Use current time as actual start date
		endDate := startDate.AddDate(0, order.RentalDuration, 0)
		nextBillingDate := startDate.AddDate(0, 1, 0) // Next month

		// Create subscription with GORM
		subscription := database.Subscription{
			OrderID:          uint(orderID),
			CustomerID:       uint(customerID),
			ProductID:        uint(productID),
			FranchiseID:      uint(franchiseID),
			Status:           database.SubscriptionStatusActive,
			StartDate:        startDate,
			EndDate:          endDate,
			NextBillingDate:  nextBillingDate,
			MonthlyRent:      order.MonthlyRent,
			LastMaintenance:  time.Time{},                // Zero value
			NextMaintenance:  startDate.AddDate(0, 3, 0), // 3 months after start
			MaintenanceNotes: "Initial setup complete",
			Notes:            "Created from order #" + strconv.FormatInt(orderID, 10),
		}

		if err := tx.Create(&subscription).Error; err != nil {
			if err := tx.Rollback().Error; err != nil {
				log.Printf("Failed to rollback transaction: %v", err)
			}
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating subscription"})
			return
		}

		// Update order's rental start date to actual start date
		order.RentalStartDate = startDate
		if err := tx.Save(&order).Error; err != nil {
			if err := tx.Rollback().Error; err != nil {
				log.Printf("Failed to rollback transaction: %v", err)
			}
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating order start date"})
			return
		}
	}

	// Create notification for customer
	var message string
	switch statusRequest.Status {
	case database.OrderStatusApproved:
		message = "Your order has been approved. Your subscription is now active."
	case database.OrderStatusRejected:
		message = "Your order has been rejected. Please contact customer support for details."
	case database.OrderStatusCancelled:
		message = "Your order has been cancelled."
	case database.OrderStatusInTransit:
		message = "Your order is in transit and will be delivered soon."
	case database.OrderStatusDelivered:
		message = "Your order has been delivered. Installation will be scheduled soon."
	case database.OrderStatusInstalled:
		message = "Your water purifier has been successfully installed."
	default:
		message = "Your order status has been updated to " + statusRequest.Status
	}

	// Create notification using GORM
	relatedIDUint := uint(orderID)
	notification := database.Notification{
		UserID:      uint(customerID),
		Title:       "Order Status Updated",
		Message:     message,
		Type:        "order",
		RelatedID:   &relatedIDUint,
		RelatedType: "order",
	}

	if err := tx.Create(&notification).Error; err != nil {
		if err := tx.Rollback().Error; err != nil {
			log.Printf("Failed to rollback transaction: %v", err)
		}
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

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated successfully"})
}

// AssignOrderRequest represents the payload for assigning a franchise
type AssignOrderRequest struct {
	FranchiseID uint `json:"franchise_id" binding:"required"`
}

// AssignOrderToFranchise allows admin to assign a franchise to an order
func AssignOrderToFranchise(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req AssignOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	var order database.Order
	if err := database.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	order.FranchiseID = req.FranchiseID

	if err := database.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign franchise"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Franchise assigned", "order": order})
}

// Helper function to generate an invoice number
func generateInvoiceNumber(orderID int64) string {
	timestamp := time.Now().Format("20060102") // YYYYMMDD format
	return "INV-" + timestamp + "-" + strconv.FormatInt(orderID, 10)
}

// AssignOrderToAgent allows admin to assign a service agent to an order
func AssignOrderToAgent(c *gin.Context) {
	fmt.Println(" AssignOrderToAgent route hit!")

	role, _ := c.Get("role")
	if role != "admin" && role != "franchise_owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req struct {
		ServiceAgentID uint `json:"service_agent_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Update order with service agent ID
	if err := database.DB.Model(&database.Order{}).
		Where("id = ?", orderID).
		Update("service_agent_id", req.ServiceAgentID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign service agent"})
		return
	}

	// Reload the full order with related data
	var order database.Order
	// Use orderID directly here instead of order.ID
	// Use the incoming `orderID` directly, not `order.ID`
	if err := database.DB.
		Preload("Customer").
		Preload("Product").
		Preload("Franchise.Owner").
		Preload("ServiceAgent").
		First(&order, orderID).Error; err != nil {
		log.Printf("Failed to reload order with associations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load full order details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order assigned to service agent successfully",
		"order":   order,
	})
}
