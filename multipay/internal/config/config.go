package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	mu sync.RWMutex

	Port     int
	GinMode  string
	DataDir  string
	AdminPIN string
	AdminKey string

	// ShopeePay QRIS
	ShopeeToken      string
	QRISString       string
	TelegramBotToken string
	TelegramChatID   string

	// Binance Pay
	BinancePayID       string
	BinanceAccountName string
	BinanceUniQRURL    string
	BinanceAPIKey      string
	BinanceSecretKey   string

	// USDT BEP-20
	ReceiverWalletAddress string
	BSCRPCURLs            []string
	RequiredConfirmations int
}

// LoadDotEnv reads a .env file and sets environment variables if not already set
func LoadDotEnv(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		return
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
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func LoadConfig() *Config {
	return Load()
}

func Load() *Config {
	LoadDotEnv(".env")

	port, _ := strconv.Atoi(getEnv("PORT", "3200"))
	if port <= 0 {
		port = 3200
	}

	rpcStr := getEnv("BSC_RPC_URLS", "https://bsc-rpc.publicnode.com,https://1rpc.io/bnb,https://bsc.meowrpc.com,https://rpc.ankr.com/bsc")
	rpcParts := strings.Split(rpcStr, ",")
	var rpcs []string
	for _, r := range rpcParts {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			rpcs = append(rpcs, trimmed)
		}
	}

	confs, _ := strconv.Atoi(getEnv("REQUIRED_CONFIRMATIONS", "2"))
	if confs <= 0 {
		confs = 2
	}

	return &Config{
		Port:                  port,
		GinMode:               getEnv("GIN_MODE", "release"),
		DataDir:               getEnv("DATA_DIR", "./data"),
		AdminPIN:              getEnv("ADMIN_PIN", "2207"),
		AdminKey:              getEnv("ADMIN_KEY", "adm_secret_multipay_2026"),
		ShopeeToken:           getEnv("SHOPEE_TOKEN", ""),
		QRISString:            getEnv("QRIS_STRING", ""),
		TelegramBotToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:        getEnv("TELEGRAM_CHAT_ID", ""),
		BinancePayID:          getEnv("BINANCE_PAY_ID", ""),
		BinanceAccountName:    getEnv("BINANCE_ACCOUNT_NAME", "StoreOfficial"),
		BinanceUniQRURL:       getEnv("BINANCE_UNI_QR_URL", ""),
		BinanceAPIKey:         getEnv("BINANCE_API_KEY", ""),
		BinanceSecretKey:      getEnv("BINANCE_SECRET_KEY", ""),
		ReceiverWalletAddress: strings.ToLower(getEnv("RECEIVER_WALLET_ADDRESS", "0x71c67ed3484a222ab20e083d9204e646a803d0a1")),
		BSCRPCURLs:            rpcs,
		RequiredConfirmations: confs,
	}
}

func (c *Config) UpdateShopeeToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ShopeeToken = token
}

func (c *Config) GetShopeeToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ShopeeToken
}

func (c *Config) UpdateBinanceConfig(payID, name, uniQR, apiKey, secret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.BinancePayID = payID
	c.BinanceAccountName = name
	c.BinanceUniQRURL = uniQR
	c.BinanceAPIKey = apiKey
	c.BinanceSecretKey = secret
}

func (c *Config) UpdateUSDTConfig(wallet string, rpcs []string, confs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ReceiverWalletAddress = strings.ToLower(wallet)
	c.BSCRPCURLs = rpcs
	c.RequiredConfirmations = confs
}
