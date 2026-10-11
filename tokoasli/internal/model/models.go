package model

// Status constants for Orders
const (
	OrderStatusPending   = "PENDING"
	OrderStatusPaid      = "PAID"
	OrderStatusExpired   = "EXPIRED"
	OrderStatusCancelled = "CANCELLED"
)

// Delivery types for Products
const (
	DeliveryTypePool  = "pool"  // Individual serial keys/accounts (stock decreases)
	DeliveryTypeFixed = "fixed" // Fixed link/file download for all buyers
)

// Stock status constants
const (
	StockAvailable = 0 // Available in pool
	StockSold      = 1 // Successfully bought & paid
	StockHeld      = 2 // Temporarily held during pending checkout (15 mins)
)

// Product represents an item for sale in the catalog
type Product struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Price        int64  `json:"price"`
	Description  string `json:"description"`
	ImageURL     string `json:"image_url"`
	DeliveryType string `json:"delivery_type"`
	FixedContent string `json:"fixed_content,omitempty"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	StockCount   int    `json:"stock_count"` // Computed: available items in pool (or 999 if fixed)
}

// ProductStock represents an individual serial key / account row in the pool
type ProductStock struct {
	ID        int64   `json:"id"`
	ProductID string  `json:"product_id"`
	Content   string  `json:"content"`
	IsUsed    int     `json:"is_used"`
	OrderID   *string `json:"order_id,omitempty"`
	HeldUntil *string `json:"held_until,omitempty"`
	CreatedAt string  `json:"created_at"`
	UsedAt    *string `json:"used_at,omitempty"`
}

// StoreOrder represents a customer order in the store
type StoreOrder struct {
	ID              string  `json:"id"`
	SecretToken     string  `json:"secret_token"`
	ProductID       string  `json:"product_id"`
	ProductName     string  `json:"product_name"`
	CustomerEmail   string  `json:"customer_email"`
	CustomerPhone   string  `json:"customer_phone,omitempty"`
	OriginalPrice   int64   `json:"original_price"`
	UniqueCode      int     `json:"unique_code"`
	TotalAmount     int64   `json:"total_amount"`
	Status          string  `json:"status"`
	DeliveryContent string  `json:"delivery_content,omitempty"`
	PaymentGOrderID string  `json:"paymentg_order_id,omitempty"`
	QRString        string  `json:"qr_string,omitempty"`
	QRURL           string  `json:"qr_url,omitempty"`
	ClientIP        string  `json:"client_ip"`
	CreatedAt       string  `json:"created_at"`
	PaidAt          *string `json:"paid_at,omitempty"`
	ExpiresAt       string  `json:"expires_at"`
}

// --- Request / Response DTOs ---

// CreateOrderRequest payload from public checkout
type CreateOrderRequest struct {
	ProductID     string `json:"product_id" binding:"required"`
	CustomerEmail string `json:"customer_email" binding:"required,email"`
	CustomerPhone string `json:"customer_phone"`
}

// CreateOrderResponse payload returned to buyer
type CreateOrderResponse struct {
	OrderID       string `json:"order_id"`
	SecretToken   string `json:"secret_token"`
	ProductName   string `json:"product_name"`
	CustomerEmail string `json:"customer_email"`
	CustomerPhone string `json:"customer_phone,omitempty"`
	OriginalPrice int64  `json:"original_price"`
	UniqueCode    int    `json:"unique_code"`
	TotalAmount   int64  `json:"total_amount"`
	QRString      string `json:"qr_string"`
	QRURL         string `json:"qr_url"`
	ExpiresAt     string `json:"expires_at"`
	Status        string `json:"status"`
}

// OrderPublicResponse payload for checking order status & claiming product
type OrderPublicResponse struct {
	ID              string  `json:"id"`
	ProductName     string  `json:"product_name"`
	CustomerEmail   string  `json:"customer_email"`
	CustomerPhone   string  `json:"customer_phone,omitempty"`
	OriginalPrice   int64   `json:"original_price"`
	UniqueCode      int     `json:"unique_code"`
	TotalAmount     int64   `json:"total_amount"`
	Status          string  `json:"status"`
	QRString        string  `json:"qr_string,omitempty"`
	QRURL           string  `json:"qr_url,omitempty"`
	ExpiresAt       string  `json:"expires_at"`
	DeliveryContent string  `json:"delivery_content,omitempty"` // only revealed if PAID and SecretToken matched
	PaidAt          *string `json:"paid_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

// AdminLoginRequest for dashboard access
type AdminLoginRequest struct {
	Password string `json:"password" binding:"required"`
}

// AdminLoginResponse returns bearer token
type AdminLoginResponse struct {
	Token string `json:"token"`
}

// AdminStatsResponse for dashboard metrics
type AdminStatsResponse struct {
	TotalRevenue       int64 `json:"total_revenue"`
	TotalPaidOrders    int   `json:"total_paid_orders"`
	TotalPendingOrders int   `json:"total_pending_orders"`
	TotalProducts      int   `json:"total_products"`
}

// CreateProductRequest for adding products
type CreateProductRequest struct {
	Name         string `json:"name" binding:"required"`
	Category     string `json:"category"`
	Price        int64  `json:"price" binding:"required,gt=0"`
	Description  string `json:"description"`
	ImageURL     string `json:"image_url"`
	DeliveryType string `json:"delivery_type" binding:"required,oneof=pool fixed"`
	FixedContent string `json:"fixed_content"`
	IsActive     bool   `json:"is_active"`
}

// UpdateProductRequest for editing products
type UpdateProductRequest struct {
	Name         string `json:"name" binding:"required"`
	Category     string `json:"category"`
	Price        int64  `json:"price" binding:"required,gt=0"`
	Description  string `json:"description"`
	ImageURL     string `json:"image_url"`
	DeliveryType string `json:"delivery_type" binding:"required,oneof=pool fixed"`
	FixedContent string `json:"fixed_content"`
	IsActive     bool   `json:"is_active"`
}

// AddStocksRequest for uploading lines into pool
type AddStocksRequest struct {
	Lines []string `json:"lines" binding:"required"`
}

// PaymentGWebhookPayload sent by PaymentG on payment success
type PaymentGWebhookPayload struct {
	Event          string `json:"event"`
	OrderID        string `json:"order_id"`
	ReferenceID    string `json:"reference_id"`
	OriginalAmount int64  `json:"original_amount"`
	UniqueCode     int    `json:"unique_code"`
	TotalAmount    int64  `json:"total_amount"`
	PaidAt         string `json:"paid_at"`
	ShopeeTxID     string `json:"shopee_tx_id"`
}

// TrackOrderRequest for retrieving secret link
type TrackOrderRequest struct {
	OrderID       string `json:"order_id" binding:"required"`
	CustomerEmail string `json:"customer_email" binding:"required,email"`
}

// TrackOrderResponse returns secret claim link
type TrackOrderResponse struct {
	OrderID       string `json:"order_id"`
	SecretToken   string `json:"secret_token"`
	ProductName   string `json:"product_name"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
	TotalAmount   int64  `json:"total_amount"`
	CreatedAt     string `json:"created_at"`
	ClaimURL      string `json:"claim_url"`
}

// StockItemDetail for admin inventory view
type StockItemDetail struct {
	ID        int64   `json:"id"`
	ProductID string  `json:"product_id"`
	Content   string  `json:"content"`
	IsUsed    int     `json:"is_used"`
	Status    string  `json:"status"`
	OrderID   *string `json:"order_id,omitempty"`
	HeldUntil *string `json:"held_until,omitempty"`
	CreatedAt string  `json:"created_at"`
	UsedAt    *string `json:"used_at,omitempty"`
}

// ProductStockSummary for admin modal
type ProductStockSummary struct {
	ProductID      string            `json:"product_id"`
	ProductName    string            `json:"product_name"`
	TotalCount     int               `json:"total_count"`
	AvailableCount int               `json:"available_count"`
	HeldCount      int               `json:"held_count"`
	SoldCount      int               `json:"sold_count"`
	Items          []StockItemDetail `json:"items"`
}
