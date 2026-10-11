package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port            int
	AdminPassword   string
	PaymentGURL     string
	PaymentGAPIKey  string
	PaymentGSandbox bool
	WebhookSecret   string
	ResendAPIKey    string
	ResendFromEmail string
	WhatsAppNumber  string
	StoreURL        string
	DataDir         string
}

// LoadDotEnv loads KEY=VALUE pairs from a .env file into os environment
func LoadDotEnv(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		return // File does not exist, ignore
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

// Load loads the active configuration
func Load() *Config {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	if port <= 0 {
		port = 8080
	}

	adminPass := getEnv("ADMIN_PASSWORD", "admin123")
	if adminPass == "" {
		adminPass = "admin123"
	}

	apiKey := getEnv("PAYMENTG_API_KEY", "")
	sbx, _ := strconv.ParseBool(getEnv("PAYMENTG_SANDBOX", "false"))
	if strings.HasPrefix(apiKey, "ak_sbx_") {
		sbx = true
	}

	pgURL := strings.TrimRight(getEnv("PAYMENTG_URL", "http://localhost:3200"), "/")
	if strings.HasSuffix(pgURL, "/sandbox") {
		pgURL = strings.TrimSuffix(pgURL, "/sandbox")
		sbx = true
	}

	cfg := &Config{
		Port:            port,
		AdminPassword:   adminPass,
		PaymentGURL:     pgURL,
		PaymentGAPIKey:  apiKey,
		PaymentGSandbox: sbx,
		WebhookSecret:   getEnv("WEBHOOK_SECRET", ""),
		ResendAPIKey:    getEnv("RESEND_API_KEY", ""),
		ResendFromEmail: getEnv("RESEND_FROM_EMAIL", "TokoAsli <onboarding@resend.dev>"),
		WhatsAppNumber:  getEnv("WHATSAPP_NUMBER", ""),
		StoreURL:        strings.TrimRight(getEnv("STORE_URL", "http://localhost:8080"), "/"),
		DataDir:         getEnv("DATA_DIR", "./data"),
	}

	modeStr := "LIVE"
	if cfg.PaymentGSandbox {
		modeStr = "SANDBOX"
	}
	log.Printf("[Config] Port: %d | DataDir: %s | PaymentG: %s (Mode: %s)", cfg.Port, cfg.DataDir, cfg.PaymentGURL, modeStr)
	return cfg
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
