package controllers

import (
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath" //  ADD THIS IMPORT
	"strconv"
	"time" //  ADD THIS IMPORT, often needed for unique filenames

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"aquahome/database"
)

// 🆕 MODIFIED: ProductRequest to handle file upload instead of direct ImageURL
type ProductRequest struct {
	Name             string  `form:"name" binding:"required"` // 🆕 Changed to `form` tag
	Description      string  `form:"description" binding:"required"`
	MonthlyRent      float64 `form:"monthly_rent" binding:"required"`
	SecurityDeposit  float64 `form:"security_deposit" binding:"required"`
	InstallationFee  float64 `form:"installation_fee" binding:"required"`
	AvailableStock   int     `form:"available_stock" binding:"required"`
	Specifications   string  `form:"specifications"`
	MaintenanceCycle int     `form:"maintenance_cycle"`
	IsActive         bool    `form:"is_active"`
	FranchiseID      uint    `form:"franchise_id" binding:"required"`
	// ImageURL         string  `json:"image_url"` // ❌ REMOVE THIS LINE
	// 🆕 ADD THIS FIELD to receive the uploaded file
	ImageFile *multipart.FileHeader `form:"image_file"` //  Correct type
}

// CreateProduct creates a new product (Admin only)
func CreateProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	var request ProductRequest
	// 🆕 Use c.ShouldBind to parse multipart form data
	if err := c.ShouldBind(&request); err != nil { // 🆕 Changed from ShouldBindJSON
		log.Println("Product creation bind error:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle image upload
	var imageURL string
	if request.ImageFile != nil {
		// Define upload directory
		uploadDir := "./uploads/products" // 🆕 Ensure this directory exists relative to your executable
		// Create a unique filename
		filename := strconv.FormatInt(time.Now().UnixNano(), 10) + filepath.Ext(request.ImageFile.Filename)
		filePath := filepath.Join(uploadDir, filename)

		// Save the file
		if err := c.SaveUploadedFile(request.ImageFile, filePath); err != nil {
			log.Println("Failed to save image:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image file"})
			return
		}
		imageURL = "/uploads/products/" + filename // Store relative URL for frontend access
	}

	product := database.Product{
		Name:             request.Name,
		Description:      request.Description,
		MonthlyRent:      request.MonthlyRent,
		SecurityDeposit:  request.SecurityDeposit,
		InstallationFee:  request.InstallationFee,
		AvailableStock:   request.AvailableStock,
		Specifications:   request.Specifications,
		MaintenanceCycle: request.MaintenanceCycle,
		IsActive:         request.IsActive,
		FranchiseID:      request.FranchiseID,
		ImageURL:         imageURL, // 🆕 Save the generated image URL
	}

	if err := database.DB.Create(&product).Error; err != nil {
		log.Println("Product creation DB error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// GetProduct retrieves a product by ID
func GetProduct(c *gin.Context) {
	id := c.Param("id")
	var product database.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve product"})
		}
		return
	}
	c.JSON(http.StatusOK, product)
}

// GetAllProducts retrieves all products (Admin only)
func GetAllProducts(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	var products []database.Product
	if err := database.DB.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve products"})
		return
	}
	c.JSON(http.StatusOK, products)
}

// UpdateProduct updates an existing product (Admin only)
func UpdateProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	id := c.Param("id")
	var product database.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var request ProductRequest
	// 🆕 Use c.ShouldBind to parse multipart form data
	if err := c.ShouldBind(&request); err != nil { // 🆕 Changed from ShouldBindJSON
		log.Println("Product update bind error:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle image upload for update
	var imageURL string
	if request.ImageFile != nil {
		uploadDir := "./uploads/products"
		filename := strconv.FormatInt(time.Now().UnixNano(), 10) + filepath.Ext(request.ImageFile.Filename)
		filePath := filepath.Join(uploadDir, filename)

		if err := c.SaveUploadedFile(request.ImageFile, filePath); err != nil {
			log.Println("Failed to save updated image:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save updated image file"})
			return
		}
		imageURL = "/uploads/products/" + filename
	} else {
		// If no new image is uploaded, retain the existing one
		imageURL = product.ImageURL
	}

	// Update product fields from request
	product.Name = request.Name
	product.Description = request.Description
	product.MonthlyRent = request.MonthlyRent
	product.SecurityDeposit = request.SecurityDeposit
	product.InstallationFee = request.InstallationFee
	product.AvailableStock = request.AvailableStock
	product.Specifications = request.Specifications
	product.MaintenanceCycle = request.MaintenanceCycle
	product.IsActive = request.IsActive
	product.FranchiseID = request.FranchiseID
	product.ImageURL = imageURL // 🆕 Update with new or existing image URL

	if err := database.DB.Save(&product).Error; err != nil {
		log.Println("Product update DB error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// DeleteProduct deletes a product by ID (Admin only)
func DeleteProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	id := c.Param("id")
	var product database.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	if err := database.DB.Delete(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
}

// ToggleProductStatus toggles the active status of a product (Admin only)
func ToggleProductStatus(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

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
		Where("is_active = ?", true).
		Joins("JOIN franchises ON products.franchise_id = franchises.id").
		Where("franchises.zip_code = ?", customer.ZipCode).
		Find(&products).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve products for customer's ZIP code"})
		return
	}

	c.JSON(http.StatusOK, products)
}
