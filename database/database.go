package database

import (
	"fmt"
	"log"

	"aquahome/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes the database connection using environment/config
func InitDB() error {
	// Setup logging mode for GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	if config.AppConfig.DBDriver == "postgres" {
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=require TimeZone=UTC",
			config.AppConfig.DBHost,
			config.AppConfig.DBPort,
			config.AppConfig.DBUser,
			config.AppConfig.DBPassword,
			config.AppConfig.DBName,
		)

		log.Printf("🔌 Connecting to PostgreSQL at host=%s port=%s db=%s...",
			config.AppConfig.DBHost,
			config.AppConfig.DBPort,
			config.AppConfig.DBName,
		)

		var err error
		DB, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			log.Printf("❌ Failed to connect to DB: %v", err)
			return err
		}

		log.Println("✅ PostgreSQL connection successful.")
		return nil
	}

	log.Println("❌ Unsupported DB driver:", config.AppConfig.DBDriver)
	return fmt.Errorf("unsupported DB driver: %s", config.AppConfig.DBDriver)
}
