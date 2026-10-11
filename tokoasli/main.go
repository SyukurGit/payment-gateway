package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"tokoasli/internal/config"
	"tokoasli/internal/database"
	"tokoasli/internal/handler"
	"tokoasli/internal/middleware"
	"tokoasli/internal/service"
)

func main() {
	// 1. Load configuration
	config.LoadDotEnv(".env")
	cfg := config.Load()

	log.Println("==================================================")
	log.Printf("[TokoAsli v2.0] Menjalankan server pada port %d", cfg.Port)
	log.Printf("[TokoAsli] PaymentG URL: %s", cfg.PaymentGURL)
	if cfg.ResendAPIKey == "" {
		log.Println("[TokoAsli] ℹ️  RESEND_API_KEY kosong -> Mode Fallback (Email dicetak ke console)")
	} else {
		log.Printf("[TokoAsli] ✓ Resend API aktif (Sender: %s)", cfg.ResendFromEmail)
	}
	if cfg.WhatsAppNumber != "" {
		log.Printf("[TokoAsli] ✓ WhatsApp CS aktif: %s", cfg.WhatsAppNumber)
	}
	log.Println("==================================================")

	// 2. Initialize database
	db, err := database.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("[TokoAsli] Database init failed: %v", err)
	}
	defer db.Close()
	log.Println("[TokoAsli] Database SQLite (WAL Mode) siap digunakan")

	// 3. Initialize services
	paymentgClient := service.NewPaymentGClient(cfg.PaymentGURL, cfg.PaymentGAPIKey, cfg.PaymentGSandbox)
	emailService := service.NewEmailService(cfg.ResendAPIKey, cfg.ResendFromEmail, cfg.StoreURL)
	sseHub := service.NewSSEHub()
	stockCleaner := service.NewStockCleaner(db, sseHub)
	sessionManager := service.NewSessionManager(24 * time.Hour)

	// 4. Start background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go stockCleaner.Start(ctx)
	log.Println("[TokoAsli] Background worker reservasi stok (Hold 15m) aktif")

	// 5. Initialize handlers
	publicHandler := handler.NewPublicHandler(db, cfg, paymentgClient)
	orderHandler := handler.NewOrderHandler(db, sseHub, paymentgClient, emailService)
	webhookHandler := handler.NewWebhookHandler(db, cfg, sseHub, emailService)
	adminHandler := handler.NewAdminHandler(db, cfg, emailService, sessionManager, sseHub)

	// 6. Setup Gin Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"app":     "TokoAsli",
			"version": "2.0.0",
		})
	})

	// Public Store API
	apiPublic := r.Group("/api")
	{
		apiPublic.GET("/config", publicHandler.GetStoreConfig)
		apiPublic.GET("/products", publicHandler.ListProducts)
		apiPublic.GET("/products/:id", publicHandler.GetProduct)
		// Anti-spam limiter: Max 5 pending orders per IP
		apiPublic.POST("/orders", middleware.IPLimiter(db, 5), publicHandler.CreateOrder)
		apiPublic.GET("/orders/:id", orderHandler.GetOrder)
		apiPublic.GET("/orders/:id/qr.png", orderHandler.GetQRImage)
		apiPublic.GET("/orders/:id/stream", orderHandler.StreamOrder)
		apiPublic.POST("/orders/:id/check", orderHandler.CheckOrderStatus)
		apiPublic.POST("/orders/:id/cancel", orderHandler.CancelOrder)
		apiPublic.POST("/orders/track", publicHandler.TrackOrder)
	}

	// Webhook endpoint (Receives payment confirmations from PaymentG)
	r.POST("/api/webhooks/paymentg", webhookHandler.HandlePaymentG)

	// Admin Auth
	r.POST("/api/admin/login", adminHandler.Login)
	r.POST("/api/admin/logout", adminHandler.Logout)

	// Admin API (Protected by SessionManager & master password fallback)
	apiAdmin := r.Group("/api/admin")
	apiAdmin.Use(middleware.AdminAuth(sessionManager, cfg.AdminPassword))
	{
		apiAdmin.GET("/stats", adminHandler.GetStats)
		apiAdmin.GET("/products", adminHandler.ListProducts)
		apiAdmin.POST("/products", adminHandler.CreateProduct)
		apiAdmin.PUT("/products/:id", adminHandler.UpdateProduct)
		apiAdmin.DELETE("/products/:id", adminHandler.DeleteProduct)
		apiAdmin.POST("/products/:id/stocks", adminHandler.AddStocks)
		apiAdmin.GET("/products/:id/stocks", adminHandler.ListProductStocks)
		apiAdmin.DELETE("/stocks/:stock_id", adminHandler.DeleteStockItem)
		apiAdmin.PATCH("/products/:id/toggle", adminHandler.ToggleProductStatus)
		apiAdmin.GET("/orders", adminHandler.ListOrders)
		apiAdmin.GET("/orders/export", adminHandler.ExportOrdersCSV)
		apiAdmin.POST("/orders/:id/mark-paid", adminHandler.MarkOrderPaid)
		apiAdmin.POST("/orders/:id/resend-email", adminHandler.ResendEmail)
		apiAdmin.POST("/upload", adminHandler.UploadImage)
	}

	// Serve Uploaded Product Images
	_ = os.MkdirAll(cfg.DataDir+"/uploads", 0755)
	r.Static("/uploads", cfg.DataDir+"/uploads")

	// Serve Frontend locally (Clean URL paths identical to Cloudflare Pages routing)
	r.Static("/assets", "./fe-toko/assets")
	r.StaticFile("/", "./fe-toko/index.html")
	r.StaticFile("/index.html", "./fe-toko/index.html")
	r.StaticFile("/order", "./fe-toko/order/index.html")
	r.StaticFile("/order/", "./fe-toko/order/index.html")
	r.StaticFile("/order/index.html", "./fe-toko/order/index.html")
	r.StaticFile("/product", "./fe-toko/product/index.html")
	r.StaticFile("/product/", "./fe-toko/product/index.html")
	r.StaticFile("/product/index.html", "./fe-toko/product/index.html")
	r.StaticFile("/admin", "./fe-toko/admin/index.html")
	r.StaticFile("/admin/", "./fe-toko/admin/index.html")
	r.StaticFile("/admin/index.html", "./fe-toko/admin/index.html")

	// 7. Start HTTP Server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("[TokoAsli] HTTP Server mendengarkan pada :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[TokoAsli] Server fatal error: %v", err)
		}
	}()

	// 8. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[TokoAsli] Mematikan server secara graceful...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[TokoAsli] Forced shutdown: %v", err)
	}

	log.Println("[TokoAsli] Server berhenti dengan sukses")
}
