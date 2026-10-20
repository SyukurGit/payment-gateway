package model

// Order statuses
const (
	StatusPending   = "PENDING"
	StatusPaid      = "PAID"
	StatusExpired   = "EXPIRED"
	StatusCancelled = "CANCELLED"
)

// Supported channels
const (
	ChannelQRIS    = "qris"
	ChannelBinance = "binance"
	ChannelUSDT    = "usdt"
)

// Order represents an invoice across any payment channel
type Order struct {
	ID                  string  `json:"id"`
	AppID               string  `json:"app_id"`
	Channel             string  `json:"channel"`      // 'qris', 'binance', 'usdt'
	ReferenceID         string  `json:"reference_id"` // merchant invoice reference
	Currency            string  `json:"currency"`     // 'IDR', 'USDT'
	Network             string  `json:"network,omitempty"`
	WalletAddress       string  `json:"wallet_address,omitempty"`
	OriginalAmount      float64 `json:"original_amount"`
	OriginalAmountMilli int64   `json:"original_amount_milli"`
	UniqueCode          int     `json:"unique_code"`
	TotalAmount         float64 `json:"total_amount"`
	TotalAmountStr      string  `json:"total_amount_str"`
	TotalAmountMilli    int64   `json:"total_amount_milli"`
	Status              string  `json:"status"`
	TxID                string  `json:"tx_id,omitempty"`
	TxHash              string  `json:"tx_hash,omitempty"`
	FromInfo            string  `json:"from_info,omitempty"`
	BlockNumber         int64   `json:"block_number,omitempty"`
	QRContent           string  `json:"qr_content"`
	QRPath              string  `json:"qr_path"`
	CreatedAt           int64   `json:"created_at"` // Unix ms
	ExpiresAt           int64   `json:"expires_at"` // Unix ms
	CooldownUntil       int64   `json:"cooldown_until"`
	PaidAt              int64   `json:"paid_at,omitempty"`
	WebhookDelivered    int     `json:"webhook_delivered"`
	WebhookAttempts     int     `json:"webhook_attempts"`
	NextWebhookRetryAt  int64   `json:"next_webhook_retry_at,omitempty"`
	Metadata            string  `json:"metadata,omitempty"`
}

// App represents a merchant registered application
type App struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	APIKey        string `json:"api_key"`
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     string `json:"created_at"`
}

// UnmatchedTx represents unmatched deposits on Binance or BSC
type UnmatchedTx struct {
	ID              string `json:"id"`
	Channel         string `json:"channel"`
	TxHash          string `json:"tx_hash,omitempty"`
	FromInfo        string `json:"from_info,omitempty"`
	Currency        string `json:"currency"`
	AmountStr       string `json:"amount_str"`
	AmountMilli     int64  `json:"amount_milli"`
	RawPayload      string `json:"raw_payload,omitempty"`
	TxTime          int64  `json:"tx_time"`
	ResolvedOrderID string `json:"resolved_order_id,omitempty"`
	RecordedAt      string `json:"recorded_at"`
}

// CreateOrderRequest is the payload from a merchant store
type CreateOrderRequest struct {
	ReferenceID   string  `json:"reference_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
	ExpiryMinutes int     `json:"expiry_minutes,omitempty"`
	Channel       string  `json:"channel,omitempty"` // optional if called via namespaced route
	Metadata      string  `json:"metadata,omitempty"`
}

// OrderResponse is returned to client apps
type OrderResponse struct {
	ID             string  `json:"id"`
	Channel        string  `json:"channel"`
	ReferenceID    string  `json:"reference_id"`
	Currency       string  `json:"currency"`
	Network        string  `json:"network,omitempty"`
	OriginalAmount float64 `json:"original_amount"`
	UniqueCode     int     `json:"unique_code"`
	TotalAmount    float64 `json:"total_amount"`
	TotalAmountStr string  `json:"total_amount_str"`
	Status         string  `json:"status"`
	QRURL          string  `json:"qr_url"`
	QRContent      string  `json:"qr_content,omitempty"`
	WalletAddress  string  `json:"wallet_address,omitempty"`
	CreatedAt      int64   `json:"created_at"`
	ExpiresAt      int64   `json:"expires_at"`
	RemainingSec   int64   `json:"remaining_seconds"`
	PaidAt         int64   `json:"paid_at,omitempty"`
	TxHash         string  `json:"tx_hash,omitempty"`
	FromInfo       string  `json:"from_info,omitempty"`
	BlockNumber    int64   `json:"block_number,omitempty"`
	CEXWarning     string  `json:"cex_warning,omitempty"`
}

// WebhookPayload sent to merchant store
type WebhookPayload struct {
	Event          string  `json:"event"`
	IsSandbox      bool    `json:"is_sandbox"`
	Channel        string  `json:"channel"`
	OrderID        string  `json:"order_id"`
	ReferenceID    string  `json:"reference_id"`
	Currency       string  `json:"currency"`
	OriginalAmount float64 `json:"original_amount"`
	UniqueCode     int     `json:"unique_code"`
	TotalAmount    float64 `json:"total_amount"`
	TotalAmountStr string  `json:"total_amount_str"`
	PaidAt         int64   `json:"paid_at"`
	TxID           string  `json:"tx_id,omitempty"`
	TxHash         string  `json:"tx_hash,omitempty"`
	Timestamp      int64   `json:"timestamp"`
}

// ShopeeTransaction model
type ShopeeTransaction struct {
	TransactionID string `json:"transactionId"`
	Amount        string `json:"amount"`
	Status        int    `json:"status"` // 1 & 3 are completed
	CreateTime    int64  `json:"createTime"`
	StoreId       int64  `json:"storeId"`
	MerchantId    int64  `json:"merchantId"`
}

type ShopeeTransactionListResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List []ShopeeTransaction `json:"list"`
	} `json:"data"`
}
