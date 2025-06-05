package main

import (
	"log"
	"os" // Import os for directory checks

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"aquahome/config"
	"aquahome/controllers" // Add controllers to directly define a public route
	"aquahome/database"
	"aquahome/routes" // Keep this for existing route setup
)

func main() {
	_ = godotenv.Load()
	config.InitConfig()

	if err := database.InitDB(); err != nil {
		log.Fatalf("❌ Failed to initialize GORM database: %v", err)
	}

	if err := database.DB.AutoMigrate(
		&database.User{},
		&database.Franchise{},
		&database.Order{},
		&database.Subscription{},
		&database.ServiceRequest{},
		&database.Payment{},
		&database.Notification{},
		&database.Location{},
		&database.FranchiseLocation{},
	); err != nil {
		log.Fatalf("❌ AutoMigrate failed: %v", err)
	}

	log.Println("✅ Database migration skipped (commented out in main.go)")
	database.SeedDefaultAdmin()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 🆕 START: ADD THESE LINES FOR STATIC FILE SERVING
	// This makes files in ./uploads accessible via /uploads/*
	r.Static("/uploads", "./uploads")
	log.Println("Serving static files from /uploads to ./uploads directory")

	// Ensure the 'uploads/products' directory exists
	// This will prevent errors if the directory is missing when saving files.
	if _, err := os.Stat("./uploads/products"); os.IsNotExist(err) {
		err := os.MkdirAll("./uploads/products", 0755) // 0755 permissions
		if err != nil {
			log.Fatalf("Failed to create uploads/products directory: %v", err)
		}
		log.Println("Created ./uploads/products directory")
	}
	// 🆕 END: ADD THESE LINES FOR STATIC FILE SERVING

	// 🆕 START: Public routes that do NOT require authentication
	// Move GetCustomerProducts here if it should be accessible without logging in
	// If it *requires* a logged-in customer, keep it within an authenticated group (not admin-specific)
	r.GET("/api/products", controllers.GetCustomerProducts) //
	// 🆕 END: Public routes

	// Setup all other API routes using your existing routes.SetupRoutes function
	routes.SetupRoutes(r) //

	for _, route := range r.Routes() {
		log.Printf("🔗 %s %s", route.Method, route.Path)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	log.Printf("🚀 Server running at http://0.0.0.0:%s", port)

	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
