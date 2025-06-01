package middleware

import (
	"aquahome/database"
	"aquahome/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT tokens and extracts user information
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := utils.ValidateJWT(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Fetch full user object from DB
		var user database.User
		if err := database.DB.First(&user, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// ✅ Set everything in context
		c.Set("userID", claims.UserID)
		c.Set("user_id", claims.UserID)
		c.Set("email", user.Email)
		c.Set("role", user.Role)
		c.Set("user", user) // ✅ THIS LINE IS THE KEY FIX

		c.Next()
	}
}

// RoleAuthMiddleware validates user roles
func RoleAuthMiddleware(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, r := range roles {
			if r == userRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		c.Abort()
	}
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return RoleAuthMiddleware("admin")
}

func FranchiseOwnerAuthMiddleware() gin.HandlerFunc {
	return RoleAuthMiddleware("admin", "franchise_owner")
}

func CustomerAuthMiddleware() gin.HandlerFunc {
	return RoleAuthMiddleware("customer", "admin")
}

func ServiceAgentAuthMiddleware() gin.HandlerFunc {
	return RoleAuthMiddleware("admin", "service_agent")
}

func AdminOrFranchiseAuthMiddleware() gin.HandlerFunc {
	return RoleAuthMiddleware("admin", "franchise_owner","service_agent")
}

func AdminOrServiceAgentAuthMiddleware() gin.HandlerFunc {
	return RoleAuthMiddleware("admin", "service_agent")
}
