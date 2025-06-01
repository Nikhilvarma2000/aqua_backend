package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"aquahome/config"
	"aquahome/database"
	"aquahome/routes"
)

func main() {

	// 🔐 TEMP: Print hash for admin password
	// hashed, err := utils.HashPassword("admin123")
	// if err != nil {
	// 	log.Fatalf("❌ Failed to hash password: %v", err)
	// }
	// fmt.Println("🔐 Hashed password for 'admin123':", hashed)
	// os.Exit(0) // Exit here to stop rest of the app
	// Load environment variables (optional for local dev)
	_ = godotenv.Load()

	// Initialize config
	config.InitConfig()

	// Initialize PostgreSQL DB
	if err := database.InitDB(); err != nil {
		log.Fatalf("❌ Failed to initialize GORM database: %v", err)
	}

	// Auto-migrate all models (commented out to prevent auto-migration)

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

	// ✅ Seed default admin if not exists
	database.SeedDefaultAdmin()

	// // (Optional) Initialize any legacy DB (only if needed)
	// if err := database.InitLegacyDB(); err != nil {
	// 	log.Fatalf("❌ Failed to initialize legacy database: %v", err)
	// }

	// Setup Gin router
	r := gin.Default()

	// Enable CORS for all origins
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Setup all API routes
	routes.SetupRoutes(r)

	// Print all registered routes
	for _, route := range r.Routes() {
		log.Printf("🔗 %s %s", route.Method, route.Path)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000" // default port for local
	}
	log.Printf("🚀 Server running at http://0.0.0.0:%s", port)

	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
