package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"aquahome/config"
)

// LegacyDB is the global raw SQL DB instance
var LegacyDB *sql.DB

// InitLegacyDB connects to the legacy DB
func InitLegacyDB() error {
	var err error

	switch config.AppConfig.DBDriver {
	case "postgres":
		dbURL := os.Getenv("DATABASE_URL")
		var connStr string

		if dbURL != "" {
			connStr = dbURL
			log.Println("Using DATABASE_URL for PostgreSQL legacy DB")
		} else {
			connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
				config.AppConfig.DBHost,
				config.AppConfig.DBPort,
				config.AppConfig.DBUser,
				config.AppConfig.DBPassword,
				config.AppConfig.DBName,
			)
			log.Printf("Connecting to legacy DB: host=%s port=%s user=%s dbname=%s",
				config.AppConfig.DBHost,
				config.AppConfig.DBPort,
				config.AppConfig.DBUser,
				config.AppConfig.DBName,
			)
		}

		LegacyDB, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("PostgreSQL legacy connection failed: %v", err)
			return err
		}

	case "sqlite", "sqlite3":
		dbDir := filepath.Dir(config.AppConfig.DBPath)
		if err := os.MkdirAll(dbDir, os.ModePerm); err != nil {
			log.Printf("Failed to create directory for SQLite DB: %v", err)
			return err
		}

		LegacyDB, err = sql.Open("sqlite3", config.AppConfig.DBPath)
		if err != nil {
			log.Printf("SQLite legacy connection failed: %v", err)
			return err
		}

		_, err = LegacyDB.Exec("PRAGMA foreign_keys = ON")
		if err != nil {
			log.Printf("Failed to enable foreign keys: %v", err)
			return err
		}
	default:
		return fmt.Errorf("Unsupported DB driver: %s", config.AppConfig.DBDriver)
	}

	if err = LegacyDB.Ping(); err != nil {
		log.Printf("Legacy DB ping failed: %v", err)
		return err
	}

	log.Println("✅ Legacy DB connected successfully")
	return nil
}

// CloseLegacyDB safely closes the legacy DB
func CloseLegacyDB() error {
	if LegacyDB != nil {
		return LegacyDB.Close()
	}
	return nil
}
