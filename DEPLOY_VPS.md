# 🚀 VPS Deployment & Setup Guide (AI Agent & Developer Prompt)

Panduan instalasi ini dirancang khusus agar **AI Agent CLI di VPS** (seperti Antigravity, Claude Code, Cursor, SSH Agent, dll.) dapat langsung membaca file ini, mengeksekusi instalasi dari awal sampai selesai secara otomatis, dan mengembalikan status verifikasi.

---

## 🤖 Prompt Context for VPS AI Agent
> **Instruction for AI Agent on VPS:**
> You are tasked with installing, compiling, and running **PaymentG** on this Linux VPS (Ubuntu/Debian).
> Please execute the steps below sequentially. Once done, verify that `curl http://localhost:3200/api/health` returns `status: ok` and output the final readiness report.

---

## 📋 Spesifikasi Kebutuhan VPS
* **OS:** Ubuntu 22.04 / 24.04 LTS atau Debian 11 / 12
* **Spesifikasi Minimum:** 1 Core CPU, 1 GB RAM (Spesifikasi Anda 2 Core, 2 GB RAM sudah **SANGAT LEBIH DARI CUKUP**).
* **Port Terbuka:** `3200` (atau reverse proxy Nginx).

---

## 🛠️ Langkah Instalasi Otomatis (Eksekusi Berurutan)

### Langkah 1: Update Sistem & Install Paket Dasar
```bash
sudo apt update && sudo apt install -y curl git python3 python3-pip python3-venv build-essential
```

### Langkah 2: Install Go (Jika Belum Terpasang)
Jika `go version` belum ada:
```bash
# Unduh dan pasang Go 1.23+ resmi
wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
rm go1.23.6.linux-amd64.tar.gz

# Tambahkan ke PATH
export PATH=$PATH:/usr/local/go
echo 'export PATH=$PATH:/usr/local/go' >> ~/.bashrc
```

### Langkah 3: Build Binary PaymentG Linux
Di dalam direktori project (`paymentg`):
```bash
# Pastikan toolchain lokal digunakan dan compile binary Linux
export GOTOOLCHAIN=local
go build -ldflags="-s -w" -o paymentg .
chmod +x paymentg
```

### Langkah 4: Setup Python & Playwright (Headless Chromium)
```bash
# Install library python yang dibutuhkan
pip3 install --break-system-packages playwright httpx

# Install Chromium headless beserta dependensi sistem Linux
playwright install chromium
playwright install-deps chromium
```

### Langkah 5: Siapkan File Konfigurasi `.env`
Jika file `.env` belum ada, salin dari contoh:
```bash
if [ ! -f .env ]; then
  cp .env.example .env
fi
```
*(Pastikan `PORT=3200`, `ADMIN_KEY`, dan `QRIS_STRING` sudah terisi di `.env`)*.

### Langkah 6: Pasang Systemd Service (Auto-Start 24 Jam)
Dapatkan lokasi direktori saat ini:
```bash
CURRENT_DIR=$(pwd)

sudo bash -c "cat <<EOF > /etc/systemd/system/paymentg.service
[Unit]
Description=PaymentG ShopeePay QRIS Gateway
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$CURRENT_DIR
ExecStart=$CURRENT_DIR/paymentg
Restart=always
RestartSec=5
Environment=PORT=3200

[Install]
WantedBy=multi-user.target
EOF"

# Reload dan aktifkan service
sudo systemctl daemon-reload
sudo systemctl enable --now paymentg
```

---

## 🌐 Langkah 7: Hubungkan ke Cloudflare Tunnel (Domain: `paymentg.syukurapi.online`)

Sistem menggunakan Cloudflare Tunnel yang sudah aktif di VPS Anda. Ini adalah **jalur keluar terbaik & paling aman**:
* **Zero Open Port:** Port 3200 tidak perlu dibuka ke publik (`ufw allow 3200` TIDAK diperlukan).
* **Anti-Bentrok:** PaymentG hanya mendengar di internal `localhost:3200`. Layanan lain di VPS tetap aman 100%.
* **Free SSL & DDoS Protection:** Otomatis HTTPS resmi oleh Cloudflare Edge.

### Instruksi untuk Agent VPS:

1. **Cari file konfigurasi Cloudflare Tunnel yang sedang aktif:**
   ```bash
   # Cek lokasi config cloudflared (biasanya di /etc/cloudflared/config.yml atau ~/.cloudflared/config.yml)
   CF_CONFIG="/etc/cloudflared/config.yml"
   if [ ! -f "$CF_CONFIG" ]; then
     CF_CONFIG="$HOME/.cloudflared/config.yml"
   fi
   echo "Menggunakan config: $CF_CONFIG"
   ```

2. **Tambahkan Ingress Rule untuk `paymentg.syukurapi.online`:**
   Sisipkan rule berikut tepat di atas baris `- service: http_status:404`:
   ```yaml
     - hostname: paymentg.syukurapi.online
       service: http://localhost:3200
   ```

3. **Arahkan DNS Subdomain di Cloudflare:**
   Jalankan perintah route DNS menggunakan nama tunnel yang sudah ada:
   ```bash
   # Dapatkan nama/UUID tunnel yang aktif
   cloudflared tunnel list

   # Hubungkan subdomain paymentg.syukurapi.online ke tunnel (ganti <NAMA_TUNNEL> dengan nama tunnel aktif Anda)
   cloudflared tunnel route dns <NAMA_TUNNEL> paymentg.syukurapi.online
   ```
   *(Atau tambahkan CNAME manual di Dashboard Cloudflare DNS: `paymentg` ➔ `<TUNNEL_UUID>.cfargot.com` dengan status Proxied 🟧)*.

4. **Restart Layanan Cloudflare Tunnel:**
   ```bash
   sudo systemctl restart cloudflared
   ```

---

## 🧪 Langkah 8: Verifikasi & Cek Status Layanan

Jalankan perintah ini untuk memastikan layanan telah aktif sempurna baik secara lokal maupun via domain publik:

```bash
# 1. Cek status proses systemd lokal
sudo systemctl status paymentg --no-pager

# 2. Uji endpoint healthcheck lokal
curl -s http://localhost:3200/api/health

# 3. Uji endpoint publik Cloudflare Tunnel (HTTPS)
curl -s https://paymentg.syukurapi.online/api/health
```

### Expected Output (200 OK):
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "token_valid": true,
    "pending_orders": 0
  }
}
```

---

## 🔒 Catatan Penting Tentang Playwright di VPS (Headless Mode)
1. **Tidak Ada Pop-up Jendela:** Di Linux VPS (tanpa desktop GUI), Playwright berjalan **100% di memori latar belakang (Headless)**. Tidak akan ada jendela Chrome yang muncul di terminal.
2. **Konsumsi Resource:** Saat standby, Playwright memakan **0 MB RAM**. Saat dipanggil untuk menyegarkan token, hanya membutuhkan **~90 MB RAM selama 3 detik**, lalu otomatis menutup kembali.
3. **Session Awal:**
   - Anda cukup menyalin folder `data/browser_profile` dari komputer lokal ke VPS (agar sesi Shopee yang sudah login terbawa).
   - Atau cukup tempel token ShopeePay dari HP Anda menggunakan tombol **"✏️ Tempel Token"** di Dashboard.

