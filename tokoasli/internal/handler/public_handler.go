package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"tokoasli/internal/config"
	"tokoasli/internal/database"
	"tokoasli/internal/model"
	"tokoasli/internal/service"
	"tokoasli/internal/util"
)

type PublicHandler struct {
	db       *database.DB
	cfg      *config.Config
	paymentg *service.PaymentGClient
}

func NewPublicHandler(db *database.DB, cfg *config.Config, paymentg *service.PaymentGClient) *PublicHandler {
	return &PublicHandler{
		db:       db,
		cfg:      cfg,
		paymentg: paymentg,
	}
}

// GetStoreConfig returns public configuration (e.g. WhatsApp CS number)
func (h *PublicHandler) GetStoreConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"whatsapp_number": h.cfg.WhatsAppNumber,
			"store_url":       h.cfg.StoreURL,
		},
	})
}

// ListProducts returns all active products for the public store catalog
func (h *PublicHandler) ListProducts(c *gin.Context) {
	products, err := h.db.ListActiveProducts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengambil daftar produk"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": products})
}

// GetProduct returns single product detail
func (h *PublicHandler) GetProduct(c *gin.Context) {
	id := c.Param("id")
	product, err := h.db.GetProduct(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Produk tidak ditemukan"})
		return
	}
	if !product.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Produk sudah tidak aktif"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": product})
}

// CreateOrder processes public checkout, holds stock, and calls PaymentG for QRIS
func (h *PublicHandler) CreateOrder(c *gin.Context) {
	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Format input tidak valid. Pastikan email benar."})
		return
	}

	clientIP := c.ClientIP()

	// Anti-spam protection: max 5 active pending orders per IP
	pendingCount, err := h.db.CountPendingOrdersByIP(clientIP)
	if err == nil && pendingCount >= 5 {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"error":   "Terlalu banyak pesanan tertunda aktif dari perangkat Anda (maksimal 5). Selesaikan atau batalkan pesanan sebelumnya terlebih dahulu.",
		})
		return
	}

	product, err := h.db.GetProduct(req.ProductID)
	if err != nil || !product.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Produk tidak ditemukan atau sedang tidak aktif"})
		return
	}

	var createdOrder *model.StoreOrder
	var lastErr error

	// Retry loop (up to 3 attempts) to ensure unique Order ID & resilient stock reservation
	for attempt := 0; attempt < 3; attempt++ {
		orderID := util.NewOrderID()
		secretToken := util.NewSecretToken()

		// 1. Hold stock if product uses pool
		var stockHeld = false
		if product.DeliveryType == model.DeliveryTypePool {
			_, err := h.db.HoldStock(product.ID, orderID, 15)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Maaf, stok produk ini baru saja habis!"})
				return
			}
			stockHeld = true
		}

		// 2. Request QRIS from PaymentG (Port 3200)
		pgResp, err := h.paymentg.CreateOrder(orderID, int64(product.Price), 15)
		if err != nil {
			if stockHeld {
				_ = h.db.ReleaseHeldStockForOrder(orderID)
			}
			log.Printf("[PublicHandler] Gagal membuat order di PaymentG: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "Gagal menghubungi sistem gateway pembayaran. Coba lagi dalam beberapa saat."})
			return
		}

		qrURL := pgResp.Data.QRURL
		if qrURL != "" && qrURL[0] == '/' {
			qrURL = h.cfg.PaymentGURL + qrURL
		}

		order := &model.StoreOrder{
			ID:              orderID,
			SecretToken:     secretToken,
			ProductID:       product.ID,
			ProductName:     product.Name,
			CustomerEmail:   strings.TrimSpace(req.CustomerEmail),
			CustomerPhone:   strings.TrimSpace(req.CustomerPhone),
			OriginalPrice:   product.Price,
			UniqueCode:      pgResp.Data.UniqueCode,
			TotalAmount:     pgResp.Data.TotalAmount,
			Status:          model.OrderStatusPending,
			PaymentGOrderID: pgResp.Data.OrderID,
			QRURL:           qrURL,
			ClientIP:        clientIP,
			ExpiresAt:       pgResp.Data.ExpiresAt,
		}

		if err := h.db.CreateOrder(order); err == nil {
			createdOrder = order
			break
		} else {
			lastErr = err
			if stockHeld {
				_ = h.db.ReleaseHeldStockForOrder(orderID)
			}
		}
	}

	if createdOrder == nil {
		log.Printf("[PublicHandler] Gagal menyimpan order ke database setelah 3 percobaan: %v", lastErr)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal membuat pesanan"})
		return
	}

	// 4. Return response to buyer
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": model.CreateOrderResponse{
			OrderID:       createdOrder.ID,
			SecretToken:   createdOrder.SecretToken,
			ProductName:   createdOrder.ProductName,
			CustomerEmail: createdOrder.CustomerEmail,
			CustomerPhone: createdOrder.CustomerPhone,
			OriginalPrice: createdOrder.OriginalPrice,
			UniqueCode:    createdOrder.UniqueCode,
			TotalAmount:   createdOrder.TotalAmount,
			QRURL:         fmt.Sprintf("/api/orders/%s/qr.png", createdOrder.ID),
			ExpiresAt:     createdOrder.ExpiresAt,
			Status:        createdOrder.Status,
		},
	})
}

// TrackOrder allows customer to recover secret link by entering Order ID and Email
func (h *PublicHandler) TrackOrder(c *gin.Context) {
	var req model.TrackOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invoice ID dan Email harus diisi"})
		return
	}

	order, err := h.db.TrackOrder(req.OrderID, req.CustomerEmail)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan atau email tidak sesuai"})
		return
	}

	claimURL := fmt.Sprintf("%s/order/?id=%s&token=%s", h.cfg.StoreURL, order.ID, order.SecretToken)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": model.TrackOrderResponse{
			OrderID:       order.ID,
			SecretToken:   order.SecretToken,
			ProductName:   order.ProductName,
			CustomerEmail: order.CustomerEmail,
			Status:        order.Status,
			TotalAmount:   order.TotalAmount,
			CreatedAt:     order.CreatedAt,
			ClaimURL:      claimURL,
		},
	})
}
