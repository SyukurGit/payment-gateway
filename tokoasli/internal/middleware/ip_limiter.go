package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"tokoasli/internal/database"
)

// IPLimiter checks if the client IP already has maxPending or more active pending orders
func IPLimiter(db *database.DB, maxPending int) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		count, err := db.CountPendingOrdersByIP(clientIP)
		if err != nil {
			// On DB error, don't block legitimate users, just proceed
			c.Next()
			return
		}

		if count >= maxPending {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Anda masih memiliki %d pesanan tertunda yang belum dibayar. Selesaikan pembayaran, batalkan pesanan di halaman transaksi, atau tunggu hingga kedaluwarsa.", count),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
