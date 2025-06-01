package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Database config
	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPath     string // SQLite database file path

	// Auth config
	JWTSecret      string
	JWTExpiryHours int

	// App config
	Environment string

	// Payment config
	RazorpayKey    string
	RazorpaySecret string
}

var AppConfig Config

// InitConfig initializes the application configuration
func InitConfig() {
	// Set default database driver to PostgreSQL
	dbDriver := getEnv("DB_DRIVER", "postgres")

	AppConfig = Config{
		DBDriver:       dbDriver,
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres"),
		DBName:         getEnv("DB_NAME", "aquahome"),
		DBPath:         getEnv("DB_PATH", "./aquahome.db"), // Default SQLite database path
		JWTSecret:      getEnv("JWT_SECRET", "aquahome_default_secret_key"),
		JWTExpiryHours: getEnvAsInt("JWT_EXPIRY_HOURS", 24),
		Environment:    getEnv("ENVIRONMENT", "development"),
		RazorpayKey:    getEnv("RAZORPAY_KEY", "rzp_test_QfMQ0LRiTplCvR"),
		RazorpaySecret: getEnv("RAZORPAY_SECRET", "169NdofVMND0u1o8yTWsgx47"),
	}
}

// Helper function to get environment variable with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Helper function to get integer environment variable with fallback
func getEnvAsInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if value, err := strconv.Atoi(strValue); err == nil {
		return value
	}
	return fallback
}

// GetJWTExpiration returns JWT expiration time
func GetJWTExpiration() time.Duration {
	return time.Duration(AppConfig.JWTExpiryHours) * time.Hour
}

// IsDevelopment returns true if the application is running in development mode
func IsDevelopment() bool {
	return AppConfig.Environment == "development"
}
