# 🏛️ Multipay Gateway — Unified Payment Monolith

Sistem Payment Gateway Terpadu berperforma tinggi berbasis **Go (Gin) + Pure-Go SQLite (`modernc.org/sqlite`)** yang menyatukan 3 engine pembayaran independen ke dalam satu service tunggal:
1. **ShopeePay Dynamic QRIS (IDR)** — EMVCo TLV Generator dengan kode unik 1 - 999.
2. **Binance Pay SAPI Personal (USDT)** — Presisi milli-units (0.001 USDT) dengan clock drift auto-correction.
3. **USDT BEP-20 Watch-Only (BSC)** — Non-custodial multi-node RPC failover & CEX fee rescue.

---

## 🌟 Fitur Utama
- **Single PIN Gate (`2207`):** Satu pintu masuk modal keamanan di dashboard dengan proteksi brute force rate limiting.
- **Sidebar Terisolasi:** Menu navigasi independen untuk setiap channel (Overview, ShopeePay QRIS, Binance Pay, USDT BEP-20, Toko Merchant, Keamanan).
- **Runtime CRUD Configuration (Tanpa Edit `.env` / Restart Server):**
  - Update Alamat Wallet Penerima USDT On-Chain & Daftar Node RPC BSC.
  - Update Binance UID / Pay ID, Nama Akun, dan SAPI Keys.
  - Update Sesi Token ShopeePay Partner & Static QRIS String.
  - Semua konfigurasi tersimpan di SQLite `system_config` dengan fallback bawaan otomatis dari `.env`.
- **Dual Database Environment Isolation:**
  - Mode Live / Produksi: `data/payment.db`
  - Mode Sandbox: `data/sandbox/payment.db`
- **1-Klik Sandbox Tester:**
  - Generator invoice uji coba instan untuk QRIS, Binance, dan USDT.
  - Tombol **1-Klik Bayar Lunas** untuk menguji alur pembayaran dan dispatch webhook ke website toko secara real-time.
- **CEX Fee Rescue:**
  - Penyelamatan mutasi bursa (Indodax, Bybit, Binance) yang terpotong biaya penarikan on-chain. Admin dapat memasangkan mutasi gantung ke invoice tujuan hanya dengan 1 klik.
- **Zero-CGO Pure-Go SQLite:**
  - Berjalan dalam mode WAL (`PRAGMA journal_mode(WAL)`), `busy_timeout(5000)`, dan `MaxOpenConns(1)` untuk keandalan maksimal tanpa memory/goroutine leak.

---

## 🚀 Cara Menjalankan

### 1. Menjalankan Binary
```bash
cd multipay
./multipay.exe
```
Server akan aktif di port yang tertera pada `.env` (default `PORT=3500` atau `3200`).

### 2. Membuka Dashboard Admin
Buka browser dan akses:
```
http://localhost:3500/dashboard/
```
Masukkan **Admin PIN** bawaan: `2207` (dapat diubah di tab Keamanan).

### 3. Dokumentasi Integrasi per Channel
- ShopeePay QRIS: `http://localhost:3500/dashboard/docs/qris.html`
- Binance Pay SAPI: `http://localhost:3500/dashboard/docs/binance.html`
- USDT BEP-20: `http://localhost:3500/dashboard/docs/usdt.html`

---

## 🌐 Hierarchy Endpoint API

### 1. ShopeePay QRIS Channel
| Method | Live Endpoint | Sandbox Endpoint | Keterangan |
| :--- | :--- | :--- | :--- |
| `POST` | `/qris/api/orders` | `/qris/api/sandbox/orders` | Buat tagihan QRIS dinamis |
| `GET` | `/qris/api/orders/:id` | `/qris/api/sandbox/orders/:id` | Cek status tagihan |
| `GET` | `/qris/api/orders/:id/qr.png` | `/qris/api/sandbox/orders/:id/qr.png` | Render QRIS PNG |
| `POST` | `/qris/api/orders/:id/check` | `/qris/api/sandbox/orders/:id/check` | Paksa cek mutasi |
| `POST` | - | `/qris/api/sandbox/orders/:id/pay` | 1-Klik Simulasi Lunas |

### 2. Binance Pay Channel
| Method | Live Endpoint | Sandbox Endpoint | Keterangan |
| :--- | :--- | :--- | :--- |
| `POST` | `/binance/api/orders` | `/binance/api/sandbox/orders` | Buat tagihan Binance Pay |
| `GET` | `/binance/api/orders/:id` | `/binance/api/sandbox/orders/:id` | Cek status tagihan |
| `GET` | `/binance/api/orders/:id/qr.png` | `/binance/api/sandbox/orders/:id/qr.png` | Render QR Binance |
| `POST` | `/binance/api/orders/:id/check` | `/binance/api/sandbox/orders/:id/check` | Tombol "Saya Sudah Bayar" |
| `POST` | - | `/binance/api/sandbox/orders/:id/pay` | 1-Klik Simulasi Lunas |

### 3. USDT BEP-20 Channel
| Method | Live Endpoint | Sandbox Endpoint | Keterangan |
| :--- | :--- | :--- | :--- |
| `POST` | `/usdt/api/orders` | `/usdt/api/sandbox/orders` | Buat tagihan USDT BEP-20 |
| `GET` | `/usdt/api/orders/:id` | `/usdt/api/sandbox/orders/:id` | Cek status tagihan |
| `GET` | `/usdt/api/orders/:id/qr.png` | `/usdt/api/sandbox/orders/:id/qr.png` | Render QR Wallet |
| `POST` | `/usdt/api/orders/:id/check` | `/usdt/api/sandbox/orders/:id/check` | Trigger scan RPC BSC |
| `POST` | - | `/usdt/api/sandbox/orders/:id/pay` | 1-Klik Simulasi Lunas |
