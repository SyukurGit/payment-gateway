# 🚀 PaymentG VPS Deployment Guide (Production & AI Agent)

Panduan deployment produksi untuk **PaymentG** pada Linux VPS (Ubuntu 22.04/24.04 LTS atau Debian 11/12). Dokumen ini disusun ringkas, padat, dan langsung dapat dieksekusi oleh **DevOps Engineer** maupun **AI Agent CLI di VPS** (Antigravity, Cursor, Claude Code, SSH Agent).

---

## 🤖 Context for VPS AI Agent

> **Agent Instruction:**
> Execute the steps in this guide sequentially.
> The target service is a compiled Go binary (`paymentg`) listening on internal port `3200`, monitored by `systemd`, exposed via Cloudflare Tunnel (or Nginx) under domain `paymentg.syukurapi.online`.
> Complete each step, verify with `curl http://localhost:3200/api/health`, and ensure HTTP 200 OK.

---

## 📋 Prasyarat Sistem

- **OS:** Ubuntu 20.04/22.04/24.04 LTS atau Debian 11/12
- **Resource Minimum:** 1 vCPU, 1 GB RAM (Rekomendasi: 2 vCPU, 2 GB RAM)
- **Port:** `3200` (Internal localhost, tidak perlu dibuka ke publik jika menggunakan Cloudflare Tunnel)

---

## 🛠️ Langkah Instalasi Berurutan

### Langkah 1: Update Paket Dasar
```bash
sudo apt update && sudo apt install -y curl git build-essential python3 python3-pip python3-venv
```

### Langkah 2: Install Go 1.23+ (Jika Belum Terpasang)
```bash
if ! command -v go &> /dev/null; then
  wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
  rm go1.23.6.linux-amd64.tar.gz
  echo 'export PATH=$PATH:/usr/local/go' >> ~/.bashrc
  export PATH=$PATH:/usr/local/go
fi
go version
```

### Langkah 3: Compile Binary Linux
Di dalam direktori project (`paymentg`):
```bash
export GOTOOLCHAIN=local
go build -ldflags="-s -w" -o paymentg .
chmod +x paymentg
```

### Langkah 4: Setup Lingkungan `.env`
Siapkan konfigurasi produksi:
```bash
if [ ! -f .env ]; then
  cp .env.example .env
fi
```
Pastikan variabel kunci berikut terisi di `.env`:
```env
PORT=3200
ADMIN_KEY=adm_secret_paymentg_2026
SHOPEE_TOKEN=
QRIS_STRING=
DEFAULT_EXPIRY_MINUTES=15
POLL_INTERVAL_SECONDS=5
DATA_DIR=./data
```

### Langkah 5: Setup Playwright Headless (Auto-Refresh Token)
Diperlukan untuk memperpanjang sesi token ShopeePay secara otomatis tanpa membuka browser:
```bash
pip3 install --break-system-packages playwright httpx
playwright install chromium
playwright install-deps chromium
```

> **Sesi Awal:**
> Salin folder profil sesi dari komputer lokal:
> `scp -r data/browser_profile user@vps:/opt/paymentg/data/`
> Atau cukup masukkan token aktif via menu **"✏️ Tempel Token"** di Dashboard web.

### Langkah 6: Daftarkan Systemd Service (Auto-Start 24/7)
```bash
APP_DIR=$(pwd)
CURRENT_USER=$(whoami)

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
```

---

## 🌐 Langkah 7: Ekspos Domain Publik (Cloudflare Tunnel)

Metode yang direkomendasikan adalah **Cloudflare Tunnel** (Zero Open Port, Free SSL, Anti-DDoS):

1. **Tambahkan Ingress Rule:**
   Buka file konfigurasi Cloudflare (`/etc/cloudflared/config.yml` atau `~/.cloudflared/config.yml`), sisipkan sebelum `- service: http_status:404`:
   ```yaml
     - hostname: paymentg.syukurapi.online
       service: http://localhost:3200
   ```

2. **Route DNS Subdomain:**
   ```bash
   cloudflared tunnel route dns <NAMA_TUNNEL> paymentg.syukurapi.online
   ```
   *(Atau tambahkan CNAME manual di Cloudflare Dashboard: `paymentg` ➔ `<TUNNEL_UUID>.cfargot.com`)*.

3. **Restart Cloudflare Tunnel:**
   ```bash
   sudo systemctl restart cloudflared
   ```

### Alternatif: Nginx Reverse Proxy (Jika Tanpa Cloudflare Tunnel)
```nginx
server {
    server_name paymentg.domainanda.com;

    location / {
        proxy_pass http://127.0.0.1:3200;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 🧪 Langkah 8: Verifikasi Deployment

Jalankan perintah pengujian:

```bash
# 1. Cek status daemon systemd
sudo systemctl status paymentg --no-pager

# 2. Uji endpoint internal
curl -s http://localhost:3200/api/health

# 3. Uji endpoint publik HTTPS
curl -s https://paymentg.syukurapi.online/api/health
```

**Hasil yang Diharapkan (HTTP 200 OK):**
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "token_valid": true,
    "pending_orders": 0,
    "last_poll_success": true
  }
}
```

---

## 🛡️ Pemeliharaan & Troubleshooting

### Memantau Log Realtime
```bash
sudo journalctl -u paymentg -f -n 100
```

### Restart Layanan
```bash
sudo systemctl restart paymentg
```

### Backup Database SQLite
PaymentG menggunakan mode SQLite WAL. Untuk mencadangkan data tanpa mematikan server:
```bash
# Backup database live & sandbox
sqlite3 data/payment.db ".backup 'data/backup_live_$(date +%F).db'"
sqlite3 data/sandbox/payment.db ".backup 'data/sandbox/backup_sandbox_$(date +%F).db'"
```
