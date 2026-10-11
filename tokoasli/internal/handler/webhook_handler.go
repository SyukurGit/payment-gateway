package handler

import (
	"encoding/json"
	"io"
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

type WebhookHandler struct {
	db           *database.DB
	cfg          *config.Config
	sseHub       *service.SSEHub
	emailService *service.EmailService
}

func NewWebhookHandler(db *database.DB, cfg *config.Config, sseHub *service.SSEHub, emailService *service.EmailService) *WebhookHandler {
	return &WebhookHandler{
		db:           db,
		cfg:          cfg,
		sseHub:       sseHub,
		emailService: emailService,
	}
}

// HandlePaymentG processes incoming webhooks from PaymentG port 3200
func (h *WebhookHandler) HandlePaymentG(c *gin.Context) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Gagal membaca body"})
		return
	}

	// Verify HMAC signature if WEBHOOK_SECRET is configured
	if h.cfg.WebhookSecret != "" {
		sigHeader := c.GetHeader("X-Webhook-Signature")
		cleanSig := strings.TrimPrefix(sigHeader, "sha256=")
		if !util.VerifyHMAC(h.cfg.WebhookSecret, string(rawBody), cleanSig) {
			log.Printf("[WebhookHandler] Signature mismatch! Header: %s", sigHeader)
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Signature webhook tidak valid"})
			return
		}
	}

	var payload model.PaymentGWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Format JSON tidak valid"})
		return
	}

	log.Printf("[WebhookHandler] Menerima notifikasi pembayaran untuk ReferenceID: %s (ShopeeTx: %s)", payload.ReferenceID, payload.ShopeeTxID)

	// Look up order by ReferenceID (which is our store order ID)
	order, err := h.db.GetOrder(payload.ReferenceID)
	if err != nil {
		log.Printf("[WebhookHandler] Order tidak ditemukan: %s", payload.ReferenceID)
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	// Idempotency: if already paid, return 200 OK
	if order.Status == model.OrderStatusPaid {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Pesanan sudah berstatus lunas sebelumnya"})
		return
	}

	// Fetch product to determine delivery type
	product, err := h.db.GetProduct(order.ProductID)
	if err != nil {
		log.Printf("[WebhookHandler] Gagal mengambil data produk: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Data produk tidak ditemukan"})
		return
	}

	// Commit stock (convert held row to sold, or get fixed content)
	deliveryContent, err := h.db.CommitStock(order.ID, product.ID, product.DeliveryType, product.FixedContent)
	if err != nil {
		log.Printf("[WebhookHandler] Error commit stock: %v", err)
	}

	// Mark order as PAID
	if err := h.db.MarkOrderPaid(order.ID, deliveryContent, payload.ShopeeTxID); err != nil {
		log.Printf("[WebhookHandler] Gagal mengupdate status order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal update status pesanan"})
		return
	}

	// Update local struct for SSE & Email
	order.Status = model.OrderStatusPaid
	order.DeliveryContent = deliveryContent

	// Broadcast via SSE without leaking delivery credentials
	h.sseHub.Broadcast(order.ID, gin.H{
		"order_id": order.ID,
		"status":   model.OrderStatusPaid,
	})

	// Dispatch confirmation email with Secret Claim Link asynchronously
	h.emailService.SendReceiptAsync(order)

	log.Printf("[WebhookHandler] Sukses memproses pembayaran lunas untuk Order #%s! Produk terkirim.", order.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Pembayaran berhasil diproses"})
}
