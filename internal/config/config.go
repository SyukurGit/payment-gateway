package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                 int
	AdminKey             string
	ShopeeToken          string
	QRISString           string
	DefaultExpiryMinutes int
	PollIntervalSeconds  int
	DataDir              string
}

func LoadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func generateAdminKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "adm_" + hex.EncodeToString(b)
}

func Load() *Config {
	LoadDotEnv(".env")

	portStr := os.Getenv("PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		port = 3200
	}

	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		adminKey = generateAdminKey()
		fmt.Printf("Generated missing ADMIN_KEY: %s\n", adminKey)
	}

	shopeeToken := os.Getenv("SHOPEE_TOKEN")
	qrisString := os.Getenv("QRIS_STRING")

	expiryStr := os.Getenv("DEFAULT_EXPIRY_MINUTES")
	expiry, err := strconv.Atoi(expiryStr)
	if err != nil || expiry == 0 {
		expiry = 15
	}

	pollStr := os.Getenv("POLL_INTERVAL_SECONDS")
	poll, err := strconv.Atoi(pollStr)
	if err != nil || poll == 0 {
		poll = 5
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	return &Config{
		Port:                 port,
		AdminKey:             adminKey,
		ShopeeToken:          shopeeToken,
		QRISString:           qrisString,
		DefaultExpiryMinutes: expiry,
		PollIntervalSeconds:  poll,
		DataDir:              dataDir,
	}
}
