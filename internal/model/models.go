package model

// Status constants
const (
	StatusPending   = "PENDING"
	StatusPaid      = "PAID"
	StatusExpired   = "EXPIRED"
	StatusCancelled = "CANCELLED"
)

// App represents a registered web client
type App struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	APIKey        string `json:"api_key"`
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     string `json:"created_at"`
}

// Order represents a payment order
type Order struct {
	ID              string  `json:"id"`
	AppID           string  `json:"app_id"`
	ReferenceID     string  `json:"reference_id,omitempty"`
	OriginalAmount  int64   `json:"original_amount"`
	UniqueCode      int     `json:"unique_code"`
	TotalAmount     int64   `json:"total_amount"`
	Status          string  `json:"status"`
	QRString        string  `json:"qr_string,omitempty"`
	ExpiresAt       string  `json:"expires_at"`
	PaidAt          *string `json:"paid_at,omitempty"`
	ShopeeTxID      string  `json:"shopee_tx_id,omitempty"`
	WebhookSent     bool    `json:"webhook_sent"`
	WebhookAttempts int     `json:"webhook_attempts"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	Metadata        string  `json:"metadata,omitempty"`
}

// ProcessedTransaction prevents double-processing
type ProcessedTransaction struct {
	ShopeeTxID     string `json:"shopee_tx_id"`
	Amount         int64  `json:"amount"`
	MatchedOrderID string `json:"matched_order_id,omitempty"`
	ProcessedAt    string `json:"processed_at"`
}

// Config holds key-value settings
type Config struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// --- API Request/Response types ---

type CreateOrderRequest struct {
	ReferenceID   string `json:"reference_id"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
	ExpiryMinutes int    `json:"expiry_minutes"`
	Metadata      string `json:"metadata"`
}

type CreateOrderResponse struct {
	OrderID        string `json:"order_id"`
	ReferenceID    string `json:"reference_id,omitempty"`
	OriginalAmount int64  `json:"original_amount"`
	UniqueCode     int    `json:"unique_code"`
	TotalAmount    int64  `json:"total_amount"`
	Status         string `json:"status"`
	QRURL          string `json:"qr_url"`
	ExpiresAt      string `json:"expires_at"`
	ExpiresInSecs  int    `json:"expires_in_seconds"`
}

type OrderStatusResponse struct {
	OrderID        string  `json:"order_id"`
	ReferenceID    string  `json:"reference_id,omitempty"`
	OriginalAmount int64   `json:"original_amount"`
	UniqueCode     int     `json:"unique_code"`
	TotalAmount    int64   `json:"total_amount"`
	Status         string  `json:"status"`
	PaidAt         *string `json:"paid_at,omitempty"`
	ShopeeTxID     string  `json:"shopee_tx_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

type CreateAppRequest struct {
	Name       string `json:"name" binding:"required"`
	WebhookURL string `json:"webhook_url" binding:"required"`
}

type CreateAppResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	APIKey        string `json:"api_key"`
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret"`
}

type UpdateTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type HealthResponse struct {
	Status          string  `json:"status"`
	TokenValid      bool    `json:"token_valid"`
	TokenUpdatedAt  string  `json:"token_updated_at,omitempty"`
	TokenAgeHours   float64 `json:"token_age_hours,omitempty"`
	TokenError      string  `json:"token_error,omitempty"`
	PendingOrders   int     `json:"pending_orders"`
	LastPollAt      string  `json:"last_poll_at,omitempty"`
	LastPollSuccess bool    `json:"last_poll_success"`
	Message         string  `json:"message,omitempty"`
}

// WebhookPayload sent to client apps
type WebhookPayload struct {
	Event          string `json:"event"`
	OrderID        string `json:"order_id"`
	ReferenceID    string `json:"reference_id,omitempty"`
	OriginalAmount int64  `json:"original_amount"`
	UniqueCode     int    `json:"unique_code"`
	TotalAmount    int64  `json:"total_amount"`
	PaidAt         string `json:"paid_at"`
	ShopeeTxID     string `json:"shopee_tx_id"`
}

// ShopeePay API types
type ShopeeTransactionListRequest struct {
	Data struct {
		Metadata struct {
			Token    string `json:"token"`
			Language string `json:"language"`
			Timezone string `json:"timezone"`
		} `json:"metadata"`
		PageSize int `json:"pageSize"`
		Filter   struct {
			StartTime   int64 `json:"startTime"`
			EndTime     int64 `json:"endTime"`
			ServiceList []int `json:"serviceList"`
		} `json:"filter"`
		Sorter struct {
			Field string `json:"field"`
			Order string `json:"order"`
		} `json:"sorter"`
		NextPosition string `json:"next_position"`
	} `json:"data"`
}

type ShopeeTransactionListResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List  []ShopeeTransaction `json:"list"`
		Total string              `json:"total"`
	} `json:"data"`
}

type ShopeeTransaction struct {
	TransactionID   string `json:"transactionId"`
	CreateTime      int64  `json:"createTime"`
	StoreID         int64  `json:"storeId"`
	StoreName       string `json:"storeName"`
	Service         int    `json:"service"`
	Amount          string `json:"amount"`
	Status          int    `json:"status"`
	TransactionType int    `json:"transactionType"`
	MerchantID      int64  `json:"merchantId"`
	MerchantName    string `json:"merchantName"`
}
