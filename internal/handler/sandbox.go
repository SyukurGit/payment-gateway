package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"paymentg/internal/config"
	"paymentg/internal/database"
	"paymentg/internal/model"
	"paymentg/internal/service"
	"paymentg/internal/util"
)

type SandboxHandler struct {
	db      *database.DB
	qris    *service.QRISService
	cfg     *config.Config
	webhook *service.WebhookService
	dataDir string
}

func NewSandboxHandler(db *database.DB, qris *service.QRISService, cfg *config.Config, webhook *service.WebhookService, dataDir string) *SandboxHandler {
	return &SandboxHandler{
		db:      db,
		qris:    qris,
		cfg:     cfg,
		webhook: webhook,
		dataDir: dataDir,
	}
}

// 1. Order Operations (Client API: X-API-Key)

func (h *SandboxHandler) CreateOrder(c *gin.Context) {
	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	appInterface, exists := c.Get("app")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	app, ok := appInterface.(*model.App)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal error"})
		return
	}

	expiryMinutes := req.ExpiryMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = h.cfg.DefaultExpiryMinutes
	}
	duration := time.Duration(expiryMinutes) * time.Minute
	now := time.Now().UTC()
	expiresAt := now.Add(duration)

	uniqueCode, err := service.GenerateUniqueCode(h.db, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate unique code: " + err.Error()})
		return
	}

	totalAmount := req.Amount + int64(uniqueCode)
	qrisString, err := h.qris.Generate(totalAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate QRIS string"})
		return
	}

	order := model.Order{
		ID:             "sbx_" + util.NewID(),
		AppID:          app.ID,
		ReferenceID:    req.ReferenceID,
		OriginalAmount: req.Amount,
		UniqueCode:     uniqueCode,
		TotalAmount:    totalAmount,
		Status:         model.StatusPending,
		QRString:       qrisString,
		ExpiresAt:      expiresAt.Format(time.RFC3339),
		CreatedAt:      now.Format(time.RFC3339),
		UpdatedAt:      now.Format(time.RFC3339),
		Metadata:       req.Metadata,
	}

	if err := h.db.CreateOrder(&order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create sandbox order"})
		return
	}

	qrBytes, err := h.qris.GenerateQRImage(totalAmount, 300)
	if err == nil {
		qrDir := filepath.Join(h.dataDir, "qr")
		os.MkdirAll(qrDir, 0755)
		os.WriteFile(filepath.Join(qrDir, order.ID+".png"), qrBytes, 0644)
	}

	resp := model.CreateOrderResponse{
		OrderID:        order.ID,
		ReferenceID:    order.ReferenceID,
		OriginalAmount: order.OriginalAmount,
		UniqueCode:     order.UniqueCode,
		TotalAmount:    order.TotalAmount,
		Status:         order.Status,
		QRURL:          fmt.Sprintf("/api/sandbox/orders/%s/qr.png", order.ID),
		ExpiresAt:      order.ExpiresAt,
		ExpiresInSecs:  int(time.Until(expiresAt).Seconds()),
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

func (h *SandboxHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	app := c.MustGet("app").(*model.App)

	order, err := h.db.GetOrder(id)
	if err != nil || order.AppID != app.ID {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "order not found"})
		return
	}

	resp := model.OrderStatusResponse{
		OrderID:        order.ID,
		ReferenceID:    order.ReferenceID,
		OriginalAmount: order.OriginalAmount,
		UniqueCode:     order.UniqueCode,
		TotalAmount:    order.TotalAmount,
		Status:         order.Status,
		PaidAt:         order.PaidAt,
		ShopeeTxID:     order.ShopeeTxID,
		CreatedAt:      order.CreatedAt,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *SandboxHandler) GetQRImage(c *gin.Context) {
	id := c.Param("id")
	qrPath := filepath.Join(h.dataDir, "qr", id+".png")

	bytes, err := os.ReadFile(qrPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "image not found"})
		return
	}

	c.Data(http.StatusOK, "image/png", bytes)
}

func (h *SandboxHandler) CheckOrder(c *gin.Context) {
	id := c.Param("id")
	app := c.MustGet("app").(*model.App)

	order, err := h.db.GetOrder(id)
	if err != nil || order.AppID != app.ID {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
}

func (h *SandboxHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	app := c.MustGet("app").(*model.App)

	order, err := h.db.GetOrder(id)
	if err != nil || order.AppID != app.ID {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "order not found"})
		return
	}

	if order.Status != model.StatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "order is not pending"})
		return
	}

	if err := h.db.UpdateOrderStatus(id, model.StatusCancelled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to cancel order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": "order cancelled"})
}

// 2. Simulator & Admin Operations (Admin Key or Dashboard PIN)

func (h *SandboxHandler) SimulatePay(c *gin.Context) {
	id := c.Param("id")

	order, err := h.db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "order not found"})
		return
	}

	if order.Status != model.StatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("order status is already %s, cannot simulate payment", order.Status)})
		return
	}

	dummyTxID := fmt.Sprintf("sbx_tx_%s_%d", util.NewID(), time.Now().Unix())
	if err := h.db.MarkOrderPaid(order.ID, dummyTxID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to mark order paid: " + err.Error()})
		return
	}

	_ = h.db.SaveProcessedTransaction(dummyTxID, order.TotalAmount, order.ID)

	updatedOrder, _ := h.db.GetOrder(order.ID)
	app, _ := h.db.GetAppByID(order.AppID)

	var webhookSent bool
	var webhookErr string
	if app != nil && updatedOrder != nil && app.WebhookURL != "" {
		if err := h.webhook.Send(app, updatedOrder); err != nil {
			webhookSent = false
			webhookErr = err.Error()
		} else {
			webhookSent = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"order":            updatedOrder,
			"webhook_sent":     webhookSent,
			"webhook_error":    webhookErr,
			"message":          "Pembayaran simulasi sandbox berhasil! Webhook telah ditembakkan ke web toko.",
		},
	})
}

func (h *SandboxHandler) ListOrders(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	orders, err := h.db.ListAllOrders(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch sandbox orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": orders})
}

func (h *SandboxHandler) GetStats(c *gin.Context) {
	stats, err := h.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch sandbox stats"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *SandboxHandler) CreateApp(c *gin.Context) {
	var req model.CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	app := model.App{
		ID:            "app_sbx_" + util.NewID(),
		Name:          req.Name,
		APIKey:        "ak_sbx_" + util.NewSecret()[:24],
		WebhookURL:    req.WebhookURL,
		WebhookSecret: "sbx_sec_" + util.NewSecret()[:24],
		IsActive:      true,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := h.db.CreateApp(&app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create sandbox app: " + err.Error()})
		return
	}

	resp := model.CreateAppResponse{
		ID:            app.ID,
		Name:          app.Name,
		APIKey:        app.APIKey,
		WebhookURL:    app.WebhookURL,
		WebhookSecret: app.WebhookSecret,
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

func (h *SandboxHandler) ListApps(c *gin.Context) {
	apps, err := h.db.ListApps()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch sandbox apps"})
		return
	}

	for i := range apps {
		if len(apps[i].APIKey) > 12 {
			apps[i].APIKey = apps[i].APIKey[:12] + "..."
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": apps})
}

func (h *SandboxHandler) DeleteApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.DeleteApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to delete sandbox app"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": "sandbox app deleted"})
}

var (
	sbxPinAttempts   = make(map[string]*pinAttempt)
	sbxPinAttemptsMu sync.Mutex
)

func (h *SandboxHandler) VerifyPIN(c *gin.Context) {
	clientIP := c.ClientIP()

	sbxPinAttemptsMu.Lock()
	attempt, exists := sbxPinAttempts[clientIP]
	if exists && attempt.count >= 5 && time.Since(attempt.lastError) < 1*time.Minute {
		sbxPinAttemptsMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"error":   "Terlalu banyak percobaan PIN salah. Silakan coba lagi dalam 1 menit.",
		})
		return
	}
	sbxPinAttemptsMu.Unlock()

	var req PINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "PIN wajib diisi"})
		return
	}

	if !h.db.VerifyPIN(req.PIN) {
		time.Sleep(500 * time.Millisecond)
		sbxPinAttemptsMu.Lock()
		if !exists {
			sbxPinAttempts[clientIP] = &pinAttempt{count: 1, lastError: time.Now()}
		} else {
			attempt.count++
			attempt.lastError = time.Now()
		}
		sbxPinAttemptsMu.Unlock()

		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "PIN akses salah"})
		return
	}

	sbxPinAttemptsMu.Lock()
	delete(sbxPinAttempts, clientIP)
	sbxPinAttemptsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"message":   "PIN valid",
			"admin_key": h.cfg.AdminKey,
		},
	})
}
