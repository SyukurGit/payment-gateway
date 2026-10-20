package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"multipay/internal/config"
	"multipay/internal/database"
)

type pinAttempt struct {
	count     int
	blockedAt time.Time
}

var (
	pinAttempts   = make(map[string]*pinAttempt)
	pinAttemptsMu sync.Mutex
)

// AdminAuth validates the X-Admin-Key header against DB or config
func AdminAuth(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminKey := c.GetHeader("X-Admin-Key")
		if adminKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				adminKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if adminKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing X-Admin-Key header"})
			return
		}

		expectedKey, _ := db.GetConfig("admin_key")
		if expectedKey == "" {
			expectedKey = cfg.AdminKey
		}

		if subtle.ConstantTimeCompare([]byte(adminKey), []byte(expectedKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid X-Admin-Key"})
			return
		}

		c.Next()
	}
}

// ClientAuth validates X-API-Key and attaches the appropriate DB (Live vs Sandbox)
func ClientAuth(liveDB *database.DB, sandboxDB *database.DB, forceSandbox bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing X-API-Key header"})
			return
		}

		isTestKey := strings.HasPrefix(apiKey, "ak_test_")
		isLiveKey := strings.HasPrefix(apiKey, "ak_live_")

		if forceSandbox && isLiveKey {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Production API keys are not permitted in sandbox endpoints"})
			return
		}
		if !forceSandbox && isTestKey {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Sandbox API keys are not permitted in production endpoints"})
			return
		}

		var targetDB *database.DB
		if forceSandbox || isTestKey {
			targetDB = sandboxDB
		} else {
			targetDB = liveDB
		}

		if targetDB == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Target database not initialized"})
			return
		}

		app, err := targetDB.GetAppByAPIKey(apiKey)
		if err != nil || app == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or inactive API key"})
			return
		}

		c.Set("app", app)
		c.Set("db", targetDB)
		c.Set("is_sandbox", targetDB.IsSandbox)
		c.Next()
	}
}

// PINRateLimiter protects the PIN verification endpoint from brute force
func PINRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		pinAttemptsMu.Lock()
		att, exists := pinAttempts[ip]
		if exists && !att.blockedAt.IsZero() {
			if time.Since(att.blockedAt) < 5*time.Minute {
				pinAttemptsMu.Unlock()
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "Terlalu banyak percobaan salah. IP Anda dikunci sementara selama 5 menit.",
				})
				return
			}
			// Unblock after 5 mins
			delete(pinAttempts, ip)
		}
		pinAttemptsMu.Unlock()

		c.Next()
	}
}

func RecordFailedPIN(ip string) {
	pinAttemptsMu.Lock()
	defer pinAttemptsMu.Unlock()

	att, exists := pinAttempts[ip]
	if !exists {
		att = &pinAttempt{count: 0}
		pinAttempts[ip] = att
	}
	att.count++
	if att.count >= 5 {
		att.blockedAt = time.Now()
	}
}

func ResetPINAttempts(ip string) {
	pinAttemptsMu.Lock()
	defer pinAttemptsMu.Unlock()
	delete(pinAttempts, ip)
}
