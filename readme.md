# ⚡ PaymentG — High-Performance ShopeePay QRIS Gateway

PaymentG adalah self-hosted payment gateway mandiri berkinerja tinggi (Go + Gin + Pure-Go SQLite). Gateway ini mengonversi QRIS statis merchant ShopeePay menjadi QRIS dinamis ber-nominal unik secara real-time, mendeteksi mutasi masuk secara otomatis via polling dashboard ShopeePay, dan mengirim notifikasi Webhook instan ke website e-commerce Anda.

Dilengkapi dengan antarmuka web modern untuk pemantauan transaksi riil di `/dashboard` dan simulator pengujian terisolasi penuh di `/sandbox`.

---

## 🌟 Fitur Utama

- **Zero-Dependency Single Binary**: Dikompilasi dengan Go murni (tanpa CGO), footprint RAM < 30 MB, startup instan.
- **Dynamic QRIS Injection**: TLV EMVCo parser bawaan yang menyuntikkan nominal tagihan ke QRIS ShopeePay statis tanpa library pihak ketiga.
- **Unique Code Matcher**: Alokasi 3-digit kode unik otomatis untuk memastikan pencocokan transaksi 100% akurat tanpa bentrok.
- **Isolated Sandbox Environment (`/sandbox`)**: Pengujian pembayaran dengan database fisik terpisah (`data/sandbox/payment.db`) dan simulator bayar 1-klik tanpa menyentuh pembukuan uang asli.
- **Zero-Code Switching**: Toko online cukup mengganti API URL & API Key dari mode Sandbox ke Live tanpa mengubah logika kode sama sekali.
- **Resilient Polling & Auto-Refresh Token**: Deteksi mutasi otomatis setiap 5 detik dengan opsi perpanjangan sesi token via headless Playwright atau bookmarklet 1-klik.
- **Direct JSON Webhook**: Notifikasi pembayaran otomatis dikirim ke webhook toko dengan payload JSON bersih dan respons HTTP 200 OK standar.

---

## 📁 Struktur Arsitektur Data

```
paymentg/
├── data/
│   ├── payment.db           # SQLite Production (Data transaksi riil & omset asli)
│   ├── qr/                  # Cache gambar QRIS PNG tagihan aktif
│   └── sandbox/
│       ├── payment.db       # SQLite Sandbox (Data testing terisolasi 100%)
│       └── qr/              # Cache gambar QRIS PNG sandbox
├── dashboard/               # Frontend UI (Dashboard Live & Sandbox Simulator)
├── internal/                # Engine Go (Config, Database, Handler, Service, Middleware)
├── main.go                  # Entry point & HTTP router
└── refresh_token.py         # Headless browser script untuk auto-refresh sesi Shopee
```

---

## 🚀 Quick Start

### 1. Jalankan Layanan

Salin `.env.example` ke `.env`, lalu jalankan binary:

```bash
# Windows
.\paymentg.exe

# Linux VPS
./paymentg
```

Layanan otomatis aktif di `http://localhost:3200` (atau port yang disetel di `.env`).

### 2. Buka Dashboard di Browser

Akses antarmuka web bawaan:
- **Mode Live (Omset Uang Asli):** `http://localhost:3200/dashboard/`
- **Mode Sandbox (Simulator Testing):** `http://localhost:3200/sandbox/`
- **Default PIN Akses:** `2207` (bisa diubah di `.env` / `ADMIN_KEY`).

---

## 🔑 Kredensial & Autentikasi

Semua request API diamankan menggunakan header HTTP:

| Header | Ditujukan Untuk | Fungsi |
| :--- | :--- | :--- |
| `X-Admin-Key` | Pemilik Server | Mengelola token ShopeePay, mendaftarkan toko baru, dan melihat agregasi transaksi. |
| `X-API-Key` | Web Toko / Klien | Membuat tagihan QRIS, memeriksa status transaksi, dan membatalkan pesanan. |

---

## 📋 Endpoint API Reference

### 1. Toko Online / Client API (`X-API-Key`)

| Method | Endpoint Live | Endpoint Sandbox | Deskripsi |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/orders` | `/api/sandbox/orders` | Membuat tagihan QRIS baru |
| `GET` | `/api/orders/:id` | `/api/sandbox/orders/:id` | Cek status pembayaran pesanan |
| `GET` | `/api/orders/:id/qr.png` | `/api/sandbox/orders/:id/qr.png` | Ambil gambar QRIS PNG (Publik / `<img>`) |
| `POST` | `/api/orders/:id/check` | `/api/sandbox/orders/:id/check` | Paksa cek mutasi instan (Tombol "Saya Sudah Bayar") |
| `POST` | `/api/orders/:id/cancel` | `/api/sandbox/orders/:id/cancel` | Batalkan tagihan & lepas kode unik |

#### Contoh Buat Tagihan (`POST /api/orders`)
```bash
curl -X POST http://localhost:3200/api/orders \
  -H "X-API-Key: ak_xxxxxxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "reference_id": "INV-1001",
    "amount": 50000,
    "expiry_minutes": 15
  }'
```

**Respons (HTTP 201 Created):**
```json
{
  "success": true,
  "data": {
    "order_id": "ord_aBcDeFg123",
    "reference_id": "INV-1001",
    "original_amount": 50000,
    "unique_code": 237,
    "total_amount": 50237,
    "status": "PENDING",
    "qr_url": "/api/orders/ord_aBcDeFg123/qr.png",
    "expires_at": "2026-10-02T16:30:00Z",
    "expires_in_seconds": 900
  }
}
```

> **PENTING:** Pembeli wajib mentransfer sejumlah `total_amount` (Rp 50.237), bukan nominal dasar. Kode unik 3-digit adalah kunci pencocokan otomatis di mutasi ShopeePay.

---

### 2. Admin & Gateway Control API (`X-Admin-Key`)

| Method | Endpoint | Deskripsi |
| :--- | :--- | :--- |
| `GET` | `/api/health` | Status server, masa aktif token, dan poller (Publik) |
| `POST` | `/api/auth/pin` | Verifikasi PIN login dashboard web |
| `PUT` | `/api/config/token` | Update manual token ShopeePay (JSON `{ "token": "..." }`) |
| `POST` | `/api/token/refresh` | Trigger auto-refresh token via Playwright |
| `GET` | `/api/stats` | Agregasi omset dan total transaksi (Live) |
| `GET` | `/api/orders` | Daftar seluruh riwayat transaksi (Live) |
| `POST` | `/api/apps` | Mendaftarkan web toko baru (Mendapatkan `X-API-Key`) |
| `GET` | `/api/apps` | Daftar web toko terdaftar |
| `DELETE` | `/api/apps/:id` | Menghapus web toko |
| `POST` | `/api/sandbox/orders/:id/pay` | Simulasi pembayaran 1-klik (Khusus Sandbox) |

---

## 🔔 Notifikasi Webhook

Saat pembayaran terdeteksi lunas di ShopeePay (atau tombol `Bayar Sekarang` diklik di Sandbox), PaymentG otomatis menembakkan HTTP POST ke `webhook_url` web toko Anda:

**Headers:**
```http
Content-Type: application/json
X-Webhook-Event: payment.success
```

**Payload JSON:**
```json
{
  "event": "payment.success",
  "order_id": "ord_aBcDeFg123",
  "reference_id": "INV-1001",
  "original_amount": 50000,
  "unique_code": 237,
  "total_amount": 50237,
  "paid_at": "2026-10-02T16:18:24Z",
  "shopee_tx_id": "122722636469377153"
}
```

Endpoint toko Anda cukup membaca payload tersebut, memperbarui database status pesanan menjadi `PAID`, lalu mengembalikan status `HTTP 200 OK`.

---

## 🔄 Pemeliharaan Sesi Token ShopeePay

Sesi dashboard ShopeePay Merchant memerlukan token aktif untuk membaca mutasi. Tersedia 2 mekanisme:

### A. Otomatis: Playwright Headless Bot
- Jalankan setup login sekali saja:
  ```bash
  python refresh_token.py --setup
  ```
- Server Go otomatis memanggil `refresh_token.py` di latar belakang bila mendeteksi token kedaluwarsa. Konsumsi RAM: 0 MB saat idle, ~90 MB selama 3 detik saat refresh.

### B. Cepat: 1-Click Bookmarklet Browser
Jika tidak menggunakan Playwright, pasang bookmarklet ini di browser:
```javascript
javascript:void(function(){var d=window.injectData||window['injectData'];if(d&&d.User&&d.User.token){fetch('http://localhost:3200/api/config/token',{method:'PUT',headers:{'Content-Type':'application/json','X-Admin-Key':'adm_secret_paymentg_2026'},body:JSON.stringify({token:d.User.token})}).then(r=>r.json()).then(j=>{alert('Token ShopeePay berhasil dikirim ke PaymentG!')}).catch(e=>alert('Gagal: '+e))}else{alert('Buka partner.shopee.co.id terlebih dahulu!')}}())
```
Setiap kali membuka `https://partner.shopee.co.id/`, klik bookmarklet dan token langsung terbarukan.

---

## 📚 Panduan Lengkap Lanjutan

- **Panduan Integrasi Toko (AI Agent & Dev):** Lihat [`INTEGRATION.md`](./INTEGRATION.md)
- **Panduan Deploy VPS & Cloudflare Tunnel:** Lihat [`DEPLOY_VPS.md`](./DEPLOY_VPS.md)
