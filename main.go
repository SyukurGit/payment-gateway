package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"paymentg/internal/config"
	"paymentg/internal/database"
	"paymentg/internal/handler"
	"paymentg/internal/middleware"
	"paymentg/internal/service"
)

func main() {
	// 1. Load config
	config.LoadDotEnv(".env")
	cfg := config.Load()

	log.Printf("[PaymentG] Starting on port %d", cfg.Port)
	log.Printf("[PaymentG] Admin key: %s...", cfg.AdminKey[:8])

	// 2. Init database (Production)
	db, err := database.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("[PaymentG] Database init failed: %v", err)
	}
	defer db.Close()

	// 2b. Init database (Isolated Sandbox)
	sandboxDataDir := filepath.Join(cfg.DataDir, "sandbox")
	sandboxDB, err := database.New(sandboxDataDir)
	if err != nil {
		log.Printf("[PaymentG] WARNING: Sandbox DB init failed: %v", err)
	} else {
		defer sandboxDB.Close()
		log.Println("[PaymentG] Sandbox Database initialized at", sandboxDataDir)
	}

	// 3. Store initial token in DB if provided via env
	if cfg.ShopeeToken != "" {
		db.SetConfig("shopee_token", cfg.ShopeeToken)
		db.SetConfig("shopee_token_updated_at", time.Now().UTC().Format(time.RFC3339))
		log.Println("[PaymentG] ShopeePay token loaded from env")
	}

	// 4. Store QRIS string in DB if provided via env
	if cfg.QRISString != "" {
		db.SetConfig("qris_string", cfg.QRISString)
		log.Println("[PaymentG] QRIS string loaded from env")
	}

	// 5. Get QRIS string (from DB or env)
	qrisString, _ := db.GetConfig("qris_string")
	if qrisString == "" {
		if _, err := os.Stat("qris-shopee.jpeg"); err == nil {
			log.Println("[PaymentG] Decoding QRIS from qris-shopee.jpeg...")
			if decoded, err := service.DecodeQRImage("qris-shopee.jpeg"); err == nil {
				qrisString = decoded
				db.SetConfig("qris_string", qrisString)
				log.Println("[PaymentG] Successfully decoded and saved QRIS string from qris-shopee.jpeg")
			} else {
				log.Printf("[PaymentG] Failed to decode qris-shopee.jpeg: %v", err)
			}
		}
	}

	if qrisString == "" {
		log.Println("[PaymentG] WARNING: No QRIS string configured. Set QRIS_STRING in .env or place qris-shopee.jpeg in working dir")
	}

	// 6. Init services
	qrisService := service.NewQRISService(qrisString)
	shopeeClient := service.NewShopeeClient()
	webhookService := service.NewWebhookService(db)
	matcher := service.NewMatcher(db, webhookService)
	poller := service.NewPoller(db, shopeeClient, matcher, cfg.PollIntervalSeconds)
	expiryService := service.NewExpiryService(db)

	// 7. Init handlers
	orderHandler := handler.NewOrderHandler(db, qrisService, cfg)
	adminHandler := handler.NewAdminHandler(db, cfg)
	healthHandler := handler.NewHealthHandler(db, poller)

	var sandboxHandler *handler.SandboxHandler
	if sandboxDB != nil {
		sandboxWebhookService := service.NewWebhookService(sandboxDB)
		sandboxHandler = handler.NewSandboxHandler(sandboxDB, qrisService, cfg, sandboxWebhookService, sandboxDataDir)
	}

	// 8. Start background goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.Start(ctx)
	go expiryService.Start(ctx)

	log.Println("[PaymentG] Background poller started")
	log.Println("[PaymentG] Background expiry checker started")

	// 9. Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware()) // Simple CORS

	// Serve static dashboard if directory exists
	if _, err := os.Stat("dashboard"); err == nil {
		r.Static("/dashboard", "./dashboard")
		rootRedirect := func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/dashboard/")
		}
		r.GET("/", rootRedirect)
		r.HEAD("/", rootRedirect)

		// Dedicated sandbox page route
		sandboxPage := func(c *gin.Context) {
			c.File("./dashboard/sandbox.html")
		}
		r.GET("/sandbox", sandboxPage)
		r.HEAD("/sandbox", sandboxPage)
		r.GET("/sandbox/", sandboxPage)
		r.HEAD("/sandbox/", sandboxPage)
	}

	// Public routes
	r.GET("/api/health", healthHandler.HealthCheck)
	r.HEAD("/api/health", healthHandler.HealthCheck)
	r.GET("/api/orders/:id/qr.png", orderHandler.GetQRImage) // public so <img> can load from any domain
	r.HEAD("/api/orders/:id/qr.png", orderHandler.GetQRImage)
	r.POST("/api/auth/pin", adminHandler.VerifyPIN) // public PIN gate for dashboard

	// Client API routes (X-API-Key auth)
	clientAPI := r.Group("/api")
	clientAPI.Use(middleware.APIKeyAuth(db))
	{
		clientAPI.POST("/orders", orderHandler.CreateOrder)
		clientAPI.GET("/orders/:id", orderHandler.GetOrder)
		clientAPI.POST("/orders/:id/check", orderHandler.CheckOrder)
		clientAPI.POST("/orders/:id/cancel", orderHandler.CancelOrder)
	}

	// Admin API routes (X-Admin-Key auth)
	adminAPI := r.Group("/api")
	adminAPI.Use(middleware.AdminAuth(cfg.AdminKey))
	{
		adminAPI.PUT("/config/token", adminHandler.UpdateToken)
		adminAPI.POST("/token/refresh", adminHandler.TriggerTokenRefresh)
		adminAPI.GET("/orders", adminHandler.ListOrders)
		adminAPI.GET("/stats", adminHandler.GetStats)
		adminAPI.POST("/apps", adminHandler.CreateApp)
		adminAPI.GET("/apps", adminHandler.ListApps)
		adminAPI.DELETE("/apps/:id", adminHandler.DeleteApp)
	}

	// 9b. Sandbox API routes (Isolated testing environment)
	if sandboxHandler != nil && sandboxDB != nil {
		r.GET("/api/sandbox/orders/:id/qr.png", sandboxHandler.GetQRImage)
		r.HEAD("/api/sandbox/orders/:id/qr.png", sandboxHandler.GetQRImage)
		r.POST("/api/sandbox/auth/pin", sandboxHandler.VerifyPIN)

		sbxClient := r.Group("/api/sandbox")
		sbxClient.Use(middleware.APIKeyAuth(sandboxDB))
		{
			sbxClient.POST("/orders", sandboxHandler.CreateOrder)
			sbxClient.GET("/orders/:id", sandboxHandler.GetOrder)
			sbxClient.POST("/orders/:id/check", sandboxHandler.CheckOrder)
			sbxClient.POST("/orders/:id/cancel", sandboxHandler.CancelOrder)
		}

		sbxAdmin := r.Group("/api/sandbox")
		sbxAdmin.Use(middleware.AdminAuth(cfg.AdminKey))
		{
			sbxAdmin.GET("/orders", sandboxHandler.ListOrders)
			sbxAdmin.POST("/orders/:id/pay", sandboxHandler.SimulatePay)
			sbxAdmin.GET("/stats", sandboxHandler.GetStats)
			sbxAdmin.POST("/apps", sandboxHandler.CreateApp)
			sbxAdmin.GET("/apps", sandboxHandler.ListApps)
			sbxAdmin.DELETE("/apps/:id", sandboxHandler.DeleteApp)
		}
	}

	// 10. Start HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("[PaymentG] HTTP server listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[PaymentG] Server error: %v", err)
		}
	}()

	// 11. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[PaymentG] Shutting down...")
	cancel() // Stop background goroutines

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[PaymentG] Forced shutdown: %v", err)
	}

	log.Println("[PaymentG] Server stopped")
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Admin-Key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
