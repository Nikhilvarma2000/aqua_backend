package controllers

import (
	"net/http"
	"strings"

	"aquahome/database"

	"github.com/gin-gonic/gin"
)

// AdminDashboard returns key statistics for the admin dashboard
func AdminDashboard(c *gin.Context) {
	var totalCustomers int64
	var totalOrders int64

	// Count customers with role 'customer'
	if err := database.DB.Model(&database.User{}).Where("role = ?", "customer").Count(&totalCustomers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count customers"})
		return
	}

	// Count total orders
	if err := database.DB.Model(&database.Order{}).Count(&totalOrders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count orders"})
		return
	}

	// Return simplified dashboard data
	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"totalCustomers":         totalCustomers,
			"totalOrders":            totalOrders,
			"totalRevenue":           0, // Optional: implement if needed
			"activeSubscriptions":    0,
			"pendingServiceRequests": 0,
			"franchiseApplications":  0,
		},
	})
}

// AdminGetOrders returns all orders with related data
func AdminGetOrders(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	user := userID.(uint)

	// For franchise owners, get orders based on their service areas
	if role == "franchise_owner" {
		var franchise database.Franchise
		if err := database.DB.Where("owner_id = ?", user).First(&franchise).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch franchise"})
			return
		}

		// Get all ZIP codes served by this franchise
		var zipCodesArray []string
		if err := database.DB.Table("franchise_locations").
			Joins("JOIN locations ON franchise_locations.location_id = locations.id").
			Where("franchise_locations.franchise_id = ?", franchise.ID).
			Pluck("locations.zip_codes", &zipCodesArray).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ZIP codes"})
			return
		}

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

		// Get users in these zip codes
		var users []database.User
		if err := database.DB.Where("zip_code IN ?", zipCodes).
			Where("role = ?", "customer").
			Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		// Extract user IDs
		var userIDs []uint
		for _, u := range users {
			userIDs = append(userIDs, u.ID)
		}

		// Get orders for these users
		var orders []database.Order
		if err := database.DB.Preload("Customer").
			Preload("Product").
			Preload("Franchise").
			Where("customer_id IN ?", userIDs).
			Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		c.JSON(http.StatusOK, orders)
		return
	}

	// For admin, get all orders
	var orders []database.Order
	if err := database.DB.Preload("Customer").
		Preload("Franchise").
		Preload("Product").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}
