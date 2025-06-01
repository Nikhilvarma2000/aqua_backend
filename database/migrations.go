package database

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// RunMigrations runs all database migrations
func RunMigrations() error {
	log.Println("Running database migrations...")

	// AutoMigrate will create tables if they don't exist
	if err := DB.AutoMigrate(
		&User{},
		&Product{},
		&Franchise{},
		&Location{},          // ✅ Service ZIPs
		&FranchiseLocation{}, // ✅ Join table for Franchise ↔ Location
		&Order{},
		&Subscription{},
		&ServiceRequest{},
		&Payment{},
		&Notification{},
		&PasswordReset{},
		&Audit{},
		&AuditLog{},
	); err != nil {
		log.Printf("Migration failed: %v", err)
		return err
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// SeedDefaultAdmin creates a default admin if none exists
func SeedDefaultAdmin() {
	var count int64
	if err := DB.Model(&User{}).Where("role = ?", RoleAdmin).Count(&count).Error; err != nil {
		log.Printf("❌ Failed to check existing admin: %v", err)
		return
	}

	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("❌ Failed to hash admin password: %v", err)
			return
		}

		admin := User{
			Name:         "Super Admin",
			Email:        "admin@aquahome.com",
			PasswordHash: string(hash),
			Role:         RoleAdmin,
			Phone:        "9999999999",
			Address:      "Admin HQ",
			City:         "Hyderabad",
			State:        "Telangana",
			ZipCode:      "500001",
		}

		if err := DB.Create(&admin).Error; err != nil {
			log.Printf("❌ Failed to create admin: %v", err)
		} else {
			log.Println("✅ Default admin user created successfully.")
		}
	} else {
		log.Println("ℹ️ Admin user already exists.")
	}
}
