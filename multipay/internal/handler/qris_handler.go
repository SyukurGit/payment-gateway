package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"multipay/internal/config"
	"multipay/internal/database"
	"multipay/internal/model"
	"multipay/internal/service"
)

type QRISHandler struct {
	qrisEngine *service.QRISEngine
	webhook    *service.WebhookDispatcher
	cfg        *config.Config
}

func NewQRISHandler(engine *service.QRISEngine, wh *service.WebhookDispatcher, cfg *config.Config) *QRISHandler {
	return &QRISHandler{
		qrisEngine: engine,
		webhook:    wh,
		cfg:        cfg,
	}
}

func (h *QRISHandler) CreateOrder(c *gin.Context) {
	app := c.MustGet("app").(*model.App)
	db := c.MustGet("db").(*database.DB)

	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if req.Amount < 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum amount for QRIS is Rp 1.000"})
		return
	}

	baseAmountInt := int64(req.Amount)
	code, totalMilli, err := db.AllocateUniqueCode(model.ChannelQRIS, "IDR", baseAmountInt)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Failed to allocate unique code. Please retry in a moment."})
		return
	}

	dynamicQRString, err := h.qrisEngine.GenerateDynamicQRIS(totalMilli)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate dynamic QRIS: " + err.Error()})
		return
	}

	orderID := database.GenerateRandomID("ord_qris")
	if db.IsSandbox {
		orderID = database.GenerateRandomID("sbx_qris")
	}

	qrFilename := fmt.Sprintf("%s.png", orderID)
	qrFilePath := filepath.Join(db.DataDir, "qr", qrFilename)
	_ = h.qrisEngine.GenerateQRCodeFile(dynamicQRString, qrFilePath)

	expiryMin := req.ExpiryMinutes
	if expiryMin <= 0 {
		expiryMin = 15
	}
	nowMs := time.Now().UnixMilli()
	expiresAt := nowMs + int64(expiryMin*60*1000)

	order := &model.Order{
		ID:                  orderID,
		AppID:               app.ID,
		Channel:             model.ChannelQRIS,
		ReferenceID:         req.ReferenceID,
		Currency:            "IDR",
		OriginalAmount:      req.Amount,
		OriginalAmountMilli: baseAmountInt,
		UniqueCode:          code,
		TotalAmount:         float64(totalMilli),
		TotalAmountStr:      fmt.Sprintf("%d", totalMilli),
		TotalAmountMilli:    totalMilli,
		Status:              model.StatusPending,
		QRContent:           dynamicQRString,
		QRPath:              qrFilePath,
		CreatedAt:           nowMs,
		ExpiresAt:           expiresAt,
		CooldownUntil:       expiresAt + (15 * 60 * 1000),
		Metadata:            req.Metadata,
	}

	if err := db.CreateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save order: " + err.Error()})
		return
	}

	baseURL := c.GetString("base_url")
	qrEndpoint := fmt.Sprintf("%s/qris/api/orders/%s/qr.png", baseURL, order.ID)
	if db.IsSandbox {
		qrEndpoint = fmt.Sprintf("%s/qris/api/sandbox/orders/%s/qr.png", baseURL, order.ID)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"order": model.OrderResponse{
			ID:             order.ID,
			Channel:        order.Channel,
			ReferenceID:    order.ReferenceID,
			Currency:       order.Currency,
			OriginalAmount: order.OriginalAmount,
			UniqueCode:     order.UniqueCode,
			TotalAmount:    order.TotalAmount,
			TotalAmountStr: order.TotalAmountStr,
			Status:         order.Status,
			QRURL:          qrEndpoint,
			QRContent:      order.QRContent,
			CreatedAt:      order.CreatedAt,
			ExpiresAt:      order.ExpiresAt,
			RemainingSec:   int64(expiryMin * 60),
		},
	})
}

func (h *QRISHandler) GetOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil || order.Channel != model.ChannelQRIS {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	remaining := (order.ExpiresAt - time.Now().UnixMilli()) / 1000
	if remaining < 0 {
		remaining = 0
	}

	baseURL := c.GetString("base_url")
	qrEndpoint := fmt.Sprintf("%s/qris/api/orders/%s/qr.png", baseURL, order.ID)
	if db.IsSandbox {
		qrEndpoint = fmt.Sprintf("%s/qris/api/sandbox/orders/%s/qr.png", baseURL, order.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"order": model.OrderResponse{
			ID:             order.ID,
			Channel:        order.Channel,
			ReferenceID:    order.ReferenceID,
			Currency:       order.Currency,
			OriginalAmount: order.OriginalAmount,
			UniqueCode:     order.UniqueCode,
			TotalAmount:    order.TotalAmount,
			TotalAmountStr: order.TotalAmountStr,
			Status:         order.Status,
			QRURL:          qrEndpoint,
			CreatedAt:      order.CreatedAt,
			ExpiresAt:      order.ExpiresAt,
			RemainingSec:   remaining,
			PaidAt:         order.PaidAt,
		},
	})
}

func (h *QRISHandler) CheckOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// In live mode, poll ShopeePay immediately if pending
	if !db.IsSandbox && order.Status == model.StatusPending {
		token, _ := db.GetConfig("shopee_token")
		if token != "" {
			if txs, err := h.qrisEngine.FetchShopeeTransactions(token); err == nil {
				for _, tx := range txs {
					if tx.Status == 1 || tx.Status == 3 {
						amt, _ := service.ParseIDRAmount(tx.Amount)
						if amt == order.TotalAmountMilli {
							_ = db.MarkOrderPaid(order.ID, tx.TransactionID, "", "", 0)
							order.Status = model.StatusPaid
							order.PaidAt = time.Now().UnixMilli()
							order.TxID = tx.TransactionID
							go h.webhook.DispatchOrder(db, order)
							break
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  order.Status,
		"is_paid": order.Status == model.StatusPaid,
	})
}

func (h *QRISHandler) CancelOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	if err := db.CancelOrder(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Order cancelled"})
}

func (h *QRISHandler) GetQRImage(c *gin.Context, db *database.DB) {
	id := c.Param("id")
	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR image not found"})
		return
	}

	// Auto-heal QR image if deleted
	if _, err := os.Stat(order.QRPath); os.IsNotExist(err) && order.QRContent != "" {
		_ = h.qrisEngine.GenerateQRCodeFile(order.QRContent, order.QRPath)
	}

	c.File(order.QRPath)
}

func (h *QRISHandler) SimulatePay(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.Status == model.StatusPaid {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Order already marked as paid"})
		return
	}

	simTxID := fmt.Sprintf("SIM_SHOPEE_%d", time.Now().UnixNano())
	if err := db.MarkOrderPaid(order.ID, simTxID, "", "", 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark order as paid: " + err.Error()})
		return
	}

	order.Status = model.StatusPaid
	order.TxID = simTxID
	order.PaidAt = time.Now().UnixMilli()

	go h.webhook.DispatchOrder(db, order)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment simulation successful. Webhook dispatched to store.",
		"order":   order,
	})
}
