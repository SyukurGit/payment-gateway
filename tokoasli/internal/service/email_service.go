package service

import (
	"bytes"
	"encoding/json"
	"html"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"tokoasli/internal/model"
)

type EmailService struct {
	apiKey     string
	fromEmail  string
	storeURL   string
	httpClient *http.Client
}

func NewEmailService(apiKey, fromEmail, storeURL string) *EmailService {
	if strings.TrimSpace(fromEmail) == "" {
		fromEmail = "TokoAsli <onboarding@resend.dev>"
	}
	return &EmailService{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		storeURL:  storeURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type ResendEmailReq struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendReceipt sends payment confirmation email with secret claim link
// If apiKey is empty, logs gracefully to console without crashing
func (e *EmailService) SendReceipt(order *model.StoreOrder) error {
	claimLink := fmt.Sprintf("%s/order/?id=%s&token=%s", e.storeURL, order.ID, order.SecretToken)

	// Fallback mode if Resend API key is not configured
	if e.apiKey == "" {
		log.Println("==================================================================")
		log.Printf("[EMAIL FALLBACK - NO RESEND API KEY CONFIGURED]")
		log.Printf("Ke: %s", order.CustomerEmail)
		log.Printf("Subjek: [TokoAsli] Pembayaran Berhasil - Pesanan #%s", order.ID)
		log.Printf("Produk: %s (Rp %d)", order.ProductName, order.TotalAmount)
		log.Printf("SECRET CLAIM LINK: %s", claimLink)
		log.Println("⚠️  (Jangan bagikan link ini kepada siapa pun)")
		log.Println("==================================================================")
		return nil
	}

	htmlContent := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="utf-8">
		<title>Pembayaran Berhasil - TokoAsli</title>
	</head>
	<body style="font-family: Arial, sans-serif; background-color: #0f172a; color: #f8fafc; padding: 30px; margin: 0;">
		<div style="max-width: 600px; margin: 0 auto; background-color: #1e293b; border-radius: 12px; padding: 28px; border: 1px solid #334155;">
			<h2 style="color: #10b981; margin-top: 0;">✓ Pembayaran Berhasil!</h2>
			<p style="font-size: 15px; color: #cbd5e1;">Halo! Pembayaran untuk pesanan Anda telah kami terima.</p>

			<div style="background-color: #0f172a; padding: 16px; border-radius: 8px; margin: 20px 0; border: 1px solid #334155;">
				<p style="margin: 4px 0; font-size: 14px; color: #94a3b8;">Nomor Invoice: <strong style="color: #f8fafc;">#%s</strong></p>
				<p style="margin: 4px 0; font-size: 14px; color: #94a3b8;">Produk: <strong style="color: #f8fafc;">%s</strong></p>
				<p style="margin: 4px 0; font-size: 14px; color: #94a3b8;">Total Dibayar: <strong style="color: #10b981;">Rp %d</strong></p>
			</div>

			<div style="text-align: center; margin: 30px 0;">
				<a href="%s" style="background-color: #3b82f6; color: #ffffff; padding: 14px 28px; font-size: 16px; font-weight: bold; text-decoration: none; border-radius: 8px; display: inline-block;">
					🎁 Buka & Ambil Produk Anda
				</a>
			</div>

			<div style="background-color: #451a03; border: 1px solid #92400e; padding: 12px; border-radius: 8px; margin-top: 25px;">
				<p style="margin: 0; font-size: 13px; color: #fcd34d;">
					⚠️ <strong>PENTING:</strong> Jangan teruskan email atau bagikan link ini kepada siapa pun, karena link tersebut berisi akses langsung ke produk digital Anda.
				</p>
			</div>

			<p style="font-size: 12px; color: #64748b; margin-top: 30px; text-align: center;">
				Terima kasih telah berbelanja di TokoAsli.
			</p>
		</div>
	</body>
	</html>
	`, html.EscapeString(order.ID), html.EscapeString(order.ProductName), order.TotalAmount, claimLink)

	payload := ResendEmailReq{
		From:    e.fromEmail,
		To:      []string{order.CustomerEmail},
		Subject: fmt.Sprintf("[TokoAsli] Pembayaran Berhasil - Pesanan #%s", order.ID),
		HTML:    htmlContent,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to create email request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		log.Printf("[EmailService] Gagal menghubungi Resend API: %v (Link: %s)", err, claimLink)
		return nil // Non-fatal, keep going
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[EmailService] Resend API error (status %d): %s", resp.StatusCode, string(body))
		return nil // Non-fatal
	}

	log.Printf("[EmailService] Sukses mengirim email tanda terima ke %s", order.CustomerEmail)
	return nil
}

// SendReceiptAsync dispatches SendReceipt in a background goroutine
func (e *EmailService) SendReceiptAsync(order *model.StoreOrder) {
	if order == nil {
		return
	}
	orderCopy := *order
	go func() {
		_ = e.SendReceipt(&orderCopy)
	}()
}

