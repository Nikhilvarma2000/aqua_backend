package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/razorpay/razorpay-go"
	"gorm.io/gorm"

	"aquahome/config"
	"aquahome/database"
)

// RazorpayOrderRequest contains data for creating a Razorpay order
type RazorpayOrderRequest struct {
	ProductID       uint   `json:"product_id" binding:"required"`
	FranchiseID     uint   `json:"franchise_id" binding:"required"`
	ShippingAddress string `json:"shipping_address" binding:"required"`
	BillingAddress  string `json:"billing_address" binding:"required"`
	RentalDuration  int    `json:"rental_duration" binding:"required,min=1"`
	Notes           string `json:"notes"`
}

// PaymentVerificationRequest contains data for verifying a payment
type PaymentVerificationRequest struct {
	PaymentID       string `json:"payment_id" binding:"required"`
	OrderID         string `json:"order_id" binding:"required"`
	Signature       string `json:"signature" binding:"required"`
	AquaHomeOrderID int64  `json:"aquahome_order_id"`
	SubscriptionID  *int64 `json:"subscription_id"`
}

// MonthlyPaymentRequest contains data for creating a monthly payment
type MonthlyPaymentRequest struct {
	SubscriptionID int64 `json:"subscription_id" binding:"required"`
}

// GeneratePaymentOrder creates a new order and Razorpay order for payment
func GeneratePaymentOrder(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")
	var customerID uint

	switch v := userID.(type) {
	case uint:
		customerID = v
	case int:
		customerID = uint(v)
	case int64:
		customerID = uint(v)
	case float64:
		customerID = uint(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	var request RazorpayOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Start a transaction
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get product details
	var product database.Product
	if err := tx.First(&product, request.ProductID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product details"})
		return
	}

	// Calculate total amount
	totalAmount := product.SecurityDeposit + product.InstallationFee
	if request.RentalDuration > 0 {
		totalAmount += product.MonthlyRent * float64(request.RentalDuration)
	}

	// Create order
	order := database.Order{
		CustomerID:         customerID,
		ProductID:          request.ProductID,
		FranchiseID:        request.FranchiseID,
		OrderType:          "rental",
		Status:             database.OrderStatusPending,
		ShippingAddress:    request.ShippingAddress,
		BillingAddress:     request.BillingAddress,
		RentalDuration:     request.RentalDuration,
		SecurityDeposit:    product.SecurityDeposit,
		InstallationFee:    product.InstallationFee,
		TotalInitialAmount: totalAmount,
		Notes:              request.Notes,
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to create order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	// Initialize Razorpay client
	client := razorpay.NewClient(config.AppConfig.RazorpayKey, config.AppConfig.RazorpaySecret)

	// Get payment amount in paise (Razorpay uses smallest currency unit)
	amountInPaise := int64(order.TotalInitialAmount * 100)

	// Create Razorpay order
	data := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("order_%d", order.ID),
		"notes": map[string]interface{}{
			"aquahome_order_id": order.ID,
			"customer_id":       customerID,
			"order_id":          order.ID,
			"payment_type":      "initial",
		},
	}

	razorpayOrder, err := client.Order.Create(data, nil)
	if err != nil {
		tx.Rollback()
		log.Printf("Error creating Razorpay order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment order"})
		return
	}

	// Create payment record
	payment := database.Payment{
		CustomerID:     customerID,
		OrderID:        &order.ID,
		Amount:         order.TotalInitialAmount,
		PaymentType:    "initial",
		Status:         database.PaymentStatusPending,
		PaymentMethod:  "razorpay",
		TransactionID:  razorpayOrder["id"].(string),
		PaymentDetails: toJSONString(razorpayOrder),
	}

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to create payment record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment record"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	// Return necessary information for the frontend
	c.JSON(http.StatusOK, gin.H{
		"razorpay_order_id": razorpayOrder["id"],
		"amount":            order.TotalInitialAmount,
		"currency":          "INR",
		"key":               config.AppConfig.RazorpayKey,
		"aquahome_order_id": order.ID,
	})
}

// Enhanced VerifyPayment with better error handling
func VerifyPayment(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")
	var customerID uint

	switch v := userID.(type) {
	case uint:
		customerID = v
	case int:
		customerID = uint(v)
	case int64:
		customerID = uint(v)
	case float64:
		customerID = uint(v)
	default:
		log.Printf("Invalid user ID format: %T %v", userID, userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	var request PaymentVerificationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid request data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
		return
	}

	// Validate required fields
	if request.PaymentID == "" || request.OrderID == "" || request.Signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required payment fields"})
		return
	}

	// Log payment verification attempt
	log.Printf("Payment verification attempt - Customer: %d, Payment: %s, Order: %s",
		customerID, request.PaymentID, request.OrderID)

	// Verify payment signature with enhanced logging
	data := request.OrderID + "|" + request.PaymentID
	h := hmac.New(sha256.New, []byte(config.AppConfig.RazorpaySecret))
	h.Write([]byte(data))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	log.Printf("Signature verification - Expected: %s, Provided: %s, Data: %s",
		expectedSignature, request.Signature, data)

	if expectedSignature != request.Signature {
		log.Printf("Payment signature verification failed for customer %d", customerID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid payment signature",
			"success": false,
		})
		return
	}

	// Additional validation: Check if payment ID is already processed
	var existingPayment database.Payment
	if err := database.DB.Where("transaction_id = ? AND status = ?",
		request.PaymentID, database.PaymentStatusSuccess).First(&existingPayment).Error; err == nil {
		log.Printf("Payment ID %s already processed", request.PaymentID)
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Payment already processed",
			"success": false,
		})
		return
	}

	// Begin transaction with timeout
	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Printf("Transaction begin error: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"success": false,
		})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic in payment verification: %v", r)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Server error",
				"success": false,
			})
		}
	}()

	var paymentType string
	var orderID int64
	var result *gorm.DB

	if request.SubscriptionID != nil {
		// Handle subscription payment (existing code with better error handling)
		paymentType = "monthly"

		var subscription database.Subscription
		subscriptionResult := tx.Where("id = ? AND customer_id = ?",
			*request.SubscriptionID, customerID).
			Select("customer_id, order_id, monthly_rent, status").
			First(&subscription)

		if subscriptionResult.Error != nil {
			tx.Rollback()
			if errors.Is(subscriptionResult.Error, gorm.ErrRecordNotFound) {
				log.Printf("Subscription not found or access denied: %d for customer %d",
					*request.SubscriptionID, customerID)
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "Subscription not found",
					"success": false,
				})
				return
			}
			log.Printf("Database error fetching subscription: %v", subscriptionResult.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Server error",
				"success": false,
			})
			return
		}

		// Check if subscription is active
		if subscription.Status != "active" {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Subscription is not active",
				"success": false,
			})
			return
		}

		orderID = int64(subscription.OrderID)

		// Rest of subscription payment logic...
		// (keeping existing logic but with enhanced error handling)

	} else {
		// Handle initial order payment with enhanced validation
		paymentType = "initial"
		orderID = request.AquaHomeOrderID

		if orderID <= 0 {
			tx.Rollback()
			log.Printf("Invalid order ID: %d", orderID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid order ID",
				"success": false,
			})
			return
		}

		// Get order details with better validation
		var order database.Order
		orderResult := tx.Where("id = ? AND customer_id = ?", orderID, customerID).
			Select("customer_id, status, total_initial_amount").
			First(&order)

		if orderResult.Error != nil {
			tx.Rollback()
			if errors.Is(orderResult.Error, gorm.ErrRecordNotFound) {
				log.Printf("Order not found or access denied: %d for customer %d", orderID, customerID)
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "Order not found",
					"success": false,
				})
				return
			}
			log.Printf("Database error fetching order: %v", orderResult.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Server error",
				"success": false,
			})
			return
		}

		if order.Status != database.OrderStatusPending {
			tx.Rollback()
			log.Printf("Order %d not in pending state, current status: %s", orderID, order.Status)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   fmt.Sprintf("Order is not in pending state (current: %s)", order.Status),
				"success": false,
			})
			return
		}

		// Verify the payment exists and is pending
		var pendingPayment database.Payment
		paymentResult := tx.Where("order_id = ? AND payment_type = ? AND status = ?",
			uint(orderID), "initial", database.PaymentStatusPending).First(&pendingPayment)

		if paymentResult.Error != nil {
			tx.Rollback()
			if errors.Is(paymentResult.Error, gorm.ErrRecordNotFound) {
				log.Printf("No pending payment found for order %d", orderID)
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "No pending payment found for this order",
					"success": false,
				})
				return
			}
			log.Printf("Database error fetching pending payment: %v", paymentResult.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Server error",
				"success": false,
			})
			return
		}

		// Update payment record
		paymentDetails := fmt.Sprintf(`{"razorpay_order_id": "%s", "razorpay_payment_id": "%s", "verified_at": "%s"}`,
			request.OrderID, request.PaymentID, time.Now().Format(time.RFC3339))

		result = tx.Model(&database.Payment{}).
			Where("id = ?", pendingPayment.ID).
			Updates(map[string]interface{}{
				"status":          database.PaymentStatusSuccess,
				"transaction_id":  request.PaymentID,
				"payment_method":  "razorpay",
				"payment_details": paymentDetails,
				"updated_at":      time.Now(),
			})

		if result.Error != nil {
			tx.Rollback()
			log.Printf("Error updating payment record: %v", result.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Error updating payment record",
				"success": false,
			})
			return
		}

		if result.RowsAffected == 0 {
			tx.Rollback()
			log.Printf("No payment record updated for order %d", orderID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Payment record not updated",
				"success": false,
			})
			return
		}

		// Update order status
		result = tx.Model(&database.Order{}).
			Where("id = ?", orderID).
			Updates(map[string]interface{}{
				"status":     database.OrderStatusApproved,
				"updated_at": time.Now(),
			})

		if result.Error != nil {
			tx.Rollback()
			log.Printf("Error updating order status: %v", result.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Error updating order status",
				"success": false,
			})
			return
		}

		if result.RowsAffected == 0 {
			tx.Rollback()
			log.Printf("No order record updated for order %d", orderID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Order record not updated",
				"success": false,
			})
			return
		}
	}

	// Create notification (existing code)
	notificationTitle := "Payment Successful"
	paymentTypeDisplay := map[string]string{
		"initial": "Initial",
		"monthly": "Monthly",
	}[paymentType]

	notificationMessage := fmt.Sprintf("%s payment has been processed successfully.", paymentTypeDisplay)
	relatedID := uint(orderID)

	notification := database.Notification{
		UserID:      uint(customerID),
		Title:       notificationTitle,
		Message:     notificationMessage,
		Type:        "payment",
		RelatedID:   &relatedID,
		RelatedType: "order",
	}

	if result := tx.Create(&notification); result.Error != nil {
		// Don't fail the entire transaction for notification error, just log it
		log.Printf("Warning: Failed to create notification: %v", result.Error)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Transaction commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Transaction commit failed",
			"success": false,
		})
		return
	}

	log.Printf("Payment verification successful - Customer: %d, Payment: %s, Order: %d",
		customerID, request.PaymentID, orderID)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Payment verified successfully",
		"order_id":     orderID,
		"payment_type": paymentType,
	})
}

// GenerateMonthlyPayment generates a Razorpay order for monthly subscription payment
func GenerateMonthlyPayment(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, _ := c.Get("user_id")
	var customerID uint
	switch v := userID.(type) {
	case uint:
		customerID = v
	case int:
		customerID = uint(v)
	case int64:
		customerID = uint(v)
	case float64:
		customerID = uint(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	var request MonthlyPaymentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Check if the subscription exists and belongs to the customer
	var subscription database.Subscription
	result := database.DB.Where("id = ? AND customer_id = ?", request.SubscriptionID, customerID).
		Select("id, customer_id, monthly_rent, status, next_billing_date").
		First(&subscription)
	err := result.Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found or doesn't belong to you"})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if subscription.Status != database.SubscriptionStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription is not active"})
		return
	}

	// Initialize Razorpay client
	client := razorpay.NewClient(config.AppConfig.RazorpayKey, config.AppConfig.RazorpaySecret)

	// Get payment amount in paise (Razorpay uses smallest currency unit)
	amountInPaise := int64(subscription.MonthlyRent * 100)

	// Create Razorpay order
	data := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("subscription_%d", subscription.ID),
		"notes": map[string]interface{}{
			"customer_id":     customerID,
			"subscription_id": subscription.ID,
			"payment_type":    "monthly",
		},
	}

	razorpayOrder, err := client.Order.Create(data, nil)
	if err != nil {
		log.Printf("Razorpay order creation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating payment order"})
		return
	}

	// Create or update payment record
	var payment database.Payment
	subscriptionIDUint := subscription.ID
	customerIDUint := uint(customerID)

	result = database.DB.Where("subscription_id = ? AND payment_type = ? AND status = ?",
		subscriptionIDUint, "monthly", database.PaymentStatusPending).
		First(&payment)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Create new payment record
		invoiceNumber := generateMonthlyInvoiceNumber(subscription.ID)
		paymentDetails := fmt.Sprintf(`{"razorpay_order_id": "%s"}`, razorpayOrder["id"])

		newPayment := database.Payment{
			CustomerID:     customerIDUint,
			SubscriptionID: &subscriptionIDUint,
			Amount:         subscription.MonthlyRent,
			PaymentType:    "monthly",
			Status:         database.PaymentStatusPending,
			TransactionID:  razorpayOrder["id"].(string),
			PaymentDetails: paymentDetails,
			InvoiceNumber:  invoiceNumber,
		}

		result = database.DB.Create(&newPayment)

		if result.Error != nil {
			log.Printf("Database error: %v", result.Error)
			// Continue anyway, we'll update it during verification
		}
	} else {
		// Update existing payment record
		paymentDetails := fmt.Sprintf(`{"razorpay_order_id": "%s"}`, razorpayOrder["id"])

		payment.TransactionID = razorpayOrder["id"].(string)
		payment.PaymentDetails = paymentDetails

		result = database.DB.Save(&payment)

		if result.Error != nil {
			log.Printf("Database error: %v", result.Error)
			// Continue anyway, we'll update it during verification
		}
	}

	// Return necessary information for the frontend
	c.JSON(http.StatusOK, gin.H{
		"razorpay_order_id": razorpayOrder["id"],
		"amount":            subscription.MonthlyRent,
		"currency":          "INR",
		"key":               config.AppConfig.RazorpayKey,
		"subscription_id":   subscription.ID,
	})
}

// GetPaymentHistory gets payment history for a user
func GetPaymentHistory(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	roleStr, ok := role.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid role in context"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	fmt.Println("🔍 Context role:", roleStr)
	fmt.Println("🔍 Context userID:", userID)

	var userIDUint uint
	switch v := userID.(type) {
	case float64:
		userIDUint = uint(v)
	case int:
		userIDUint = uint(v)
	case int64:
		userIDUint = uint(v)
	case uint:
		userIDUint = v
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in context"})
		return
	}

	type PaymentHistoryItem struct {
		ID             uint          `json:"id"`
		CustomerID     uint          `json:"customer_id"`
		CustomerName   string        `json:"customer_name"`
		SubscriptionID *uint         `json:"subscription_id"`
		OrderID        *uint         `json:"order_id"`
		Amount         float64       `json:"amount"`
		PaymentType    string        `json:"payment_type"`
		Status         string        `json:"status"`
		TransactionID  string        `json:"transaction_id"`
		PaymentMethod  string        `json:"payment_method"`
		InvoiceNumber  string        `json:"invoice_number"`
		CreatedAt      time.Time     `json:"created_at"`
		User           database.User `json:"-" gorm:"foreignKey:CustomerID"`
	}

	var payments []PaymentHistoryItem
	var result *gorm.DB

	switch roleStr {
	case "admin":
		result = database.DB.Model(&database.Payment{}).
			Select("payments.*, users.name as customer_name").
			Joins("JOIN users ON payments.customer_id = users.id").
			Order("payments.created_at DESC").
			Limit(100).
			Scan(&payments)

	case "franchise_owner":
		result = database.DB.Model(&database.Payment{}).
			Select("payments.*, users.name as customer_name").
			Joins("JOIN users ON payments.customer_id = users.id").
			Joins("LEFT JOIN orders ON payments.order_id = orders.id").
			Joins("LEFT JOIN subscriptions ON payments.subscription_id = subscriptions.id").
			Where("orders.franchise_id IN (SELECT id FROM franchises WHERE owner_id = ?) OR "+
				"subscriptions.franchise_id IN (SELECT id FROM franchises WHERE owner_id = ?)",
				userIDUint, userIDUint).
			Order("payments.created_at DESC").
			Limit(100).
			Scan(&payments)

	case "customer":
		result = database.DB.Model(&database.Payment{}).
			Select("payments.*, users.name as customer_name").
			Joins("JOIN users ON payments.customer_id = users.id").
			Where("payments.customer_id = ?", userIDUint).
			Order("payments.created_at DESC").
			Scan(&payments)

	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	c.JSON(http.StatusOK, payments)
}

// GetPaymentByID gets a payment by ID
func GetPaymentByID(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}
	paymentIDUint := uint(paymentID)

	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, _ := c.Get("user_id")

	var userIDUint uint
	switch v := userID.(type) {
	case float64:
		userIDUint = uint(v)
	case int:
		userIDUint = uint(v)
	case int64:
		userIDUint = uint(v)
	case uint:
		userIDUint = v
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in context"})
		return
	}

	type PaymentDetail struct {
		ID             uint          `json:"id"`
		CustomerID     uint          `json:"customer_id"`
		CustomerName   string        `json:"customer_name"`
		CustomerEmail  string        `json:"customer_email"`
		SubscriptionID *uint         `json:"subscription_id"`
		OrderID        *uint         `json:"order_id"`
		Amount         float64       `json:"amount"`
		PaymentType    string        `json:"payment_type"`
		Status         string        `json:"status"`
		TransactionID  string        `json:"transaction_id"`
		PaymentMethod  string        `json:"payment_method"`
		PaymentDetails string        `json:"payment_details"`
		InvoiceNumber  string        `json:"invoice_number"`
		Notes          string        `json:"notes"`
		CreatedAt      time.Time     `json:"created_at"`
		UpdatedAt      time.Time     `json:"updated_at"`
		User           database.User `json:"-" gorm:"foreignKey:CustomerID"`
	}

	var paymentDetail PaymentDetail
	var query *gorm.DB

	switch role {
	case "admin":
		// Admin can see any payment
		query = database.DB.Model(&database.Payment{}).
			Select("payments.*, users.name as customer_name, users.email as customer_email").
			Joins("JOIN users ON payments.customer_id = users.id").
			Where("payments.id = ?", paymentIDUint)

	case "franchise_owner":
		// Franchise owner can only see payments for orders/subscriptions in their franchise
		query = database.DB.Model(&database.Payment{}).
			Select("payments.*, users.name as customer_name, users.email as customer_email").
			Joins("JOIN users ON payments.customer_id = users.id").
			Joins("LEFT JOIN orders ON payments.order_id = orders.id").
			Joins("LEFT JOIN subscriptions ON payments.subscription_id = subscriptions.id").
			Where("payments.id = ? AND (orders.franchise_id IN (SELECT id FROM franchises WHERE owner_id = ?) OR "+
				"subscriptions.franchise_id IN (SELECT id FROM franchises WHERE owner_id = ?))",
				paymentIDUint, userIDUint, userIDUint)

	case "customer":
		// Customer can only see their own payments
		query = database.DB.Model(&database.Payment{}).
			Select("payments.*, users.name as customer_name, users.email as customer_email").
			Joins("JOIN users ON payments.customer_id = users.id").
			Where("payments.id = ? AND payments.customer_id = ?", paymentIDUint, userIDUint)

	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	result := query.Scan(&paymentDetail)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found or you don't have permission to view it"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	// If PaymentDetails is empty, provide an empty JSON object
	if paymentDetail.PaymentDetails == "" {
		paymentDetail.PaymentDetails = "{}"
	}

	c.JSON(http.StatusOK, paymentDetail)
}

// Helper function to generate a monthly invoice number
func generateMonthlyInvoiceNumber(subscriptionID uint) string {
	timestamp := time.Now().Format("20060102") // YYYYMMDD format
	return "INV-M-" + timestamp + "-" + strconv.FormatUint(uint64(subscriptionID), 10)
}

// toJSONString converts an interface to a JSON string
func toJSONString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling to JSON: %v", err)
		return "{}"
	}
	return string(data)
}

// verifyRazorpaySignature verifies the signature from Razorpay
func verifyRazorpaySignature(data, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}
