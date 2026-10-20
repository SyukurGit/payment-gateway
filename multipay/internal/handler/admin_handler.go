package handler

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"multipay/internal/config"
	"multipay/internal/database"
	"multipay/internal/middleware"
	"multipay/internal/model"
	"multipay/internal/service"
)

type AdminHandler struct {
	liveDB     *database.DB
	sandboxDB  *database.DB
	cfg        *config.Config
	qrisEngine *service.QRISEngine
	binanceEng *service.BinanceEngine
	usdtEngine *service.USDTEngine
	webhook    *service.WebhookDispatcher
}

func NewAdminHandler(
	liveDB *database.DB,
	sandboxDB *database.DB,
	cfg *config.Config,
	qrisEngine *service.QRISEngine,
	binanceEng *service.BinanceEngine,
	usdtEngine *service.USDTEngine,
	webhook *service.WebhookDispatcher,
) *AdminHandler {
	return &AdminHandler{
		liveDB:     liveDB,
		sandboxDB:  sandboxDB,
		cfg:        cfg,
		qrisEngine: qrisEngine,
		binanceEng: binanceEng,
		usdtEngine: usdtEngine,
		webhook:    webhook,
	}
}

// LoginPIN verifies admin PIN with brute-force rate limiting
func (h *AdminHandler) LoginPIN(c *gin.Context) {
	var req struct {
		PIN string `json:"pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PIN is required"})
		return
	}

	expectedPIN, _ := h.liveDB.GetConfig("admin_pin")
	if expectedPIN == "" {
		expectedPIN = h.cfg.AdminPIN
	}
	if expectedPIN == "" {
		expectedPIN = "2207"
	}

	ip := c.ClientIP()
	if subtle.ConstantTimeCompare([]byte(req.PIN), []byte(expectedPIN)) != 1 {
		middleware.RecordFailedPIN(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PIN salah. Silakan coba lagi."})
		return
	}

	middleware.ResetPINAttempts(ip)

	adminKey, _ := h.liveDB.GetConfig("admin_key")
	if adminKey == "" {
		adminKey = h.cfg.AdminKey
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"token":     adminKey,
		"admin_key": adminKey,
		"message":   "Login berhasil",
	})
}

// GetConfig returns runtime configurations per channel with defaults pre-filled
func (h *AdminHandler) GetConfig(c *gin.Context) {
	allConfig, err := h.liveDB.GetAllConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fill defaults if empty
	defaultVal := func(key, fallback string) string {
		if v, exists := allConfig[key]; exists && v != "" {
			return v
		}
		return fallback
	}

	tokenAgeHours := 0
	updatedAtStr := defaultVal("shopee_token_updated_at", "")
	if updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			tokenAgeHours = int(time.Since(t).Hours())
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"qris": gin.H{
			"shopee_token":            defaultVal("shopee_token", h.cfg.ShopeeToken),
			"shopee_token_updated_at": updatedAtStr,
			"token_age_hours":         tokenAgeHours,
			"token_is_active":         defaultVal("shopee_token", "") != "" && tokenAgeHours < 24,
			"qris_string":             defaultVal("qris_string", h.cfg.QRISString),
		},
		"binance": gin.H{
			"pay_id":       defaultVal("binance_pay_id", h.cfg.BinancePayID),
			"account_name": defaultVal("binance_account_name", h.cfg.BinanceAccountName),
			"uni_qr_url":   defaultVal("binance_uni_qr_url", h.cfg.BinanceUniQRURL),
			"api_key":      defaultVal("binance_api_key", h.cfg.BinanceAPIKey),
			"secret_key":   defaultVal("binance_secret_key", h.cfg.BinanceSecretKey),
		},
		"usdt": gin.H{
			"receiver_wallet_address": defaultVal("receiver_wallet_address", h.cfg.ReceiverWalletAddress),
			"bsc_rpc_urls":            defaultVal("bsc_rpc_urls", strings.Join(h.cfg.BSCRPCURLs, ",")),
			"required_confirmations":  defaultVal("required_confirmations", "2"),
		},
		"security": gin.H{
			"admin_pin": defaultVal("admin_pin", h.cfg.AdminPIN),
			"admin_key": defaultVal("admin_key", h.cfg.AdminKey),
		},
	})
}

// UpdateQRISConfig updates ShopeePay token and QRIS string
func (h *AdminHandler) UpdateQRISConfig(c *gin.Context) {
	var req struct {
		ShopeeToken string `json:"shopee_token"`
		QRISString  string `json:"qris_string"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ShopeeToken != "" {
		nowStr := time.Now().Format(time.RFC3339)
		_ = h.liveDB.SetConfig("shopee_token", strings.TrimSpace(req.ShopeeToken))
		_ = h.liveDB.SetConfig("shopee_token_updated_at", nowStr)
		h.cfg.UpdateShopeeToken(strings.TrimSpace(req.ShopeeToken))
	}

	if req.QRISString != "" {
		_ = h.liveDB.SetConfig("qris_string", strings.TrimSpace(req.QRISString))
		h.cfg.QRISString = strings.TrimSpace(req.QRISString)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Konfigurasi ShopeePay QRIS berhasil diperbarui"})
}

// UpdateBinanceConfig updates Binance Pay SAPI keys and Pay ID
func (h *AdminHandler) UpdateBinanceConfig(c *gin.Context) {
	var req struct {
		PayID       string `json:"pay_id"`
		AccountName string `json:"account_name"`
		UniQRURL    string `json:"uni_qr_url"`
		APIKey      string `json:"api_key"`
		SecretKey   string `json:"secret_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.PayID != "" {
		_ = h.liveDB.SetConfig("binance_pay_id", strings.TrimSpace(req.PayID))
	}
	if req.AccountName != "" {
		_ = h.liveDB.SetConfig("binance_account_name", strings.TrimSpace(req.AccountName))
	}
	if req.UniQRURL != "" {
		_ = h.liveDB.SetConfig("binance_uni_qr_url", strings.TrimSpace(req.UniQRURL))
	}
	if req.APIKey != "" {
		_ = h.liveDB.SetConfig("binance_api_key", strings.TrimSpace(req.APIKey))
	}
	if req.SecretKey != "" {
		_ = h.liveDB.SetConfig("binance_secret_key", strings.TrimSpace(req.SecretKey))
	}

	h.cfg.UpdateBinanceConfig(
		strings.TrimSpace(req.PayID),
		strings.TrimSpace(req.AccountName),
		strings.TrimSpace(req.UniQRURL),
		strings.TrimSpace(req.APIKey),
		strings.TrimSpace(req.SecretKey),
	)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Konfigurasi Binance Pay berhasil disimpan"})
}

// UpdateUSDTConfig updates receiver wallet and RPC endpoints
func (h *AdminHandler) UpdateUSDTConfig(c *gin.Context) {
	var req struct {
		ReceiverWallet string `json:"receiver_wallet_address"`
		BSCRPCURLs     string `json:"bsc_rpc_urls"`
		Confirmations  string `json:"required_confirmations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ReceiverWallet != "" {
		_ = h.liveDB.SetConfig("receiver_wallet_address", strings.TrimSpace(req.ReceiverWallet))
	}
	if req.BSCRPCURLs != "" {
		_ = h.liveDB.SetConfig("bsc_rpc_urls", strings.TrimSpace(req.BSCRPCURLs))
	}
	if req.Confirmations != "" {
		_ = h.liveDB.SetConfig("required_confirmations", strings.TrimSpace(req.Confirmations))
	}

	var rpcs []string
	if req.BSCRPCURLs != "" {
		for _, u := range strings.Split(req.BSCRPCURLs, ",") {
			if tu := strings.TrimSpace(u); tu != "" {
				rpcs = append(rpcs, tu)
			}
		}
	}
	confs := 2
	if req.Confirmations != "" {
		if c, err := strconv.Atoi(req.Confirmations); err == nil && c > 0 {
			confs = c
		}
	}
	h.cfg.UpdateUSDTConfig(strings.TrimSpace(req.ReceiverWallet), rpcs, confs)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Konfigurasi USDT BEP-20 berhasil disimpan"})
}

// UpdateSecurityConfig updates Admin PIN and Admin Key
func (h *AdminHandler) UpdateSecurityConfig(c *gin.Context) {
	var req struct {
		AdminPIN string `json:"admin_pin"`
		AdminKey string `json:"admin_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.AdminPIN != "" {
		_ = h.liveDB.SetConfig("admin_pin", strings.TrimSpace(req.AdminPIN))
		h.cfg.AdminPIN = strings.TrimSpace(req.AdminPIN)
	}
	if req.AdminKey != "" {
		_ = h.liveDB.SetConfig("admin_key", strings.TrimSpace(req.AdminKey))
		h.cfg.AdminKey = strings.TrimSpace(req.AdminKey)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Keamanan sistem berhasil diperbarui"})
}

// TestBinance tests SAPI connectivity
func (h *AdminHandler) TestBinance(c *gin.Context) {
	offset, err := h.binanceEng.SyncServerTime("")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Koneksi ke Binance SAPI gagal: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Koneksi ke Binance SAPI berhasil!",
		"clock_offset": fmt.Sprintf("%d ms", offset),
	})
}

// TestUSDT tests BSC RPC node latency and fetches latest block
func (h *AdminHandler) TestUSDT(c *gin.Context) {
	start := time.Now()
	block, rpcURL, err := h.usdtEngine.GetLatestBlockNumber(c.Request.Context())
	latency := time.Since(start).Milliseconds()

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Koneksi BSC RPC gagal: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Koneksi BSC RPC aktif dan normal!",
		"latest_block": block,
		"active_rpc":   rpcURL,
		"latency_ms":   latency,
	})
}

// GetStats returns aggregate stats
func (h *AdminHandler) GetStats(c *gin.Context) {
	isSandbox := c.Query("sandbox") == "true"
	targetDB := h.liveDB
	if isSandbox && h.sandboxDB != nil {
		targetDB = h.sandboxDB
	}

	stats, err := targetDB.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"qris":    stats.QRIS,
		"binance": stats.Binance,
		"usdt":    stats.USDT,
	})
}

// GetOrders returns orders with filtering
func (h *AdminHandler) GetOrders(c *gin.Context) {
	channel := c.Query("channel")
	status := c.Query("status")
	isSandbox := c.Query("sandbox") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	targetDB := h.liveDB
	if isSandbox && h.sandboxDB != nil {
		targetDB = h.sandboxDB
	}

	orders, err := targetDB.ListOrders(channel, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    orders,
	})
}

// GetApps lists merchant apps
func (h *AdminHandler) GetApps(c *gin.Context) {
	apps, err := h.liveDB.ListApps()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enhance with sandbox key info
	type AppItem struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		LiveAPIKey    string `json:"live_api_key"`
		SandboxAPIKey string `json:"sandbox_api_key"`
		WebhookURL    string `json:"webhook_url"`
		WebhookSecret string `json:"webhook_secret"`
		IsActive      bool   `json:"is_active"`
		CreatedAt     string `json:"created_at"`
	}

	var result []AppItem
	for _, a := range apps {
		sandKey := ""
		if h.sandboxDB != nil {
			if sApp, err := h.sandboxDB.GetAppByID(a.ID); err == nil && sApp != nil {
				sandKey = sApp.APIKey
			}
		}
		result = append(result, AppItem{
			ID:            a.ID,
			Name:          a.Name,
			LiveAPIKey:    a.APIKey,
			SandboxAPIKey: sandKey,
			WebhookURL:    a.WebhookURL,
			WebhookSecret: a.WebhookSecret,
			IsActive:      a.IsActive,
			CreatedAt:     a.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// CreateApp creates a new merchant app with live and sandbox keys
func (h *AdminHandler) CreateApp(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		WebhookURL string `json:"webhook_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	whSecret := database.GenerateRandomID("whsec")
	liveApp, err := h.liveDB.CreateApp(req.Name, req.WebhookURL, whSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create app in live DB: " + err.Error()})
		return
	}

	sandKey := ""
	if h.sandboxDB != nil {
		sandKey = database.GenerateRandomID("ak_test")
		_, _ = h.sandboxDB.CreateAppWithExplicit(liveApp.ID, req.Name, sandKey, req.WebhookURL, whSecret)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":              liveApp.ID,
			"name":            liveApp.Name,
			"live_api_key":    liveApp.APIKey,
			"sandbox_api_key": sandKey,
			"webhook_url":     liveApp.WebhookURL,
			"webhook_secret":  whSecret,
		},
	})
}

// DeleteApp removes an app
func (h *AdminHandler) DeleteApp(c *gin.Context) {
	id := c.Param("id")
	_ = h.liveDB.DeleteApp(id)
	if h.sandboxDB != nil {
		_ = h.sandboxDB.DeleteApp(id)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "App deleted successfully"})
}

// GetUnmatched lists unmatched mutations
func (h *AdminHandler) GetUnmatched(c *gin.Context) {
	channel := c.Query("channel")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	list, err := h.liveDB.ListUnmatched(channel, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// ResolveUnmatched performs CEX Fee Rescue
func (h *AdminHandler) ResolveUnmatched(c *gin.Context) {
	var req struct {
		UnmatchedID string `json:"unmatched_id" binding:"required"`
		OrderID     string `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.liveDB.GetOrder(req.OrderID)
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order tidak ditemukan"})
		return
	}

	if err := h.liveDB.ResolveUnmatched(req.UnmatchedID, req.OrderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memasangkan mutasi: " + err.Error()})
		return
	}

	// Mark order as paid
	if order.Status != model.StatusPaid {
		order.Status = model.StatusPaid
		order.PaidAt = time.Now().UnixMilli()
		_ = h.liveDB.MarkOrderPaid(order.ID, order.TxID, order.TxHash, order.FromInfo, order.BlockNumber)
		go h.webhook.DispatchOrder(h.liveDB, order)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Mutasi berhasil dipasangkan ke order dan webhook dikirim.",
	})
}
