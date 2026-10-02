package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"paymentg/internal/database"
)

func APIKeyAuth(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Missing API Key"})
			return
		}

		app, err := db.GetAppByAPIKey(apiKey)
		if err != nil || !app.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid or inactive API Key"})
			return
		}

		c.Set("app", app)
		c.Next()
	}
}

func AdminAuth(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Admin-Key")
		if key == "" || key != adminKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized admin"})
			return
		}
		c.Next()
	}
}
