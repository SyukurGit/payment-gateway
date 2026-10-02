package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"paymentg/internal/config"
	"paymentg/internal/database"
	"paymentg/internal/model"
	"paymentg/internal/service"
	"paymentg/internal/util"
)

type OrderHandler struct {
	db   *database.DB
	qris *service.QRISService
	cfg  *config.Config
}

func NewOrderHandler(db *database.DB, qris *service.QRISService, cfg *config.Config) *OrderHandler {
	return &OrderHandler{
		db:   db,
		qris: qris,
		cfg:  cfg,
	}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
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
	expiresAt := time.Now().Add(duration)

	uniqueCode, err := service.GenerateUniqueCode(h.db, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate unique code"})
		return
	}

	totalAmount := req.Amount + int64(uniqueCode)
	qrisString, err := h.qris.Generate(totalAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate QRIS string"})
		return
	}

	now := time.Now()
	order := model.Order{
		ID:             util.NewID(),
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
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create order"})
		return
	}

	qrBytes, err := h.qris.GenerateQRImage(totalAmount, 300)
	if err == nil {
		qrDir := filepath.Join(h.cfg.DataDir, "qr")
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
		QRURL:          fmt.Sprintf("/api/orders/%s/qr.png", order.ID),
		ExpiresAt:      order.ExpiresAt,
		ExpiresInSecs:  int(time.Until(expiresAt).Seconds()),
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	app := c.MustGet("app").(*model.App)

	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "order not found"})
		return
	}

	if order.AppID != app.ID {
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

func (h *OrderHandler) GetQRImage(c *gin.Context) {
	id := c.Param("id")
	qrPath := filepath.Join(h.cfg.DataDir, "qr", id+".png")
	
	bytes, err := os.ReadFile(qrPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "image not found"})
		return
	}
	
	c.Data(http.StatusOK, "image/png", bytes)
}

func (h *OrderHandler) CheckOrder(c *gin.Context) {
	id := c.Param("id")
	app := c.MustGet("app").(*model.App)

	order, err := h.db.GetOrder(id)
	if err != nil || order.AppID != app.ID {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "order not found"})
		return
	}

	if order.Status != model.StatusPending {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
		return
	}

	token, err := h.db.GetConfig("shopee_token")
	if err != nil || token == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "shopee token not configured"})
		return
	}

	client := service.NewShopeeClient()
	txs, err := client.FetchTransactions(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch transactions"})
		return
	}

	webhookSvc := service.NewWebhookService(h.db)
	matcher := service.NewMatcher(h.db, webhookSvc)
	matcher.MatchTransactions(txs)

	order, _ = h.db.GetOrder(id)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
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
