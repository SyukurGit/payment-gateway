package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"multipay/internal/config"
	"multipay/internal/database"
	"multipay/internal/service"
)

type HealthHandler struct {
	liveDB     *database.DB
	cfg        *config.Config
	qrisEngine *service.QRISEngine
	binanceEng *service.BinanceEngine
	usdtEngine *service.USDTEngine
	startTime  time.Time
}

func NewHealthHandler(
	liveDB *database.DB,
	cfg *config.Config,
	qrisEngine *service.QRISEngine,
	binanceEng *service.BinanceEngine,
	usdtEngine *service.USDTEngine,
) *HealthHandler {
	return &HealthHandler{
		liveDB:     liveDB,
		cfg:        cfg,
		qrisEngine: qrisEngine,
		binanceEng: binanceEng,
		usdtEngine: usdtEngine,
		startTime:  time.Now(),
	}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	// 1. Check Shopee token
	shopeeToken, _ := h.liveDB.GetConfig("shopee_token")
	shopeeUpdated, _ := h.liveDB.GetConfig("shopee_token_updated_at")
	shopeeActive := false
	shopeeTTLHours := 0
	if shopeeToken != "" && shopeeUpdated != "" {
		if t, err := time.Parse(time.RFC3339, shopeeUpdated); err == nil {
			elapsed := time.Since(t)
			if elapsed < 24*time.Hour {
				shopeeActive = true
				shopeeTTLHours = 24 - int(elapsed.Hours())
			}
		}
	}

	// 2. Check Binance SAPI
	binanceConfigured := false
	binanceKey, _ := h.liveDB.GetConfig("binance_api_key")
	if binanceKey == "" {
		binanceKey = h.cfg.BinanceAPIKey
	}
	if binanceKey != "" {
		binanceConfigured = true
	}

	// 3. Check BSC RPC
	bscBlock := int64(0)
	activeRPC := ""
	rpcOK := false
	if block, rpc, err := h.usdtEngine.GetLatestBlockNumber(ctx); err == nil {
		bscBlock = block
		activeRPC = rpc
		rpcOK = true
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"uptime": time.Since(h.startTime).String(),
		"engines": gin.H{
			"qris": gin.H{
				"channel":       "ShopeePay Dynamic QRIS",
				"token_active":  shopeeActive,
				"ttl_hours":     shopeeTTLHours,
				"is_configured": shopeeToken != "",
			},
			"binance": gin.H{
				"channel":       "Binance Pay SAPI",
				"is_configured": binanceConfigured,
				"currency":      "USDT",
			},
			"usdt": gin.H{
				"channel":       "USDT BEP-20 BSC",
				"network":       "BNB Smart Chain (BEP-20)",
				"rpc_connected": rpcOK,
				"latest_block":  bscBlock,
				"active_rpc":    activeRPC,
			},
		},
	})
}
