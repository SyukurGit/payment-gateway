package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"multipay/internal/config"
	"multipay/internal/database"
	"multipay/internal/handler"
	"multipay/internal/middleware"
	"multipay/internal/service"
)

func main() {
	log.Println("=====================================================")
	log.Println("🚀 Starting Multipay Gateway (QRIS + Binance + USDT)")
	log.Println("=====================================================")

	// 1. Load Config
	cfg := config.LoadConfig()

	// 2. Initialize SQLite Databases (Live & Sandbox)
	liveDB, err := database.InitDB("./data/payment.db", false, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Live database: %v", err)
	}
	defer liveDB.Close()

	sandboxDB, err := database.InitDB("./data/sandbox/payment.db", true, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Sandbox database: %v", err)
	}
	defer sandboxDB.Close()

	// 3. Initialize Services & Engines
	qrisStr, _ := liveDB.GetConfig("qris_string")
	if qrisStr == "" && cfg.QRISString != "" {
		qrisStr = cfg.QRISString
		_ = liveDB.SetConfig("qris_string", qrisStr)
	}
	if qrisStr == "" {
		if _, err := os.Stat("qris-shopee.jpeg"); err == nil {
			if decoded, err := service.DecodeQRImage("qris-shopee.jpeg"); err == nil && decoded != "" {
				qrisStr = decoded
				_ = liveDB.SetConfig("qris_string", qrisStr)
				log.Println("📱 Successfully decoded and registered QRIS from qris-shopee.jpeg")
			}
		}
	}

	webhookDispatcher := service.NewWebhookDispatcher(cfg)
	qrisEngine := service.NewQRISEngine(liveDB, qrisStr)
	binanceEngine := service.NewBinanceEngine(liveDB, webhookDispatcher)
	usdtEngine := service.NewUSDTEngine(liveDB, webhookDispatcher)

	// 4. Background Workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerManager := service.NewWorkerManager(
		cfg,
		liveDB,
		sandboxDB,
		qrisEngine,
		binanceEngine,
		usdtEngine,
		webhookDispatcher,
	)
	workerManager.Start(ctx)

	// 5. Initialize Handlers
	qrisHandler := handler.NewQRISHandler(qrisEngine, webhookDispatcher, cfg)
	binanceHandler := handler.NewBinanceHandler(binanceEngine, webhookDispatcher, cfg)
	usdtHandler := handler.NewUSDTHandler(usdtEngine, webhookDispatcher, cfg)
	adminHandler := handler.NewAdminHandler(
		liveDB,
		sandboxDB,
		cfg,
		qrisEngine,
		binanceEngine,
		usdtEngine,
		webhookDispatcher,
	)
	healthHandler := handler.NewHealthHandler(liveDB, cfg, qrisEngine, binanceEngine, usdtEngine)

	// 6. Router Setup
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	// Dynamic Base URL middleware
	r.Use(func(c *gin.Context) {
		proto := c.GetHeader("X-Forwarded-Proto")
		if proto == "" {
			if c.Request.TLS != nil {
				proto = "https"
			} else {
				proto = "http"
			}
		}
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		c.Set("base_url", fmt.Sprintf("%s://%s", proto, host))
		c.Next()
	})

	// Static Dashboard & Root Redirect
	r.Static("/dashboard", "./dashboard")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/dashboard/")
	})

	// Health Check
	r.GET("/api/health", healthHandler.HealthCheck)

	// -------------------------------------------------------------
	// 📱 ShopeePay QRIS Channel Routes
	// -------------------------------------------------------------
	// Public QR Images
	r.GET("/qris/api/orders/:id/qr.png", func(c *gin.Context) { qrisHandler.GetQRImage(c, liveDB) })
	r.GET("/qris/api/sandbox/orders/:id/qr.png", func(c *gin.Context) { qrisHandler.GetQRImage(c, sandboxDB) })

	// Live QRIS
	qrisLive := r.Group("/qris/api/orders", middleware.ClientAuth(liveDB, sandboxDB, false))
	{
		qrisLive.POST("", qrisHandler.CreateOrder)
		qrisLive.GET("/:id", qrisHandler.GetOrder)
		qrisLive.POST("/:id/check", qrisHandler.CheckOrder)
		qrisLive.POST("/:id/cancel", qrisHandler.CancelOrder)
	}

	// Sandbox QRIS
	qrisSandbox := r.Group("/qris/api/sandbox/orders", middleware.ClientAuth(liveDB, sandboxDB, true))
	{
		qrisSandbox.POST("", qrisHandler.CreateOrder)
		qrisSandbox.GET("/:id", qrisHandler.GetOrder)
		qrisSandbox.POST("/:id/check", qrisHandler.CheckOrder)
		qrisSandbox.POST("/:id/cancel", qrisHandler.CancelOrder)
		qrisSandbox.POST("/:id/pay", qrisHandler.SimulatePay)
	}

	// -------------------------------------------------------------
	// 🟡 Binance Pay Channel Routes
	// -------------------------------------------------------------
	// Public QR Images
	r.GET("/binance/api/orders/:id/qr.png", func(c *gin.Context) { binanceHandler.GetQRImage(c, liveDB) })
	r.GET("/binance/api/sandbox/orders/:id/qr.png", func(c *gin.Context) { binanceHandler.GetQRImage(c, sandboxDB) })

	// Live Binance
	binanceLive := r.Group("/binance/api/orders", middleware.ClientAuth(liveDB, sandboxDB, false))
	{
		binanceLive.POST("", binanceHandler.CreateOrder)
		binanceLive.GET("/:id", binanceHandler.GetOrder)
		binanceLive.POST("/:id/check", binanceHandler.CheckOrder)
		binanceLive.POST("/:id/cancel", binanceHandler.CancelOrder)
	}

	// Sandbox Binance
	binanceSandbox := r.Group("/binance/api/sandbox/orders", middleware.ClientAuth(liveDB, sandboxDB, true))
	{
		binanceSandbox.POST("", binanceHandler.CreateOrder)
		binanceSandbox.GET("/:id", binanceHandler.GetOrder)
		binanceSandbox.POST("/:id/check", binanceHandler.CheckOrder)
		binanceSandbox.POST("/:id/cancel", binanceHandler.CancelOrder)
		binanceSandbox.POST("/:id/pay", binanceHandler.SimulatePay)
	}

	// -------------------------------------------------------------
	// 🟢 USDT BEP-20 Channel Routes
	// -------------------------------------------------------------
	// Public QR Images
	r.GET("/usdt/api/orders/:id/qr.png", func(c *gin.Context) { usdtHandler.GetQRImage(c, liveDB) })
	r.GET("/usdt/api/sandbox/orders/:id/qr.png", func(c *gin.Context) { usdtHandler.GetQRImage(c, sandboxDB) })

	// Live USDT
	usdtLive := r.Group("/usdt/api/orders", middleware.ClientAuth(liveDB, sandboxDB, false))
	{
		usdtLive.POST("", usdtHandler.CreateOrder)
		usdtLive.GET("/:id", usdtHandler.GetOrder)
		usdtLive.POST("/:id/check", usdtHandler.CheckOrder)
		usdtLive.POST("/:id/cancel", usdtHandler.CancelOrder)
	}

	// Sandbox USDT
	usdtSandbox := r.Group("/usdt/api/sandbox/orders", middleware.ClientAuth(liveDB, sandboxDB, true))
	{
		usdtSandbox.POST("", usdtHandler.CreateOrder)
		usdtSandbox.GET("/:id", usdtHandler.GetOrder)
		usdtSandbox.POST("/:id/check", usdtHandler.CheckOrder)
		usdtSandbox.POST("/:id/cancel", usdtHandler.CancelOrder)
		usdtSandbox.POST("/:id/pay", usdtHandler.SimulatePay)
	}

	// -------------------------------------------------------------
	// ⚙️ Admin & Dashboard API Routes
	// -------------------------------------------------------------
	r.POST("/api/admin/login", middleware.PINRateLimiter(), adminHandler.LoginPIN)

	admin := r.Group("/api/admin", middleware.AdminAuth(liveDB, cfg))
	{
		admin.GET("/config", adminHandler.GetConfig)
		admin.PUT("/config/qris", adminHandler.UpdateQRISConfig)
		admin.PUT("/config/binance", adminHandler.UpdateBinanceConfig)
		admin.PUT("/config/usdt", adminHandler.UpdateUSDTConfig)
		admin.PUT("/config/security", adminHandler.UpdateSecurityConfig)

		admin.POST("/config/binance/test", adminHandler.TestBinance)
		admin.POST("/config/usdt/test", adminHandler.TestUSDT)

		admin.GET("/stats", adminHandler.GetStats)
		admin.GET("/orders", adminHandler.GetOrders)
		admin.GET("/apps", adminHandler.GetApps)
		admin.POST("/apps", adminHandler.CreateApp)
		admin.DELETE("/apps/:id", adminHandler.DeleteApp)

		admin.GET("/unmatched", adminHandler.GetUnmatched)
		admin.POST("/unmatched/resolve", adminHandler.ResolveUnmatched)
	}

	// 7. Start HTTP Server with Graceful Shutdown
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🌐 Multipay Server listening on port %d", cfg.Port)
		log.Printf("👉 Dashboard: http://localhost:%d/dashboard/", cfg.Port)
		log.Printf("👉 Health:    http://localhost:%d/api/health", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Listen error: %s", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down Multipay Gateway gracefully...")
	cancel() // Stop workers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}
	log.Println("✅ Multipay Gateway stopped cleanly.")
}
