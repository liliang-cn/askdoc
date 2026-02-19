package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Auth returns an API key authentication middleware
func Auth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth if no API key configured
		if apiKey == "" {
			c.Next()
			return
		}

		// Get API key from header
		key := c.GetHeader("X-API-Key")
		if key == "" {
			// Also try Authorization header
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if key != apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Next()
	}
}
