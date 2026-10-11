package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"tokoasli/internal/database"
	"tokoasli/internal/model"
	"tokoasli/internal/service"
)

type OrderHandler struct {
	db       *database.DB
	sseHub   *service.SSEHub
	paymentg *service.PaymentGClient
	emailSvc *service.EmailService
}

func NewOrderHandler(db *database.DB, sseHub *service.SSEHub, paymentg *service.PaymentGClient, emailSvc *service.EmailService) *OrderHandler {
	return &OrderHandler{
		db:       db,
		sseHub:   sseHub,
		paymentg: paymentg,
		emailSvc: emailSvc,
	}
}

// GetQRImage proxies the QRIS PNG from PaymentG so buyers on mobile networks do not get broken images or CORS errors
func (h *OrderHandler) GetQRImage(c *gin.Context) {
	id := c.Param("id")
	order, err := h.db.GetOrder(id)
	if err != nil || order.QRURL == "" {
		c.Status(http.StatusNotFound)
		return
	}

	targetURL := order.QRURL
	if strings.HasPrefix(targetURL, "/") && h.paymentg != nil {
		targetURL = h.paymentg.BaseURL() + targetURL
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "public, max-age=600")
	_, _ = io.Copy(c.Writer, resp.Body)
}

// GetOrder returns order status, and reveals secret delivery content ONLY if token matches and status is PAID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	token := c.Query("token")

	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	qrURL := order.QRURL
	if qrURL != "" {
		qrURL = fmt.Sprintf("/api/orders/%s/qr.png", order.ID)
	}

	resp := model.OrderPublicResponse{
		ID:            order.ID,
		ProductName:   order.ProductName,
		CustomerEmail: order.CustomerEmail,
		CustomerPhone: order.CustomerPhone,
		OriginalPrice: order.OriginalPrice,
		UniqueCode:    order.UniqueCode,
		TotalAmount:   order.TotalAmount,
		Status:        order.Status,
		QRString:      order.QRString,
		QRURL:         qrURL,
		ExpiresAt:     order.ExpiresAt,
		PaidAt:        order.PaidAt,
		CreatedAt:     order.CreatedAt,
	}

	// Secret Delivery Content is ONLY revealed if secret token matches AND status is PAID
	if token != "" && token == order.SecretToken && order.Status == model.OrderStatusPaid {
		resp.DeliveryContent = order.DeliveryContent
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// StreamOrder provides a Server-Sent Events (SSE) stream for real-time payment notification with keep-alive pings
func (h *OrderHandler) StreamOrder(c *gin.Context) {
	id := c.Param("id")

	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming unsupported"})
		return
	}

	// Send current status immediately (Never leak delivery content in SSE!)
	initialEvent, _ := json.Marshal(gin.H{
		"order_id": order.ID,
		"status":   order.Status,
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", initialEvent)
	flusher.Flush()

	// If already in terminal status, close stream
	if order.Status == model.OrderStatusPaid || order.Status == model.OrderStatusExpired || order.Status == model.OrderStatusCancelled {
		return
	}

	// Subscribe to realtime broadcast
	eventChan, unsubscribe := h.sseHub.Subscribe(id)
	defer unsubscribe()

	// Ping ticker (20s) to keep reverse-proxy connections (Cloudflare/Nginx) alive
	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-pingTicker.C:
			fmt.Fprintf(c.Writer, ": ping\n\n")
			flusher.Flush()
		case eventData, ok := <-eventChan:
			if !ok {
				return
			}
			c.Writer.Write([]byte("data: "))
			c.Writer.Write(eventData)
			c.Writer.Write([]byte("\n\n"))
			flusher.Flush()
			return // After terminal event is received, close stream
		}
	}
}

// CheckOrderStatus allows buyer to manually trigger status check against PaymentG
func (h *OrderHandler) CheckOrderStatus(c *gin.Context) {
	id := c.Param("id")

	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	// If already paid, return status
	if order.Status == model.OrderStatusPaid {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"order_id": order.ID,
				"status":   order.Status,
				"message":  "Pesanan sudah lunas",
			},
		})
		return
	}

	// Query gateway
	pgOrderID := order.PaymentGOrderID
	if pgOrderID == "" {
		pgOrderID = order.ID
	}

	checkResp, err := h.paymentg.CheckOrder(pgOrderID)
	if err != nil {
		log.Printf("[OrderHandler] Cek status ke PaymentG gagal: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"order_id": order.ID,
				"status":   order.Status,
				"message":  "Mutasi pembayaran belum terdeteksi. Silakan coba kembali dalam 1-2 menit.",
			},
		})
		return
	}

	if checkResp.Data.Status == model.OrderStatusPaid {
		// Fetch product to determine delivery type
		product, prodErr := h.db.GetProduct(order.ProductID)
		deliveryType := model.DeliveryTypePool
		fixedContent := ""
		if prodErr == nil {
			deliveryType = product.DeliveryType
			fixedContent = product.FixedContent
		}

		deliveryContent, err := h.db.CommitStock(order.ID, order.ProductID, deliveryType, fixedContent)
		if err != nil {
			log.Printf("[OrderHandler] Gagal commit stok saat check manual: %v", err)
		}

		if err := h.db.MarkOrderPaid(order.ID, deliveryContent, pgOrderID); err != nil {
			log.Printf("[OrderHandler] Gagal update order paid saat check manual: %v", err)
		}

		order.Status = model.OrderStatusPaid
		order.DeliveryContent = deliveryContent

		// Broadcast without leaking delivery content
		h.sseHub.Broadcast(order.ID, gin.H{
			"order_id": order.ID,
			"status":   model.OrderStatusPaid,
		})

		// Send email asynchronously
		go func(o model.StoreOrder) {
			_ = h.emailSvc.SendReceipt(&o)
		}(*order)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"order_id": order.ID,
			"status":   order.Status,
			"message":  fmt.Sprintf("Status pesanan saat ini: %s", order.Status),
		},
	})
}

// CancelOrder allows buyer to cancel a pending order and release held stock immediately
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	token := c.Query("token")
	if token == "" {
		var body struct {
			Token string `json:"token"`
		}
		_ = c.ShouldBindJSON(&body)
		token = body.Token
	}

	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	// Verify token
	if token == "" || token != order.SecretToken {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Token otorisasi pesanan tidak valid"})
		return
	}

	if order.Status != model.OrderStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Pesanan dengan status %s tidak dapat dibatalkan", order.Status)})
		return
	}

	// 1. Cancel in local database
	if err := h.db.CancelOrder(order.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal membatalkan pesanan"})
		return
	}

	// 2. Release held stock immediately
	_ = h.db.ReleaseHeldStockForOrder(order.ID)

	// 3. Cancel on gateway
	pgOrderID := order.PaymentGOrderID
	if pgOrderID == "" {
		pgOrderID = order.ID
	}
	_ = h.paymentg.CancelOrder(pgOrderID)

	// 4. Broadcast cancellation event
	h.sseHub.Broadcast(order.ID, gin.H{
		"order_id": order.ID,
		"status":   model.OrderStatusCancelled,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Pesanan berhasil dibatalkan dan stok telah dilepas.",
	})
}
