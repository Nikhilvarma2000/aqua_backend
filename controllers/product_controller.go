package controllers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
)

// ProductRequest contains the data for product creation or update
type ProductRequest struct {
	Name             string  `json:"name" binding:"required"`
	Description      string  `json:"description" binding:"required"`
	ImageURL         string  `json:"image_url"`
	MonthlyRent      float64 `json:"monthly_rent" binding:"required"`
	SecurityDeposit  float64 `json:"security_deposit" binding:"required"`
	InstallationFee  float64 `json:"installation_fee" binding:"required"`
	AvailableStock   int     `json:"available_stock" binding:"required"`
	Specifications   string  `json:"specifications"`
	MaintenanceCycle int     `json:"maintenance_cycle"`
	IsActive         bool    `json:"is_active"`
	FranchiseID      uint    `json:"franchise_id" binding:"required"` // ✅ Add this
}

// CreateProduct creates a new product (Admin only)
func CreateProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	var productRequest ProductRequest
	if err := c.ShouldBindJSON(&productRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	if productRequest.MaintenanceCycle == 0 {
		productRequest.MaintenanceCycle = 90 // Default 90 days
	}

	// Validate that the FranchiseID exists in the system
	var franchise database.Franchise
	if err := database.DB.First(&franchise, productRequest.FranchiseID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Franchise ID"})
		return
	}

	product := database.Product{
		Name:             productRequest.Name,
		Description:      productRequest.Description,
		ImageURL:         productRequest.ImageURL,
		MonthlyRent:      productRequest.MonthlyRent,
		SecurityDeposit:  productRequest.SecurityDeposit,
		InstallationFee:  productRequest.InstallationFee,
		AvailableStock:   productRequest.AvailableStock,
		Specifications:   productRequest.Specifications,
		MaintenanceCycle: productRequest.MaintenanceCycle,
		IsActive:         productRequest.IsActive,
		FranchiseID:      productRequest.FranchiseID, // ✅ Important
	}

	result := database.DB.Create(&product)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// GetProducts gets all products (admin sees all, customer/public sees all but can only order active ones)
func GetProducts(c *gin.Context) {
	var products []database.Product

	query := database.DB.Preload("Franchise") // 👈 preload franchise

	roleInterface, exists := c.Get("role")
	if exists {
		role := roleInterface.(string)
		if role == "customer" {
			query = query.Where("is_active = ?", true)
		}
	}

	if err := query.Find(&products).Error; err != nil {
		log.Println("GetProducts DB error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get products"})
		return
	}

	c.JSON(http.StatusOK, products)
}

// GetProductByID gets a product by ID
func GetProductByID(c *gin.Context) {
	id := c.Param("id")
	var product database.Product

	if err := database.DB.Preload("Franchise").First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
		}
		return
	}

	roleInterface, _ := c.Get("role")
	if role, ok := roleInterface.(string); ok && role == "customer" && !product.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Product not available"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// UpdateProduct updates a product (Admin only)
func UpdateProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	productIDStr := c.Param("id")
	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var productRequest ProductRequest
	if err := c.ShouldBindJSON(&productRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	var product database.Product
	result := database.DB.First(&product, uint(productID))
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	product.Name = productRequest.Name
	product.Description = productRequest.Description
	product.ImageURL = productRequest.ImageURL
	product.MonthlyRent = productRequest.MonthlyRent
	product.SecurityDeposit = productRequest.SecurityDeposit
	product.InstallationFee = productRequest.InstallationFee
	product.AvailableStock = productRequest.AvailableStock
	product.Specifications = productRequest.Specifications
	product.MaintenanceCycle = productRequest.MaintenanceCycle
	product.IsActive = productRequest.IsActive
	product.FranchiseID = productRequest.FranchiseID //  Also update

	result = database.DB.Save(&product)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating product"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// DeleteProduct permanently deletes a product (Admin only)
func DeleteProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	productIDStr := c.Param("id")
	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product database.Product
	result := database.DB.First(&product, uint(productID))
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	result = database.DB.Delete(&product)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted permanently"})
}

// ToggleProductStatus toggles the IsActive status of a product (Admin only)
func ToggleProductStatus(c *gin.Context) {
	id := c.Param("id")
	var product database.Product

	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var body struct {
		IsActive bool `json:"isActive"` // ✅ MATCHES frontend key
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	log.Println("Received toggle status:", body.IsActive)
	product.IsActive = body.IsActive
	if err := database.DB.Save(&product).Error; err != nil {
		log.Println("Save failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product status"})
		return
	}
	c.JSON(http.StatusOK, product)
}
func GetCustomerProducts(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
		return
	}

	customer := user.(database.User)
	if customer.ZipCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ZIP code is required"})
		return
	}

	var products []database.Product
	err := database.DB.
		Preload("Franchise").
		Joins("JOIN franchises ON franchises.id = products.franchise_id").
		Where("products.is_active = ? AND franchises.is_active = ? AND franchises.zip_code = ?", true, true, customer.ZipCode).
		Find(&products).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	c.JSON(http.StatusOK, products)
}
