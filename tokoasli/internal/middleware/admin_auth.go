package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"tokoasli/internal/service"
)

// AdminAuth validates Bearer token against active SessionManager sessions or fallback master password
func AdminAuth(sm *service.SessionManager, fallbackPassword string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authorization header missing"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid authorization format. Use 'Bearer <token>'"})
			c.Abort()
			return
		}

		providedToken := strings.TrimSpace(parts[1])

		// 1. Check if token is a valid active crypto session
		if sm != nil && sm.ValidateSession(providedToken) {
			c.Next()
			return
		}

		// 2. Fallback constant-time check against master password (for direct API/curl scripts)
		if fallbackPassword != "" && subtle.ConstantTimeCompare([]byte(providedToken), []byte(fallbackPassword)) == 1 {
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid or expired admin session. Please login again."})
		c.Abort()
	}
}
