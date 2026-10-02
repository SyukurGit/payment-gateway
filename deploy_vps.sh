#!/usr/bin/env bash
# ==============================================================================
# PaymentG VPS Auto-Deploy Script
# Supported OS: Ubuntu 20.04 / 22.04 / 24.04 LTS, Debian 11 / 12
# ==============================================================================

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}   🚀 PaymentG Automated VPS Deployment Script       ${NC}"
echo -e "${GREEN}=====================================================${NC}"

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CURRENT_USER="$(whoami)"

echo -e "\n${YELLOW}[1/7] Updating package lists & installing base tools...${NC}"
sudo apt update -y
sudo apt install -y curl git build-essential sqlite3 python3 python3-pip python3-venv

echo -e "\n${YELLOW}[2/7] Checking Go 1.23+ toolchain...${NC}"
if ! command -v go &> /dev/null; then
    echo "Installing Go 1.23.6 Linux amd64..."
    wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
    rm -f go1.23.6.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go
    if ! grep -q '/usr/local/go' ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go' >> ~/.bashrc
    fi
fi
go version

echo -e "\n${YELLOW}[3/7] Compiling PaymentG binary...${NC}"
cd "$APP_DIR"
export GOTOOLCHAIN=local
go build -ldflags="-s -w" -o paymentg .
chmod +x paymentg
echo -e "${GREEN}✓ Binary compiled successfully.${NC}"

echo -e "\n${YELLOW}[4/7] Ensuring environment configuration (.env)...${NC}"
if [ ! -f "$APP_DIR/.env" ]; then
    if [ -f "$APP_DIR/.env.example" ]; then
        cp "$APP_DIR/.env.example" "$APP_DIR/.env"
        echo -e "${GREEN}✓ Created .env from .env.example.${NC}"
    else
        cat <<EOF > "$APP_DIR/.env"
PORT=3200
ADMIN_KEY=adm_secret_paymentg_2026
SHOPEE_TOKEN=
QRIS_STRING=
DEFAULT_EXPIRY_MINUTES=15
POLL_INTERVAL_SECONDS=5
DATA_DIR=./data
EOF
        echo -e "${GREEN}✓ Created default .env file.${NC}"
    fi
else
    echo -e "${GREEN}✓ Existing .env preserved.${NC}"
fi

echo -e "\n${YELLOW}[5/7] Setting up Playwright for Auto-Refresh Token (Optional)...${NC}"
if command -v pip3 &> /dev/null; then
    pip3 install --break-system-packages playwright httpx || pip3 install playwright httpx || true
    playwright install chromium || true
    playwright install-deps chromium || true
    echo -e "${GREEN}✓ Playwright setup completed.${NC}"
fi

echo -e "\n${YELLOW}[6/7] Installing Systemd Service (paymentg.service)...${NC}"
sudo bash -c "cat <<EOF > /etc/systemd/system/paymentg.service
[Unit]
Description=PaymentG ShopeePay QRIS Gateway
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/paymentg
Restart=always
RestartSec=5
Environment=PORT=3200
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF"

sudo systemctl daemon-reload
sudo systemctl enable --now paymentg
sudo systemctl restart paymentg

echo -e "\n${YELLOW}[7/7] Verifying service health...${NC}"
sleep 2

HEALTH_CHECK=$(curl -s http://localhost:3200/api/health || echo "FAILED")

if echo "$HEALTH_CHECK" | grep -q '"success":true'; then
    echo -e "${GREEN}=====================================================${NC}"
    echo -e "${GREEN}   ✅ PaymentG Berhasil Diaktifkan di VPS!          ${NC}"
    echo -e "${GREEN}=====================================================${NC}"
    echo -e "Health response: $HEALTH_CHECK"
    echo -e "\nStatus Layanan:"
    echo -e "- Endpoint Internal : http://localhost:3200/api/health"
    echo -e "- Dashboard Live    : http://localhost:3200/dashboard/"
    echo -e "- Sandbox Simulator : http://localhost:3200/sandbox/"
    echo -e "\nPerintah Berguna:"
    echo -e "- Cek log   : sudo journalctl -u paymentg -f"
    echo -e "- Restart   : sudo systemctl restart paymentg"
    echo -e "- Stop      : sudo systemctl stop paymentg"
else
    echo -e "${RED}⚠️ Peringatan: Service aktif tapi healthcheck belum membalas OK.${NC}"
    echo -e "Response: $HEALTH_CHECK"
    echo -e "Silakan cek log dengan: sudo journalctl -u paymentg -n 50"
fi
