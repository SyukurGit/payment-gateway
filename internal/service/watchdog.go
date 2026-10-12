package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"paymentg/internal/database"
)

type Watchdog struct {
	db             *database.DB
	botToken       string
	chatID         string
	httpClient     *http.Client
	alertSent      bool
	criticalSent   bool
	lastCheckTime  time.Time
	mu             sync.Mutex
}

func NewWatchdog(db *database.DB, botToken, chatID string) *Watchdog {
	return &Watchdog{
		db:         db,
		botToken:   botToken,
		chatID:     chatID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *Watchdog) Start(ctx context.Context) {
	log.Println("[Watchdog] Background token expiry watchdog started")

	// Periksa setiap 15 menit
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// Initial check on startup
	w.checkTokenAge()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkTokenAge()
		}
	}
}

func (w *Watchdog) checkTokenAge() {
	updatedAtStr, err := w.db.GetConfig("shopee_token_updated_at")
	if err != nil || updatedAtStr == "" {
		return
	}

	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return
	}

	ageHours := time.Since(updatedAt).Hours()

	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. Kondisi Pemulihan (Recovery): Token baru masuk (< 2 jam) setelah sebelumnya ada alert
	if ageHours < 2.0 && (w.alertSent || w.criticalSent) {
		log.Printf("[Watchdog] Sesi ShopeePay pulih. Token baru berusia %.2f jam.", ageHours)
		w.sendTelegram(fmt.Sprintf(
			"✅ *[PaymentG VPS] Sesi ShopeePay Berhasil Diperbarui*\n\n"+
				"Token baru telah disinkronkan secara otomatis.\n"+
				"• Usia token saat ini: `%.1f menit`\n"+
				"• Status Gateway: *Normal / Aktif*",
			ageHours*60,
		))
		w.alertSent = false
		w.criticalSent = false
		return
	}

	// 2. Kondisi Kritis (Critical): Usia token >= 23 jam (tersisa < 1 jam)
	if ageHours >= 23.0 && !w.criticalSent {
		log.Printf("[Watchdog] KRITIS: Usia token ShopeePay sudah %.1f jam!", ageHours)
		w.sendTelegram(fmt.Sprintf(
			"🚨 *[PaymentG VPS] KRITIS: Sesi ShopeePay Akan Mati Segera!*\n\n"+
				"• Usia token: *%.1f jam*\n"+
				"• Sisa masa aktif: *kurang dari 1 jam*\n\n"+
				"Segera nyalakan PC atau buka dashboard Shopee Partner agar auto-sync memperbarui sesi sebelum pembayaran terhenti!",
			ageHours,
		))
		w.criticalSent = true
		w.alertSent = true
		return
	}

	// 3. Peringatan Awal (Warning): Usia token >= 20 jam (tersisa ~4 jam)
	if ageHours >= 20.0 && !w.alertSent {
		log.Printf("[Watchdog] Peringatan: Usia token ShopeePay sudah %.1f jam (sisa ~%.1f jam)", ageHours, 24.0-ageHours)
		w.sendTelegram(fmt.Sprintf(
			"⚠️ *[PaymentG VPS] Peringatan: Sesi ShopeePay Mendekati Batas 24 Jam*\n\n"+
				"• Usia token: `%.1f jam`\n"+
				"• Estimasi sisa aktif: `~%.1f jam`\n\n"+
				"Pastikan komputer sinkronisasi Anda menyala atau buka web Shopee Partner untuk perpanjangan otomatis.",
			ageHours,
			24.0-ageHours,
		))
		w.alertSent = true
	}
}

func (w *Watchdog) sendTelegram(text string) {
	if w.botToken == "" || w.chatID == "" {
		// Telegram tidak dikonfigurasi, cukup lewat log
		return
	}

	go func() {
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", w.botToken)
		payload := map[string]interface{}{
			"chat_id":    w.chatID,
			"text":       text,
			"parse_mode": "Markdown",
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			log.Printf("[Watchdog] Gagal mengirim alert Telegram: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[Watchdog] Telegram API response non-200: %d", resp.StatusCode)
		} else {
			log.Println("[Watchdog] Berhasil mengirim alert Telegram.")
		}
	}()
}
