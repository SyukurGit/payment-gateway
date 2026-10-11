package handler

import (
	"crypto/subtle"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"tokoasli/internal/config"
	"tokoasli/internal/database"
	"tokoasli/internal/model"
	"tokoasli/internal/service"
	"tokoasli/internal/util"
)

type AdminHandler struct {
	db             *database.DB
	cfg            *config.Config
	emailService   *service.EmailService
	sessionManager *service.SessionManager
	sseHub         *service.SSEHub
}

func NewAdminHandler(db *database.DB, cfg *config.Config, emailService *service.EmailService, sessionManager *service.SessionManager, sseHub *service.SSEHub) *AdminHandler {
	return &AdminHandler{
		db:             db,
		cfg:            cfg,
		emailService:   emailService,
		sessionManager: sessionManager,
		sseHub:         sseHub,
	}
}

// Login validates admin password and returns cryptographically secure session token
func (h *AdminHandler) Login(c *gin.Context) {
	var req model.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Password wajib diisi"})
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.cfg.AdminPassword)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Password admin salah"})
		return
	}

	sessionToken := h.sessionManager.CreateSession()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   sessionToken,
	})
}

// Logout invalidates the active admin session token
func (h *AdminHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		h.sessionManager.RevokeSession(parts[1])
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logout berhasil"})
}

// GetStats returns high-level business analytics for dashboard
func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.db.GetAdminStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal menghitung statistik"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// ListProducts returns all products for admin management
func (h *AdminHandler) ListProducts(c *gin.Context) {
	products, err := h.db.ListAllProducts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengambil daftar produk"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": products})
}

// CreateProduct adds a new product to catalog
func (h *AdminHandler) CreateProduct(c *gin.Context) {
	var req model.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Input tidak valid. Periksa nama, harga, dan tipe pengiriman."})
		return
	}

	p := &model.Product{
		ID:           util.NewProductID(),
		Name:         req.Name,
		Category:     req.Category,
		Price:        req.Price,
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		DeliveryType: req.DeliveryType,
		FixedContent: req.FixedContent,
		IsActive:     req.IsActive,
	}

	if p.Category == "" {
		p.Category = "Umum"
	}

	if err := h.db.CreateProduct(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal menyimpan produk"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": p})
}

// UpdateProduct modifies an existing product
func (h *AdminHandler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Input tidak valid"})
		return
	}

	p := &model.Product{
		ID:           id,
		Name:         req.Name,
		Category:     req.Category,
		Price:        req.Price,
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		DeliveryType: req.DeliveryType,
		FixedContent: req.FixedContent,
		IsActive:     req.IsActive,
	}

	if p.Category == "" {
		p.Category = "Umum"
	}

	if err := h.db.UpdateProduct(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengupdate produk: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

// DeleteProduct removes or archives a product safely
func (h *AdminHandler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.DeleteProduct(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal menghapus produk: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Produk berhasil dihapus atau diarsipkan"})
}

// ToggleProductStatus toggles the active/inactive state of a product directly
func (h *AdminHandler) ToggleProductStatus(c *gin.Context) {
	id := c.Param("id")
	p, err := h.db.GetProduct(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Produk tidak ditemukan"})
		return
	}

	p.IsActive = !p.IsActive
	if err := h.db.UpdateProduct(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengubah status produk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

// AddStocks bulk inserts serial keys / account lines into pool
func (h *AdminHandler) AddStocks(c *gin.Context) {
	id := c.Param("id")
	var req model.AddStocksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Data baris stok wajib diisi"})
		return
	}

	count, err := h.db.AddStocks(id, req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal memasukkan stok: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil menambahkan stok",
		"added":   count,
	})
}

// ListProductStocks gets detailed stock items and breakdown for a product
func (h *AdminHandler) ListProductStocks(c *gin.Context) {
	id := c.Param("id")
	summary, err := h.db.GetProductStockSummary(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengambil data stok"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// DeleteStockItem removes a single unused stock item
func (h *AdminHandler) DeleteStockItem(c *gin.Context) {
	stockIDStr := c.Param("stock_id")
	stockID, err := strconv.ParseInt(stockIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ID stok tidak valid"})
		return
	}

	if err := h.db.DeleteStockItem(stockID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Item stok berhasil dihapus"})
}

// ListOrders returns recent orders for admin table with full pagination metadata
func (h *AdminHandler) ListOrders(c *gin.Context) {
	status := c.DefaultQuery("status", "ALL")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	orders, err := h.db.ListOrders(status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengambil daftar pesanan"})
		return
	}

	totalCount, _ := h.db.CountOrders(status)
	totalPages := (totalCount + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    orders,
		"pagination": gin.H{
			"total":       totalCount,
			"page":        page,
			"limit":       limit,
			"total_pages": totalPages,
		},
	})
}

// MarkOrderPaid allows admin to manually confirm an order as PAID
func (h *AdminHandler) MarkOrderPaid(c *gin.Context) {
	id := c.Param("id")

	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	if order.Status == model.OrderStatusPaid {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Pesanan sudah berstatus lunas sebelumnya"})
		return
	}

	product, err := h.db.GetProduct(order.ProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Data produk tidak ditemukan"})
		return
	}

	deliveryContent, err := h.db.CommitStock(order.ID, product.ID, product.DeliveryType, product.FixedContent)
	if err != nil {
		deliveryContent = "STOK KOSONG (Diproses manual oleh Admin)"
	}

	if err := h.db.MarkOrderPaid(order.ID, deliveryContent, "MANUAL-APPROVAL-BY-ADMIN"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengupdate status pesanan"})
		return
	}

	order.Status = model.OrderStatusPaid
	order.DeliveryContent = deliveryContent

	// Broadcast via SSE to any open tabs
	if h.sseHub != nil {
		h.sseHub.Broadcast(order.ID, gin.H{
			"order_id": order.ID,
			"status":   model.OrderStatusPaid,
		})
	}

	// Send confirmation email
	h.emailService.SendReceiptAsync(order)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Pesanan berhasil ditandai Lunas secara manual dan produk telah diserahkan.",
	})
}

// ExportOrdersCSV exports order history to CSV
func (h *AdminHandler) ExportOrdersCSV(c *gin.Context) {
	status := c.DefaultQuery("status", "ALL")
	orders, err := h.db.ExportOrders(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal mengambil data ekspor"})
		return
	}

	filename := fmt.Sprintf("orders_export_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write UTF-8 BOM so Excel opens it correctly
	_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))

	// Headers
	_ = writer.Write([]string{
		"Invoice ID", "Tanggal Dibuat", "Nama Produk", "Email Pembeli", "No. WhatsApp",
		"Harga Asli (Rp)", "Kode Unik", "Total Bayar (Rp)", "Status",
		"PaymentG Order ID", "Tanggal Lunas",
	})

	for _, o := range orders {
		paidAt := ""
		if o.PaidAt != nil {
			paidAt = *o.PaidAt
		}
		_ = writer.Write([]string{
			o.ID,
			o.CreatedAt,
			o.ProductName,
			o.CustomerEmail,
			o.CustomerPhone,
			strconv.FormatInt(o.OriginalPrice, 10),
			strconv.Itoa(o.UniqueCode),
			strconv.FormatInt(o.TotalAmount, 10),
			o.Status,
			o.PaymentGOrderID,
			paidAt,
		})
	}
}

// ResendEmail manually triggers email receipt resend for a paid order
func (h *AdminHandler) ResendEmail(c *gin.Context) {
	id := c.Param("id")
	order, err := h.db.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Pesanan tidak ditemukan"})
		return
	}

	if order.Status != model.OrderStatusPaid {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email hanya dapat dikirim ulang untuk pesanan yang sudah lunas (PAID)"})
		return
	}

	h.emailService.SendReceiptAsync(order)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Email tanda terima telah dikirimkan ulang"})
}

// UploadImage handles image file uploads for products
func (h *AdminHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File gambar tidak ditemukan"})
		return
	}

	// Limit 5MB
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Ukuran gambar maksimal 5MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Format gambar harus JPG, PNG, WEBP, atau GIF"})
		return
	}

	uploadsDir := filepath.Join(h.cfg.DataDir, "uploads")
	_ = os.MkdirAll(uploadsDir, 0755)

	filename := fmt.Sprintf("img_%d_%s%s", time.Now().Unix(), util.NewProductID()[5:], ext)
	dst := filepath.Join(uploadsDir, filename)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Gagal menyimpan file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"url":     "/uploads/" + filename,
	})
}
