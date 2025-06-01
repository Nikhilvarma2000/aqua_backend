package utils

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"aquahome/config"
)

// JWTClaims represents the claims in the JWT token
type JWTClaims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWT generates a new JWT token
func GenerateJWT(userID uint, email, role string, expTime time.Time) (string, error) {
	// Create claims
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate signed token
	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateJWT validates a JWT token and extracts its claims
func ValidateJWT(tokenString string) (*JWTClaims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	// Extract claims
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GenerateResetToken generates a random token for password reset
func GenerateResetToken() string {
	// Generate a random string for password reset
	bytes := make([]byte, 32)
	for i := 0; i < 32; i++ {
		bytes[i] = byte(time.Now().Nanosecond() % 256)
		time.Sleep(1 * time.Nanosecond) // Add tiny sleep to ensure nanosecond changes
	}

	// Convert to hex string
	token := ""
	for _, b := range bytes {
		token += string(b)
	}

	return token
}

// GetAdminToken returns the admin token from config
func GetAdminToken() string {
	// Use environment variable or a default value
	return os.Getenv("ADMIN_TOKEN")
}
