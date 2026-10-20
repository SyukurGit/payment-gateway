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

type USDTHandler struct {
	usdtEngine *service.USDTEngine
	webhook    *service.WebhookDispatcher
	cfg        *config.Config
}

func NewUSDTHandler(engine *service.USDTEngine, wh *service.WebhookDispatcher, cfg *config.Config) *USDTHandler {
	return &USDTHandler{
		usdtEngine: engine,
		webhook:    wh,
		cfg:        cfg,
	}
}

func (h *USDTHandler) CreateOrder(c *gin.Context) {
	app := c.MustGet("app").(*model.App)
	db := c.MustGet("db").(*database.DB)

	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if req.Amount < 0.01 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum amount for USDT BEP-20 is 0.01 USDT"})
		return
	}

	baseMilli := int64(math.Round(req.Amount * 1000.0))
	code, totalMilli, err := db.AllocateUniqueCode(model.ChannelUSDT, "USDT", baseMilli)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Failed to allocate unique code. Please retry in a moment."})
		return
	}

	totalAmountFloat := float64(totalMilli) / 1000.0
	totalAmountStr := fmt.Sprintf("%.3f", totalAmountFloat)

	orderID := database.GenerateRandomID("ord_usdt")
	if db.IsSandbox {
		orderID = database.GenerateRandomID("sbx_usdt")
	}

	receiverWallet, _ := db.GetConfig("receiver_wallet_address")
	if receiverWallet == "" {
		receiverWallet = h.cfg.ReceiverWalletAddress
	}

	// EVM URI or plain address
	qrContent := receiverWallet

	qrFilename := fmt.Sprintf("%s.png", orderID)
	qrFilePath := filepath.Join(db.DataDir, "qr", qrFilename)
	_ = h.usdtEngine.GenerateQRCodeFile(qrContent, qrFilePath)

	expiryMin := req.ExpiryMinutes
	if expiryMin <= 0 {
		expiryMin = 15
	}
	nowMs := time.Now().UnixMilli()
	expiresAt := nowMs + int64(expiryMin*60*1000)

	order := &model.Order{
		ID:                  orderID,
		AppID:               app.ID,
		Channel:             model.ChannelUSDT,
		ReferenceID:         req.ReferenceID,
		Currency:            "USDT",
		Network:             "BSC",
		WalletAddress:       receiverWallet,
		OriginalAmount:      req.Amount,
		OriginalAmountMilli: baseMilli,
		UniqueCode:          code,
		TotalAmount:         totalAmountFloat,
		TotalAmountStr:      totalAmountStr,
		TotalAmountMilli:    totalMilli,
		Status:              model.StatusPending,
		QRContent:           qrContent,
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
	qrEndpoint := fmt.Sprintf("%s/usdt/api/orders/%s/qr.png", baseURL, order.ID)
	if db.IsSandbox {
		qrEndpoint = fmt.Sprintf("%s/usdt/api/sandbox/orders/%s/qr.png", baseURL, order.ID)
	}

	cexWarning := fmt.Sprintf(
		"PENTING: Hanya kirim USDT melalui jaringan BNB Smart Chain (BEP-20). Jika mengirim dari Exchange (Indodax/Tokocrypto/Bybit/Binance), pastikan Jumlah Bersih Diterima (Receive Amount) adalah TEPAT %s USDT setelah dipotong fee penarikan.",
		order.TotalAmountStr,
	)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"order": model.OrderResponse{
			ID:             order.ID,
			Channel:        order.Channel,
			ReferenceID:    order.ReferenceID,
			Currency:       order.Currency,
			Network:        order.Network,
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
			CEXWarning:     cexWarning,
		},
	})
}

func (h *USDTHandler) GetOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil || order.Channel != model.ChannelUSDT {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	remaining := (order.ExpiresAt - time.Now().UnixMilli()) / 1000
	if remaining < 0 {
		remaining = 0
	}

	baseURL := c.GetString("base_url")
	qrEndpoint := fmt.Sprintf("%s/usdt/api/orders/%s/qr.png", baseURL, order.ID)
	if db.IsSandbox {
		qrEndpoint = fmt.Sprintf("%s/usdt/api/sandbox/orders/%s/qr.png", baseURL, order.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"order": model.OrderResponse{
			ID:             order.ID,
			Channel:        order.Channel,
			ReferenceID:    order.ReferenceID,
			Currency:       order.Currency,
			Network:        order.Network,
			WalletAddress:  order.WalletAddress,
			OriginalAmount: order.OriginalAmount,
			UniqueCode:     order.UniqueCode,
			TotalAmount:    order.TotalAmount,
			TotalAmountStr: order.TotalAmountStr,
			Status:         order.Status,
			TxHash:         order.TxHash,
			FromInfo:       order.FromInfo,
			BlockNumber:    order.BlockNumber,
			QRURL:          qrEndpoint,
			CreatedAt:      order.CreatedAt,
			ExpiresAt:      order.ExpiresAt,
			RemainingSec:   remaining,
			PaidAt:         order.PaidAt,
		},
	})
}

func (h *USDTHandler) CheckOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// On-demand scan in live mode if pending
	if !db.IsSandbox && order.Status == model.StatusPending {
		_, _ = h.usdtEngine.ScanOnChainTransfers(c.Request.Context())
		if updated, err := db.GetOrder(id); err == nil && updated != nil {
			order = updated
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  order.Status,
		"is_paid": order.Status == model.StatusPaid,
		"tx_hash": order.TxHash,
	})
}

func (h *USDTHandler) CancelOrder(c *gin.Context) {
	db := c.MustGet("db").(*database.DB)
	id := c.Param("id")

	if err := db.CancelOrder(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Order cancelled"})
}

func (h *USDTHandler) GetQRImage(c *gin.Context, db *database.DB) {
	id := c.Param("id")
	order, err := db.GetOrder(id)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR image not found"})
		return
	}

	if _, err := os.Stat(order.QRPath); os.IsNotExist(err) && order.QRContent != "" {
		_ = h.usdtEngine.GenerateQRCodeFile(order.QRContent, order.QRPath)
	}

	c.File(order.QRPath)
}

func (h *USDTHandler) SimulatePay(c *gin.Context) {
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

	simTxHash := fmt.Sprintf("0x%016x%016x%016x%016x", time.Now().UnixNano(), time.Now().UnixMicro(), time.Now().Unix(), time.Now().UnixNano()^0x5555555555555555)
	simSender := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	simBlock := time.Now().Unix()

	if err := db.MarkOrderPaid(order.ID, simTxHash, simTxHash, simSender, simBlock); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark order as paid: " + err.Error()})
		return
	}

	order.Status = model.StatusPaid
	order.TxID = simTxHash
	order.TxHash = simTxHash
	order.FromInfo = simSender
	order.BlockNumber = simBlock
	order.PaidAt = time.Now().UnixMilli()

	go h.webhook.DispatchOrder(db, order)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment simulation successful. Webhook dispatched to store.",
		"order":   order,
	})
}
