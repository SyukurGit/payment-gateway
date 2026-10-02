# 🚀 PaymentG VPS Deployment Guide (Production & AI Agent)

Panduan deployment produksi untuk **PaymentG** pada Linux VPS (Ubuntu 20.04/22.04/24.04 LTS atau Debian 11/12). Dokumen ini disusun ringkas, padat, dan langsung dapat dieksekusi oleh **DevOps Engineer** maupun **AI Agent CLI di VPS** (Antigravity, Cursor, Claude Code, SSH Agent).

---

## 🤖 Context for VPS AI Agent

> **Agent Instruction:**
> When asked to deploy or run PaymentG on this Linux VPS:
> 1. You can execute the automated script: `bash deploy_vps.sh` (handles Go install, build, .env, systemd, and health verification).
> 2. Ensure the `dashboard/` folder is preserved next to the `paymentg` binary so both `/dashboard/` and `/sandbox/` web UIs work properly.
> 3. Verify that `curl -s http://localhost:3200/api/health` returns `"success":true`.
> 4. If Cloudflare Tunnel is used, ensure ingress route maps `paymentg.syukurapi.online` to `http://localhost:3200`.

---

## 📋 Prasyarat Sistem & Struktur Folder

- **OS:** Ubuntu 20.04/22.04/24.04 LTS atau Debian 11/12
- **Resource Minimum:** 1 vCPU, 1 GB RAM (Rekomendasi: 2 vCPU, 2 GB RAM)
- **Port:** `3200` (Internal localhost, tidak perlu dibuka ke publik jika menggunakan Cloudflare Tunnel)

### 📁 Struktur Direktori di VPS (`/opt/paymentg` atau `~/paymentg`)
Pastikan folder `dashboard/` ikut terbawa ke VPS agar frontend web dapat dimuat:
```
paymentg/
├── paymentg             # Binary Go hasil kompilasi
├── .env                 # File konfigurasi server
├── dashboard/           # Folder aset frontend (WAJIB ADA)
│   ├── index.html       # Dashboard Live
│   ├── sandbox.html     # Sandbox Simulator
│   ├── style.css        # Stylesheet
│   ├── app.js           # JS Live
│   └── sandbox.js       # JS Sandbox
├── refresh_token.py     # Script headless Playwright auto-refresh token
├── deploy_vps.sh        # Script instalasi otomatis
└── data/                # Dibuat otomatis oleh binary (database & qr cache)
```

---

## ⚡ Opsi 1: Instalasi Otomatis 1-Perintah (Direkomendasikan)

Jika Anda sudah meng-clone repository ini di VPS, cukup jalankan satu perintah:

```bash
bash deploy_vps.sh
```

Script ini otomatis:
1. Menginstall dependensi Linux & compiler Go 1.23+ resmi.
2. Mengompilasi binary `paymentg` dengan optimasi ukuran `-ldflags="-s -w"`.
3. Menyiapkan file `.env` jika belum ada.
4. Menginstal Playwright Chromium headless untuk auto-refresh sesi token.
5. Memasang dan mengaktifkan service Systemd `paymentg.service` (auto-restart 24/7).
6. Menguji endpoint `/api/health` dan menampilkan laporan kesiapan.

---

## 🛠️ Opsi 2: Instalasi Manual Langkah Demi Langkah

Bagi yang ingin menjalankan tiap perintah satu per satu:

### Langkah 1: Update Sistem & Paket Dasar
```bash
sudo apt update && sudo apt install -y curl git build-essential sqlite3 python3 python3-pip python3-venv
```

### Langkah 2: Install Go 1.23+ (Jika Belum Ada)
```bash
if ! command -v go &> /dev/null; then
  wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
  rm -f go1.23.6.linux-amd64.tar.gz
  echo 'export PATH=$PATH:/usr/local/go' >> ~/.bashrc
  export PATH=$PATH:/usr/local/go
fi
go version
```

### Langkah 3: Compile Binary Linux
```bash
export GOTOOLCHAIN=local
go build -ldflags="-s -w" -o paymentg .
chmod +x paymentg
```

### Langkah 4: Siapkan File `.env`
```bash
if [ ! -f .env ]; then
  cp .env.example .env
fi
```
Isi konfigurasi minimal di `.env`:
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
```bash
pip3 install --break-system-packages playwright httpx || pip3 install playwright httpx
playwright install chromium
playwright install-deps chromium
```

> 💡 **Input Sesi Token ShopeePay:**
> Anda cukup menempelkan token ShopeePay aktif dari HP/PC via tombol **"✏️ Tempel Token"** di Dashboard Web (atau bookmarklet 1-klik), tanpa harus login ulang di VPS.

### Langkah 6: Pasang Systemd Service (Auto-Start 24/7)
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

## 💻 Opsi 3: Cross-Compile dari Komputer Lokal (Windows)

Jika Anda tidak ingin menginstall Go compiler di VPS, Anda bisa mengompilasi dari komputer lokal Anda:

1. **Build binary Linux di Windows (PowerShell):**
   ```powershell
   $env:GOOS="linux"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o paymentg .
   ```
2. **Kirim file ke VPS (SSH/SCP):**
   ```powershell
   scp paymentg .env refresh_token.py user@IP_VPS:/opt/paymentg/
   scp -r dashboard user@IP_VPS:/opt/paymentg/
   ```
3. **Di VPS:** Buat service systemd seperti Langkah 6 di atas dan jalankan!

---

## 🌐 Menghubungkan Domain Publik

### Pilihan A: Cloudflare Tunnel (Sangat Direkomendasikan)
- **Keunggulan:** Port 3200 tetap tertutup dari publik (hanya internal), bebas SSL, anti-DDoS, zero network exposure.

1. **Tambahkan Ingress Rule di VPS:**
   Edit config Cloudflare Tunnel (`/etc/cloudflared/config.yml` atau `~/.cloudflared/config.yml`):
   ```yaml
     - hostname: paymentg.syukurapi.online
       service: http://localhost:3200
   ```
2. **Route DNS Subdomain:**
   ```bash
   cloudflared tunnel route dns <NAMA_TUNNEL> paymentg.syukurapi.online
   ```
3. **Restart Cloudflare Tunnel:**
   ```bash
   sudo systemctl restart cloudflared
   ```

---

### Pilihan B: Nginx Reverse Proxy + Certbot SSL

Jika tidak menggunakan Cloudflare Tunnel:
```bash
sudo apt install -y nginx certbot python3-certbot-nginx
```

Buat file `/etc/nginx/sites-available/paymentg`:
```nginx
server {
    listen 80;
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
Aktifkan dan pasang SSL gratis Let's Encrypt:
```bash
sudo ln -s /etc/nginx/sites-available/paymentg /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d paymentg.domainanda.com
```

---

## 🧪 Verifikasi & Uji Hasil Deployment

Jalankan perintah pengujian berikut di terminal VPS:

```bash
# 1. Cek status daemon systemd
sudo systemctl status paymentg --no-pager

# 2. Uji endpoint internal lokal
curl -s http://localhost:3200/api/health

# 3. Uji endpoint publik HTTPS
curl -s https://paymentg.syukurapi.online/api/health
```

**Respons yang Diharapkan (HTTP 200 OK):**
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

Buka di browser Anda:
- **Live Dashboard:** `https://paymentg.syukurapi.online/dashboard/`
- **Sandbox Simulator:** `https://paymentg.syukurapi.online/sandbox/`

---

## 🛡️ Pemeliharaan Harian & Troubleshooting

| Kebutuhan | Perintah |
| :--- | :--- |
| **Lihat Log Realtime** | `sudo journalctl -u paymentg -f -n 100` |
| **Restart Layanan** | `sudo systemctl restart paymentg` |
| **Hentikan Layanan** | `sudo systemctl stop paymentg` |
| **Backup SQLite Live** | `sqlite3 data/payment.db ".backup 'data/backup_live_$(date +%F).db'"` |
| **Backup SQLite Sandbox** | `sqlite3 data/sandbox/payment.db ".backup 'data/sandbox/backup_sandbox_$(date +%F).db'"` |
| **Reset Data Sandbox** | `rm -rf data/sandbox/* && sudo systemctl restart paymentg` |
