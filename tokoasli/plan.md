# ⚡ Blueprint & Arsitektur TokoAsli v2.0 (Production-Grade & Hardened)

---

## 📌 1. Rangkuman Eksekutif & Pembaruan v2.0 (Changelog & Audit Fix)

Dokumen ini merupakan spesifikasi arsitektur resmi, panduan teknis, dan cetak biru operasional untuk **TokoAsli v2.0** — sistem toko online produk digital (akun streaming, lisensi software, voucher, file) mandiri yang terintegrasi langsung dengan gateway pembayaran QRIS dinamis **PaymentG**.

### 🛠️ Masalah Kritis yang Diperbaiki pada v2.0:
1. **Perbaikan Fatal Timezone & Perbandingan String SQLite:**
   - *Masalah Sebelumnya:* SQLite menyimpan tanggal sebagai string teks biasa dan membandingkannya secara leksikografis. Format `datetime('now')` lokal WIB (menggunakan spasi `YYYY-MM-DD HH:MM:SS`) bertabrakan dengan string UTC RFC3339 dari Go (menggunakan huruf `T`, e.g. `2026-10-04T12:00:00Z`). Akibatnya, pesanan baru langsung dianggap hangus (`EXPIRED`) oleh worker dalam waktu 30 detik pertama atau mengunci IP pembeli secara permanen.
   - *Solusi v2.0:* Seluruh operasi penulisan dan query SQLite distandardisasi 100% menggunakan `time.Now().UTC().Format(time.RFC3339)`. Klausa `datetime('now')` dihapus total dari seluruh file query SQL (`orders.go`, `stocks.go`, `products.go`).
2. **Zero-Leak Server-Sent Events (SSE) Stream:**
   - *Masalah Sebelumnya:* Payload broadcast SSE mengirimkan `delivery_content` (data akun/lisensi mentah) ke seluruh client yang mendengarkan event stream tanpa verifikasi token klaim rahasia.
   - *Solusi v2.0:* Field `delivery_content` dicabut dari broadcast SSE. Client hanya menerima event `{ "order_id": "...", "status": "PAID" }`. Pengambilan akun digital wajib melalui endpoint terotentikasi `GET /api/orders/:id?token=...` yang memverifikasi `secret_token` unik.
3. **Resiliensi Reverse Proxy (Nginx / Cloudflare):**
   - *Masalah Sebelumnya:* Koneksi SSE diputus secara sepihak oleh Cloudflare dan Nginx karena ketiadaan traffic (timeout 60–100 detik).
   - *Solusi v2.0:* Ditambahkan heartbeat/ping loop 20 detik (`: ping\n\n`) pada handler SSE, serta header `X-Accel-Buffering: no` dan `Cache-Control: no-cache`.
4. **Session Manager Kriptografis untuk Admin:**
   - *Masalah Sebelumnya:* Master password admin dikirim langsung dan disimpan sebagai token sesi di `sessionStorage` browser.
   - *Solusi v2.0:* Diimplementasikan `SessionManager` in-memory berbasis token kriptografi acak 32-byte (64 karakter heksadesimal) dengan masa berlaku (TTL) 24 jam dan background cleanup otomatis setiap 1 jam. Password master diverifikasi menggunakan *constant-time comparison* (`subtle.ConstantTimeCompare`) untuk mencegah *timing attacks*.
5. **Entropi Order ID Tinggi & Retry Loop:**
   - *Masalah Sebelumnya:* ID pesanan hanya menggunakan 2-byte acak (16-bit) tanpa loop penanganan benturan primary key.
   - *Solusi v2.0:* ID pesanan ditingkatkan menjadi 5-byte acak (`ORD-YYYYMMDD-XXXXXXXXXX`) dengan retry loop hingga 3 kali jika terjadi benturan kode unik / primary key.
6. **Email Sender Resend Terkonfigurasi:**
   - Ditambahkan env `RESEND_FROM_EMAIL` agar pengiriman email tanda terima tidak ditolak (`403 Forbidden`) oleh Resend ketika domain pengirim kustom telah terverifikasi.
7. **Soft-Archiving Produk:**
   - Penghapusan produk yang telah memiliki riwayat pesanan otomatis diubah menjadi soft-delete (`is_active = 0`) untuk mencegah kegagalan *foreign key constraint* (`FOREIGN KEY constraint failed`).

---

## 🎨 2. Peningkatan Fitur Bisnis & Pengalaman Pengguna (CX)

### A. Sisi Pembeli (Frontend / Public)
* **Cek Status Pembayaran Manual ("Sudah Transfer?"):** Tombol `POST /api/orders/:id/check` di layar QRIS untuk memverifikasi langsung status pesanan ke gateway jika mutasi bank/e-wallet telah masuk tetapi webhook mengalami delay jaringan.
* **Pembatalan Pesanan Mandiri ("Batalkan Pesanan"):** Tombol `POST /api/orders/:id/cancel` yang memungkinkan pembeli membatalkan pesanan sebelum masa berlaku habis, seketika melepas stok produk yang sedang di-hold agar dapat dibeli oleh orang lain.
* **Halaman Lacak Pesanan Digital ("🔍 Lacak Pesanan"):** Form modal pelacakan pesanan via Invoice ID + Email pembeli (`POST /api/orders/track`) untuk mengambil kembali tautan klaim rahasia (*Secret Claim Link*) jika tab browser tertutup atau pembeli berganti perangkat.
* **Widget Customer Service WhatsApp Kontekstual:** Tombol floating WhatsApp otomatis dengan pesan yang sudah terisi ID Invoice pesanan atau nama produk yang sedang dilihat.
* **Halaman Detail Produk Terpisah (`/product/?id=...`):** Halaman detail produk mandiri dengan breadcrumb dinamis, galeri gambar, deskripsi terstruktur, dan input opsional nomor WhatsApp pembeli.
* **Multi-Layer API Discovery (`config.js`):** Frontend mendukung `window.API_BASE_OVERRIDE`, meta tag `<meta name="api-base">`, auto-detection localhost port 8080, dan fallback VPS domain sehingga dapat langsung di-deploy ke Cloudflare Pages tanpa kendala *cross-origin* atau *404 routing*.

### B. Sisi Pemilik Toko (Dashboard Admin)
* **Konfirmasi Pelunasan Manual:** Tombol `POST /api/admin/orders/:id/mark-paid` pada modal rincian transaksi untuk menandai pesanan lunas secara manual, melepaskan stok akun ke pembeli, dan menembak email tanda terima secara otomatis.
* **Ekspor Laporan Transaksi ke CSV:** Tombol `GET /api/admin/orders/export?status=...` untuk mengekspor data pesanan ke format `.csv` sesuai filter status (Semua, Lunas, Pending, Expired, Cancelled).
* **Paginasi Tabel Pesanan:** Dukungan query `page` & `limit` pada backend dan kontrol tombol navigasi (`Sebelumnya`, `Halaman X dari Y`, `Selanjutnya`) pada frontend.
* **Inspeksi Pool Stok Produk:** Modal rincian stok (`GET /api/admin/products/:id/stocks`) untuk memantau ringkasan stok (Tersedia, Hold, Terjual) dan menghapus baris stok tertentu (`DELETE /api/admin/stocks/:id`) yang belum digunakan.

---

## 🏗️ 3. Topologi Arsitektur Sistem

```mermaid
graph TD
    subgraph BuyerClient ["Perangkat Pembeli (Mobile / Desktop)"]
        Browser["Web Browser (LocalStorage History)"]
    end

    subgraph CloudflareEdge ["Cloudflare Pages (Frontend Static CDN)"]
        FE_Home["/ (Katalog & Lacak Pesanan)"]
        FE_Product["/product/ (Detail Produk & Checkout)"]
        FE_Order["/order/ (QRIS Dinamis, SSE & Klaim)"]
        FE_Admin["/admin/ (Portal Manajemen Toko)"]
    end

    subgraph VPS ["VPS Host (Ubuntu / Debian, RAM < 60MB)"]
        subgraph StoreBackend ["TokoAsli Backend (Go Gin Port :8080)"]
            Router["Gin Engine & Middleware (IP Limiter, Admin Auth, CORS)"]
            SessionMgr["In-Memory Crypto SessionManager (TTL 24h)"]
            StockCleaner["Stock Hold & Expire Worker (30s Ticker)"]
            EmailWorker["Resend Email Service (Goroutine Non-Blocking)"]
            SSEBroadcaster["Zero-Leak SSE Hub (20s Keep-Alive Ping)"]
            StoreDB[(SQLite WAL: data/store.db)]
        end

        subgraph PaymentGInstance ["Payment Gateway (Port :3200 Internal)"]
            PGCore["PaymentG Engine"]
            ShopeePoller["ShopeePay Mutation Poller"]
            PGDB[(SQLite: data/payment.db)]
        end
    end

    subgraph ExternalServices ["Layanan Eksternal"]
        ResendAPI["Resend.com Email API"]
        ShopeeBackend["ShopeePay Core System"]
        WhatsAppAPI["WhatsApp CS Gateway (wa.me)"]
    end

    Browser -->|Buka Tampilan Web| CloudflareEdge
    Browser -->|Rest API & SSE Stream| Router
    Router -->|Autentikasi Sesi| SessionMgr
    Router -->|Query & Mutasi Data| StoreDB
    StockCleaner -->|Release Expired Stock & Broadcast| StoreDB
    StockCleaner -->|Kirim Sinyal Hangus| SSEBroadcaster
    Router -->|1. Create Order via X-API-Key| PGCore
    PGCore -->|Query Mutasi Akun| ShopeePoller
    ShopeePoller -->|Fetch Mutasi QRIS| ShopeeBackend
    PGCore -->|2. Webhook Lunas (HMAC-SHA256)| Router
    Router -->|3. Kirim Email Tanda Terima| ResendAPI
    Router -->|4. Push Sinyal Lunas via SSE| SSEBroadcaster
    SSEBroadcaster -->|Broadcast Status PAID| Browser
    Browser -->|5. Ambil Produk Terenkripsi Token| Router
    ResendAPI -->|Kirim Bukti & Claim Link| EmailBuyer["Kotak Masuk Email Pembeli"]
    Browser -->|Klik Bantuan CS| WhatsAppAPI
```

---

## 🗄️ 4. Skema Database SQLite WAL (`data/store.db`)

Database berjalan dengan konfigurasi performa tinggi SQLite WAL:
`_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)`

```sql
-- 1. TABEL PRODUK
CREATE TABLE IF NOT EXISTS products (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'Umum',
    price INTEGER NOT NULL,
    description TEXT,
    image_url TEXT,
    delivery_type TEXT NOT NULL DEFAULT 'pool', -- 'pool' (akun/serial unik) atau 'fixed' (link/file sama)
    fixed_content TEXT,                        -- Konten statis jika delivery_type = 'fixed'
    is_active INTEGER NOT NULL DEFAULT 1,      -- 1 = Aktif, 0 = Non-aktif / Soft-archived
    created_at TEXT NOT NULL,                  -- Format UTC RFC3339: YYYY-MM-DDTHH:MM:SSZ
    updated_at TEXT NOT NULL
);

-- 2. TABEL POOL STOK (VOUCHER / AKUN DIGITAL)
CREATE TABLE IF NOT EXISTS product_stocks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id TEXT NOT NULL,
    content TEXT NOT NULL,                     -- Kredensial akun 'user:pass' atau 'SERIAL-KEY'
    is_used INTEGER NOT NULL DEFAULT 0,        -- 0 = Tersedia, 1 = Terjual, 2 = Di-hold
    order_id TEXT,                             -- ID order yang memegang stok ini
    held_until TEXT,                           -- Batas waktu hold UTC RFC3339 (15 menit)
    created_at TEXT NOT NULL,
    used_at TEXT,
    FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
);

-- 3. TABEL PESANAN TOKO
CREATE TABLE IF NOT EXISTS store_orders (
    id TEXT PRIMARY KEY,                       -- Format: ORD-YYYYMMDD-XXXXXXXXXX (5-byte hex)
    secret_token TEXT NOT NULL UNIQUE,         -- Token kriptografi rahasia untuk klaim akun
    product_id TEXT NOT NULL,
    product_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    customer_phone TEXT,                       -- Nomor WhatsApp pembeli (opsional)
    original_price INTEGER NOT NULL,
    unique_code INTEGER NOT NULL,              -- 3-digit kode unik verifikasi
    total_amount INTEGER NOT NULL,             -- Total tagihan (original_price + unique_code)
    status TEXT NOT NULL DEFAULT 'PENDING',    -- 'PENDING', 'PAID', 'EXPIRED', 'CANCELLED'
    delivery_content TEXT,                     -- Data akun/voucher yang berhasil diklaim
    paymentg_order_id TEXT,                    -- ID referensi transaksi di PaymentG
    client_ip TEXT NOT NULL,
    created_at TEXT NOT NULL,                  -- Format UTC RFC3339
    paid_at TEXT,
    expires_at TEXT NOT NULL,                  -- Format UTC RFC3339 (created_at + 15 menit)
    FOREIGN KEY(product_id) REFERENCES products(id)
);

-- INDEKS EFISIENSI TINGGI
CREATE INDEX IF NOT EXISTS idx_products_active ON products(is_active);
CREATE INDEX IF NOT EXISTS idx_stocks_lookup ON product_stocks(product_id, is_used);
CREATE INDEX IF NOT EXISTS idx_stocks_held ON product_stocks(is_used, held_until);
CREATE INDEX IF NOT EXISTS idx_orders_status ON store_orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_secret ON store_orders(secret_token);
CREATE INDEX IF NOT EXISTS idx_orders_ip ON store_orders(client_ip, status);
CREATE INDEX IF NOT EXISTS idx_orders_created ON store_orders(created_at);
```

---

## 📡 5. Spesifikasi Lengkap API Endpoint Toko

### A. Endpoint Publik & Pembeli (Tanpa Autentikasi)

#### 1. Konfigurasi Toko & Kontak CS
* **Method & Path:** `GET /api/config`
* **Response `200 OK`:**
  ```json
  {
    "success": true,
    "data": {
      "whatsapp_number": "6281234567890",
      "currency": "IDR",
      "payment_expiry_minutes": 15
    }
  }
  ```

#### 2. Ambil Katalog Produk Aktif
* **Method & Path:** `GET /api/products`
* **Response `200 OK`:**
  ```json
  {
    "success": true,
    "data": [
      {
        "id": "prod_123456",
        "name": "Netflix Premium 1 Bulan",
        "category": "Streaming",
        "price": 35000,
        "description": "Garansi 30 hari anti-on hold.",
        "image_url": "/uploads/netflix.png",
        "delivery_type": "pool",
        "stock_count": 12,
        "is_active": true
      }
    ]
  }
  ```

#### 3. Detail Produk Mandiri
* **Method & Path:** `GET /api/products/:id`
* **Response `200 OK`:** Mengembalikan data lengkap satu produk aktif beserta sisa stoknya.

#### 4. Buat Pesanan Baru (Checkout)
* **Method & Path:** `POST /api/orders`
* **Request Body:**
  ```json
  {
    "product_id": "prod_123456",
    "customer_email": "buyer@gmail.com",
    "customer_phone": "081234567890"
  }
  ```
* **Alur Validasi & Eksekusi:**
  1. Validasi format email & pengecekan rate limit IP (maksimal 2 pesanan berstatus `PENDING` bersamaan per IP).
  2. Pengecekan ketersediaan stok produk (jika tipe `pool`, kunci 1 baris stok dengan `is_used = 2` dan `held_until = now + 15 min`).
  3. Memanggil API PaymentG `POST /api/v1/orders` untuk menghasilkan QRIS dinamis dan kode unik. Jika gagal, reservasi stok langsung di-rollback otomatis.
  4. Menyimpan record transaksi ke tabel `store_orders` dengan format UTC RFC3339.
* **Response `201 Created`:**
  ```json
  {
    "success": true,
    "data": {
      "order_id": "ORD-20261004-9AF32B1C05",
      "secret_token": "a1b2c3d4e5f6... (64 hex chars)",
      "product_name": "Netflix Premium 1 Bulan",
      "total_amount": 35123,
      "unique_code": 123,
      "qr_string": "00020101021226...",
      "qr_url": "http://localhost:3200/api/v1/qr/...",
      "expires_at": "2026-10-04T12:15:00Z"
    }
  }
  ```

#### 5. Ambil Detail Pesanan & Buka Produk Digital
* **Method & Path:** `GET /api/orders/:id?token=<secret_token>`
* **Response `200 OK` (Status `PAID`):**
  ```json
  {
    "success": true,
    "data": {
      "id": "ORD-20261004-9AF32B1C05",
      "product_name": "Netflix Premium 1 Bulan",
      "customer_email": "buyer@gmail.com",
      "customer_phone": "081234567890",
      "total_amount": 35123,
      "status": "PAID",
      "delivery_content": "Email: user1@stream.com | Sandi: Secret123",
      "paid_at": "2026-10-04T12:02:15Z"
    }
  }
  ```
  *(Catatan Keamanan: Jika `secret_token` tidak dikirim atau tidak cocok, field `delivery_content` dikosongkan).*

#### 6. Real-Time Status Stream (Zero-Leak SSE)
* **Method & Path:** `GET /api/orders/:id/stream?token=<secret_token>`
* **Header Response:**
  ```http
  Content-Type: text/event-stream
  Cache-Control: no-cache
  Connection: keep-alive
  X-Accel-Buffering: no
  ```
* **Event Stream Payload (Saat Pembayaran Diterima):**
  ```sse
  : ping

  data: {"order_id":"ORD-20261004-9AF32B1C05","status":"PAID"}
  ```
  *(Tidak ada kebocoran kredensial produk dalam stream SSE; client frontend otomatis memanggil endpoint terotentikasi no. 5 untuk mengambil akun).*

#### 7. Cek Status Pembayaran Manual
* **Method & Path:** `POST /api/orders/:id/check`
* **Fungsi:** Menghubungi gateway PaymentG secara on-demand untuk memeriksa mutasi ShopeePay jika webhook mengalami keterlambatan. Jika di gateway terdeteksi lunas, backend TokoAsli seketika menyelesaikan pesanan, mengunci stok, dan mengirim email.

#### 8. Batalkan Pesanan Mandiri
* **Method & Path:** `POST /api/orders/:id/cancel?token=<secret_token>`
* **Fungsi:** Mengubah status order menjadi `CANCELLED` dan seketika mengembalikan stok yang di-hold menjadi `is_used = 0` (tersedia) di katalog.

#### 9. Lacak Pesanan Digital
* **Method & Path:** `POST /api/orders/track`
* **Request Body:**
  ```json
  {
    "order_id": "ORD-20261004-9AF32B1C05",
    "customer_email": "buyer@gmail.com"
  }
  ```
* **Response `200 OK`:**
  ```json
  {
    "success": true,
    "data": {
      "order_id": "ORD-20261004-9AF32B1C05",
      "secret_token": "a1b2c3d4e5f6...",
      "product_name": "Netflix Premium 1 Bulan",
      "customer_email": "buyer@gmail.com",
      "status": "PAID",
      "total_amount": 35123,
      "created_at": "2026-10-04T12:00:00Z",
      "claim_url": "/order/?id=ORD-20261004-9AF32B1C05&token=a1b2c3d4e5f6..."
    }
  }
  ```

---

### B. Webhook API (Internal dari PaymentG)

#### Verifikasi Pelunasan Pembayaran
* **Method & Path:** `POST /api/webhooks/paymentg`
* **Headers:**
  ```http
  Content-Type: application/json
  X-Webhook-Event: payment.success
  X-Webhook-Signature: sha256=<hex_hmac_signature>
  ```
* **Request Payload dari PaymentG:**
  ```json
  {
    "event": "payment.success",
    "timestamp": 1728043335,
    "data": {
      "order_id": "ORD-20261004-9AF32B1C05",
      "amount": 35123,
      "status": "PAID",
      "payment_method": "SHOPEEPAY_QRIS"
    }
  }
  ```
* **Logika Backend:**
  1. Memverifikasi HMAC-SHA256 signature terhadap raw body menggunakan `WEBHOOK_SECRET`.
  2. Idempotency check: jika order sudah berstatus `PAID`, langsung kembalikan `200 OK`.
  3. Mengubah status order menjadi `PAID` dan mengikat konten produk ke `store_orders.delivery_content`.
  4. Menandai stok pool menjadi `is_used = 1` (terjual permanen) dan `used_at = UTC now`.
  5. Menembak sinyal realtime via SSE Hub: `{"order_id": "...", "status": "PAID"}`.
  6. Mengirim email tanda terima & link klaim via goroutine non-blocking (Resend API).

---

### C. Admin API (Dilindungi Token Sesi Kriptografis)

Seluruh request wajib menyertakan header:
`Authorization: Bearer <session_token>`

#### 1. Autentikasi Login Admin
* **Method & Path:** `POST /api/admin/login`
* **Request Body:** `{"password": "MasterAdminPassword"}`
* **Response `200 OK`:**
  ```json
  {
    "success": true,
    "token": "7f8e9d0a1b2c3d4e5f60718293a4b5c6... (64 hex characters)",
    "expires_in": 86400
  }
  ```

#### 2. Logout Admin
* **Method & Path:** `POST /api/admin/logout`
* **Fungsi:** Mencabut token sesi aktif dari `SessionManager`.

#### 3. Metrik & Statistik Bisnis
* **Method & Path:** `GET /api/admin/stats`
* **Response `200 OK`:**
  ```json
  {
    "success": true,
    "data": {
      "total_revenue": 1450000,
      "total_paid_orders": 42,
      "total_pending_orders": 3,
      "total_products": 8
    }
  }
  ```

#### 4. Manajemen Produk (CRUD)
* `GET /api/admin/products` — Mengambil seluruh produk (termasuk yang disembunyikan/non-aktif).
* `POST /api/admin/products` — Menambah produk baru.
* `PUT /api/admin/products/:id` — Memperbarui data produk.
* `DELETE /api/admin/products/:id` — Menghapus produk. Jika produk sudah memiliki riwayat transaksi, backend otomatis mengubah status menjadi `is_active = 0` (soft-archive) agar tidak terjadi crash foreign key.
* `PATCH /api/admin/products/:id/toggle` — Mengubah status aktif/non-aktif produk dalam 1 klik.

#### 5. Manajemen Pool Stok
* `GET /api/admin/products/:id/stocks` — Melihat ringkasan stok (`available`, `held`, `sold`, `total`) serta daftar seluruh item stok.
* `POST /api/admin/products/:id/stocks` — Bulk upload stok via format textarea (1 baris = 1 akun/voucher).
* `DELETE /api/admin/stocks/:id` — Menghapus 1 baris stok dari pool (hanya berlaku untuk stok yang berstatus `available`).

#### 6. Monitoring & Operasional Pesanan
* `GET /api/admin/orders?status=ALL&page=1&limit=20` — Mengambil daftar transaksi dengan paginasi.
  * Response menyertakan objek `pagination: {"total": 120, "page": 1, "limit": 20, "total_pages": 6}`.
* `POST /api/admin/orders/:id/mark-paid` — **Konfirmasi Lunas Manual:** Menandai pesanan lunas, mengalokasikan stok, dan menembak email tanda terima.
* `GET /api/admin/orders/export?status=ALL` — **Ekspor CSV:** Mengunduh file `orders_<status>_<date>.csv` dengan header lengkap.
* `POST /api/admin/orders/:id/resend-email` — Mengirim ulang tanda terima ke email pembeli.

#### 7. Unggah Foto Produk
* `POST /api/admin/upload` (Multipart form-data: `image`, maks 5MB) — Menyimpan file foto ke folder `data/uploads/` dan mengembalikan URL publik.

---

## 🔒 6. Matriks Keamanan & Hardening

| Vektor Ancaman | Mekanisme Pertahanan di TokoAsli v2.0 |
|---|---|
| **Eavesdropping Akun via SSE** | Data akun (`delivery_content`) dicabut 100% dari SSE broadcast. Akses kredensial wajib via endpoint terotentikasi token 64-karakter. |
| **Pencurian Password Admin** | Digantikan dengan token kriptografis in-memory 32-byte acak (TTL 24 jam). Tidak ada password mentah di storage browser. |
| **Timing Attacks pada Password** | Verifikasi password menggunakan `subtle.ConstantTimeCompare`. |
| **Pemalsuan Webhook Pembayaran** | Setiap payload webhook diverifikasi menggunakan tanda tangan digital HMAC-SHA256 (`X-Webhook-Signature`). |
| **Spam Checkout & Penguncian Stok** | Rate limiter membatasi maks 2 pesanan `PENDING` aktif per alamat IP pembeli. |
| **Deadlock & Busy Database** | SQLite dikonfigurasi dengan WAL Mode (`journal_mode=WAL`), `busy_timeout=5000` (5 detik antrean), dan pooling koneksi Go `SetMaxOpenConns(1)`. |
| **Reverse Proxy Timeout** | Keep-alive loop 20 detik (`: ping\n\n`) menjaga koneksi SSE tetap hidup di Cloudflare dan Nginx. |
| **Cross-Site Scripting (XSS)** | Seluruh string dinamis yang dirender ke DOM frontend diproteksi dengan fungsi sanitasi `escapeHtml()`. |

---

## 🚀 7. Panduan Deployment VPS & Cloudflare Pages

### A. Deploy Backend di VPS (Ubuntu / Debian)

1. **Persiapan Direktori & Binary:**
   ```bash
   mkdir -p /var/www/tokoasli/data/uploads
   cd /var/www/tokoasli
   # Salin binary tokoasli yang telah dikompilasi
   chmod +x tokoasli
   ```

2. **File Konfigurasi Environment (`/var/www/tokoasli/.env`):**
   ```env
   PORT=8080
   DB_PATH=data/store.db
   ADMIN_PASSWORD=GantiDenganPasswordSuperKuat2026!
   PAYMENTG_BASE_URL=http://localhost:3200
   PAYMENTG_API_KEY=app_live_xxxxxxxxxxxxxxxx
   WEBHOOK_SECRET=whsec_xxxxxxxxxxxxxxxx
   RESEND_API_KEY=re_xxxxxxxxxxxxxxxx
   RESEND_FROM_EMAIL=TokoAsli <noreply@domainanda.com>
   WHATSAPP_NUMBER=6281234567890
   CORS_ORIGINS=https://tokoanda.com,https://*.pages.dev,http://localhost:8080
   ```

3. **Service Systemd (`/etc/systemd/system/tokoasli.service`):**
   ```ini
   [Unit]
   Description=TokoAsli Backend Service
   After=network.target paymentg.service

   [Service]
   Type=simple
   User=www-data
   Group=www-data
   WorkingDirectory=/var/www/tokoasli
   ExecStart=/var/www/tokoasli/tokoasli
   Restart=always
   RestartSec=5s
   LimitNOFILE=65535

   [Install]
   WantedBy=multi-user.target
   ```
   Aktifkan service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now tokoasli
   ```

4. **Konfigurasi Reverse Proxy Nginx (`/etc/nginx/sites-available/api.tokoanda.com`):**
   ```nginx
   server {
       server_name api.tokoanda.com;

       location / {
           proxy_pass http://127.0.0.1:8080;
           proxy_http_version 1.1;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;

           # Konfigurasi Wajib untuk Server-Sent Events (SSE)
           proxy_set_header Connection '';
           proxy_buffering off;
           proxy_cache off;
           proxy_read_timeout 3600s;
           chunked_transfer_encoding off;
       }
   }
   ```

### B. Deploy Frontend di Cloudflare Pages (Gratis Selamanya)

1. Buat project baru di **Cloudflare Dashboard ➔ Workers & Pages ➔ Pages ➔ Create a project**.
2. Hubungkan repositori GitHub atau upload langsung folder:
   **Folder Output:** `tokoasli/fe-toko/`
3. Konfigurasikan API Endpoint di `fe-toko/assets/js/config.js` atau tambahkan meta tag pada seluruh file HTML:
   ```html
   <meta name="api-base" content="https://api.tokoanda.com">
   ```
4. Tambahkan file `_redirects` pada root `fe-toko/` untuk clean URL handling:
   ```text
   /order/*    /order/index.html   200
   /product/*  /product/index.html 200
   /admin/*    /admin/index.html   200
   /*          /index.html         200
   ```

---

## 🛠️ 8. Prosedur Pemeliharaan & Operasional

### A. Backup Database Otomatis (Cron)
Buat cron job harian untuk backup aman SQLite WAL menggunakan perintah `.backup`:
```bash
0 3 * * * sqlite3 /var/www/tokoasli/data/store.db ".backup '/var/backups/tokoasli_$(date +\%Y\%m\%d).db'"
```

### B. Monitoring Log Transaksi Realtime
```bash
journalctl -u tokoasli -f -n 100
```

### C. Pengujian End-to-End di Mesin Lokal
```bash
# 1. Jalankan backend TokoAsli
cd C:\laragon\www\paymentg\tokoasli
go run .

# 2. Buka antarmuka web di browser
http://localhost:8080/
# Halaman Lacak Pesanan: http://localhost:8080/ (Klik tombol '🔍 Lacak Pesanan')
# Detail Produk:        http://localhost:8080/product/?id=...
# Portal Admin:         http://localhost:8080/admin/
```

---

*Cetak biru TokoAsli v2.0 ini disiapkan secara komprehensif untuk pengoperasian stabil, aman, dan tanpa pemeliharaan manual berlebih.*
