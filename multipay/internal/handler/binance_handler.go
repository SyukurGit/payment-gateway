package handler

import (
	"fmt"
	"math"
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

type BinanceHandler struct {
	binanceEng *service.BinanceEngine
	webhook    *service.WebhookDispatcher
	cfg        *config.Config
}

func NewBinanceHandler(engine *service.BinanceEngine, wh *service.WebhookDispatcher, cfg *config.Config) *BinanceHandler {
	return &BinanceHandler{
		binanceEng: engine,
		webhook:    wh,
		cfg:        cfg,
	}
}

func (h *BinanceHandler) CreateOrder(c *gin.Context) {
	app := c.MustGet("app").(*model.App)
	db := c.MustGet("db").(*database.DB)

	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if req.Amount < 0.01 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum amount for Binance Pay is 0.01 USDT"})
		return
	}

	baseMilli := int64(math.Round(req.Amount * 1000.0))
	code, totalMilli, err := db.AllocateUniqueCode(model.ChannelBinance, "USDT", baseMilli)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Failed to allocate unique code. Please retry in a moment."})
		return
	}

	totalAmountFloat := float64(totalMilli) / 1000.0
	totalAmountStr := fmt.Sprintf("%.3f", totalAmountFloat)

	orderID := database.GenerateRandomID("ord_binance")
	if db.IsSandbox {
		orderID = database.GenerateRandomID("sbx_binance")
	}

	uniQR, _ := db.GetConfig("binance_uni_qr_url")
	if uniQR == "" {
		payID, _ := db.GetConfig("binance_pay_id")
		uniQR = fmt.Sprintf("https://app.binance.com/qr/dopaid?merchantId=%s&amount=%s", payID, totalAmountStr)
	}

	qrFilename := fmt.Sprintf("%s.png", orderID)
	qrFilePath := filepath.Join(db.DataDir, "qr", qrFilename)
	_ = h.binanceEng.GenerateQRCodeFile(uniQR, qrFilePath)

	expiryMin := req.ExpiryMinutes
	if expiryMin <= 0 {
		expiryMin = 15
	}
	nowMs := time.Now().UnixMilli()
	expiresAt := nowMs + int64(expiryMin*60*1000)

	payID, _ := db.GetConfig("binance_pay_id")
	order := &model.Order{
		ID:                  orderID,
		AppID:               app.ID,
		Channel:             model.ChannelBinance,
		ReferenceID:         req.ReferenceID,
		Currency:            "USDT",
		WalletAddress:       payID,
		OriginalAmount:      req.Amount,
		OriginalAmountMilli: baseMilli,
		UniqueCode:          code,
		TotalAmount:         totalAmountFloat,
		TotalAmountStr:      totalAmountStr,
		TotalAmountMilli:    totalMilli,
		Status:              model.StatusPending,
		QRContent:           uniQR,
		QRPath:              qrFilePath,
		CreatedAt:           nowMs,
		ExpiresAt:           expiresAt,
		CooldownUntil:       expiresAt + (60 * 60 * 1000), // 60m cooldown
		Metadata:            req.Metadata,
	}

	if err := db.CreateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save order: " + err.Error()})
		return
	}

	baseURL := c.GetString("base_url")
	qrEndpoint := fmt.Sprintf("%s/binance/api/orders/%s/qr.png", baseURL, order.ID)
	if db.IsSandbox {
		qrEndpoint = fmt.Sprintf("%s/binance/api/sandbox/orders/%s/qr.png", baseURL, order.ID)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"order": model.OrderResponse{
			ID:             order.ID,
			Channel:        order.Channel,
			ReferenceID:    order.ReferenceID,
			Currency:       order.Currency,
			WalletAddress:  order.WalletAddress,
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

func (h *BinanceHandler) GetOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil || order.Channel != model.ChannelBinance {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	remaining := (order.ExpiresAt - time.Now().UnixMilli()) / 1000
	if remaining < 0 {
		remaining = 0
	}

	baseURL := c.GetString("base_url")
	qrEndpoint := fmt.Sprintf("%s/binance/api/orders/%s/qr.png", baseURL, order.ID)
	if db.IsSandbox {
		qrEndpoint = fmt.Sprintf("%s/binance/api/sandbox/orders/%s/qr.png", baseURL, order.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"order": model.OrderResponse{
			ID:             order.ID,
			Channel:        order.Channel,
			ReferenceID:    order.ReferenceID,
			Currency:       order.Currency,
			WalletAddress:  order.WalletAddress,
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

func (h *BinanceHandler) CheckOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// In live mode, execute on-demand check if order is pending
	if !db.IsSandbox && order.Status == model.StatusPending {
		_, _ = h.binanceEng.ProcessIncomingTransactions()
		// Re-fetch to get latest status
		if updated, err := db.GetOrder(id); err == nil && updated != nil {
			order = updated
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  order.Status,
		"is_paid": order.Status == model.StatusPaid,
	})
}

func (h *BinanceHandler) CancelOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	if err := db.CancelOrder(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Order cancelled"})
}

func (h *BinanceHandler) GetQRImage(c *gin.Context, db *database.DB) {
	id := c.Param("id")
	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR image not found"})
		return
	}

	if _, err := os.Stat(order.QRPath); os.IsNotExist(err) && order.QRContent != "" {
		_ = h.binanceEng.GenerateQRCodeFile(order.QRContent, order.QRPath)
	}

	c.File(order.QRPath)
}

func (h *BinanceHandler) SimulatePay(c *gin.Context) {
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

	simTxID := fmt.Sprintf("SIM_BINANCE_%d", time.Now().UnixNano())
	if err := db.MarkOrderPaid(order.ID, simTxID, "", "DemoPayer#123", 0); err != nil {
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
