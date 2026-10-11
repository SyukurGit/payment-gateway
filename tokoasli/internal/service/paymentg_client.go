package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type PaymentGClient struct {
	baseURL    string
	apiKey     string
	isSandbox  bool
	httpClient *http.Client
}

func NewPaymentGClient(baseURL, apiKey string, isSandbox bool) *PaymentGClient {
	if strings.HasPrefix(apiKey, "ak_sbx_") {
		isSandbox = true
	}
	return &PaymentGClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		isSandbox: isSandbox,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type PaymentGCreateOrderReq struct {
	ReferenceID   string `json:"reference_id"`
	Amount        int64  `json:"amount"`
	ExpiryMinutes int    `json:"expiry_minutes"`
}

type PaymentGCreateOrderResp struct {
	Success bool `json:"success"`
	Data    struct {
		OrderID        string `json:"order_id"`
		ReferenceID    string `json:"reference_id"`
		OriginalAmount int64  `json:"original_amount"`
		UniqueCode     int    `json:"unique_code"`
		TotalAmount    int64  `json:"total_amount"`
		Status         string `json:"status"`
		QRURL          string `json:"qr_url"`
		ExpiresAt      string `json:"expires_at"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

type PaymentGOrderStatusData struct {
	OrderID        string  `json:"order_id"`
	ID             string  `json:"id"`
	ReferenceID    string  `json:"reference_id"`
	OriginalAmount int64   `json:"original_amount"`
	UniqueCode     int     `json:"unique_code"`
	TotalAmount    int64   `json:"total_amount"`
	Status         string  `json:"status"`
	PaidAt         *string `json:"paid_at"`
	ShopeeTxID     string  `json:"shopee_tx_id"`
	CreatedAt      string  `json:"created_at"`
}

type PaymentGStatusResp struct {
	Success bool                    `json:"success"`
	Data    PaymentGOrderStatusData `json:"data"`
	Error   string                  `json:"error,omitempty"`
}

func (p *PaymentGClient) BaseURL() string {
	return p.baseURL
}

func (p *PaymentGClient) getBasePath() string {
	if p.isSandbox || strings.HasPrefix(p.apiKey, "ak_sbx_") {
		return "/api/sandbox/orders"
	}
	return "/api/orders"
}

// CreateOrder calls PaymentG API to generate dynamic QRIS
func (p *PaymentGClient) CreateOrder(refID string, amount int64, expiryMinutes int) (*PaymentGCreateOrderResp, error) {
	if p.apiKey == "" {
		return nil, errors.New("PAYMENTG_API_KEY belum dikonfigurasi di .env")
	}

	reqBody := PaymentGCreateOrderReq{
		ReferenceID:   refID,
		Amount:        amount,
		ExpiryMinutes: expiryMinutes,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal paymentg request: %w", err)
	}

	url := fmt.Sprintf("%s%s", p.baseURL, p.getBasePath())
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to build paymentg request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi PaymentG (%s): %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paymentg response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("paymentg error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result PaymentGCreateOrderResp
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal paymentg response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("paymentg returned failure: %s", result.Error)
	}

	return &result, nil
}

// CheckOrder forces a mutation poll and checks order status in PaymentG
func (p *PaymentGClient) CheckOrder(pgOrderID string) (*PaymentGStatusResp, error) {
	if p.apiKey == "" {
		return nil, errors.New("PAYMENTG_API_KEY belum dikonfigurasi di .env")
	}

	url := fmt.Sprintf("%s%s/%s/check", p.baseURL, p.getBasePath(), pgOrderID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build check request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi PaymentG check (%s): %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read check response: %w", err)
	}

	var result PaymentGStatusResp
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse check response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("paymentg check error: %s", result.Error)
	}

	// Normalize OrderID if returned in ID field
	if result.Data.OrderID == "" && result.Data.ID != "" {
		result.Data.OrderID = result.Data.ID
	}

	return &result, nil
}

// CancelOrder calls PaymentG to cancel the order on gateway side
func (p *PaymentGClient) CancelOrder(pgOrderID string) error {
	if p.apiKey == "" {
		return errors.New("PAYMENTG_API_KEY belum dikonfigurasi di .env")
	}

	url := fmt.Sprintf("%s%s/%s/cancel", p.baseURL, p.getBasePath(), pgOrderID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to build cancel request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gagal menghubungi PaymentG cancel (%s): %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("paymentg cancel failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetOrder fetches order status from PaymentG
func (p *PaymentGClient) GetOrder(pgOrderID string) (*PaymentGStatusResp, error) {
	if p.apiKey == "" {
		return nil, errors.New("PAYMENTG_API_KEY belum dikonfigurasi di .env")
	}

	url := fmt.Sprintf("%s%s/%s", p.baseURL, p.getBasePath(), pgOrderID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build get order request: %w", err)
	}

	req.Header.Set("X-API-Key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi PaymentG get (%s): %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read get order response: %w", err)
	}

	var result PaymentGStatusResp
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse get order response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("paymentg get order error: %s", result.Error)
	}

	if result.Data.OrderID == "" && result.Data.ID != "" {
		result.Data.OrderID = result.Data.ID
	}

	return &result, nil
}
