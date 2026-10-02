# 📖 PaymentG — ShopeePay QRIS Gateway API Documentation

PaymentG adalah self-hosted QRIS payment gateway mandiri berkinerja tinggi (Go + Gin + SQLite). Sistem ini mengubah QRIS statis merchant ShopeePay menjadi QRIS dinamis ber-nominal unik secara real-time, mendeteksi mutasi masuk secara otomatis via polling dashboard ShopeePay, dan mengirim notifikasi Webhook ke website jual-beli Anda.

---

## 🚀 Quick Start (Cara Tercepat Pakai)

### 1. Jalankan Service
Buka terminal di folder project:
```powershell
.\paymentg.exe
```
Service akan aktif di `http://localhost:3200`.

### 2. Dapatkan Kredensial untuk Web Anda (Sekali Saja)
Jalankan perintah ini untuk mendaftarkan web toko Anda:
```bash
curl -X POST http://localhost:3200/api/apps \
  -H "X-Admin-Key: adm_secret_paymentg_2026" \
  -H "Content-Type: application/json" \
  -d '{"name": "Web Toko Saya", "webhook_url": "https://webtoko.com/api/webhook"}'
```
Simpan `api_key` dan `webhook_secret` dari respons JSON.

### 3. Buat Tagihan QRIS Saat Checkout
Setiap kali ada pembeli mau bayar:
```bash
curl -X POST http://localhost:3200/api/orders \
  -H "X-API-Key: <api_key_anda>" \
  -H "Content-Type: application/json" \
  -d '{"reference_id": "INV-1001", "amount": 25000, "expiry_minutes": 15}'
```
Respons akan memberikan `total_amount` (misal `25362`) dan `qr_url` untuk ditampilkan ke pembeli:
```html
<img src="http://localhost:3200/api/orders/ord_xxxx/qr.png" alt="QRIS Pembayaran" width="280" />
```

---

## 🔐 Sistem Otentikasi

Semua request wajib menyertakan salah satu header otentikasi berikut:

| Header | Ditujukan Untuk | Deskripsi |
| :--- | :--- | :--- |
| `X-Admin-Key` | Pemilik Server | Mengelola token ShopeePay, mendaftarkan app baru, dan memonitor status gateway. Disetel di `.env` (`ADMIN_KEY`). |
| `X-API-Key` | Web Client / Toko | Membuat order tagihan QRIS, mengecek status pembayaran, dan membatalkan pesanan. |

---

## 📋 Daftar Endpoint Lengkap

### A. Admin Endpoints (`X-Admin-Key`)

#### 1. Cek Kesehatan & Status Token
* **URL:** `GET /api/health`
* **Header:** Tidak wajib / Publik
* **Respons Contoh (200 OK):**
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "token_valid": true,
    "token_updated_at": "2026-10-02T16:11:24+07:00",
    "token_age_hours": 1.25,
    "pending_orders": 2,
    "last_poll_at": "2026-10-02T16:15:30+07:00",
    "last_poll_success": true,
    "message": ""
  }
}
```

#### 2. Update Token ShopeePay
Digunakan saat session browser ShopeePay Anda habis tanpa perlu restart server.
* **URL:** `PUT /api/config/token`
* **Header:** `X-Admin-Key: adm_secret_paymentg_2026`
* **Body:**
```json
{
  "token": "B:EMLvEFtuXiS2a/eWRNs6iKtT5..."
}
```
* **Respons (200 OK):**
```json
{
  "success": true,
  "data": "token updated successfully"
}
```

#### 3. Daftarkan Web Client Baru
* **URL:** `POST /api/apps`
* **Header:** `X-Admin-Key: adm_secret_paymentg_2026`
* **Body:**
```json
{
  "name": "Toko Donasi Online",
  "webhook_url": "https://tokoku.com/webhook/payment"
}
```
* **Respons (201 Created):**
```json
{
  "success": true,
  "data": {
    "id": "ord_CNlhTuaTki",
    "name": "Toko Donasi Online",
    "api_key": "ak_iFV5BsdprjxCb4AnpkCkrYzc",
    "webhook_url": "https://tokoku.com/webhook/payment",
    "webhook_secret": "G4WK3Sb0Pd1ZmHSd2TzSunEl6rv08omQ"
  }
}
```

#### 4. List Semua Web Client
* **URL:** `GET /api/apps`
* **Header:** `X-Admin-Key: adm_secret_paymentg_2026`

#### 5. Hapus Web Client
* **URL:** `DELETE /api/apps/:id`
* **Header:** `X-Admin-Key: adm_secret_paymentg_2026`

---

### B. Client / Order Endpoints (`X-API-Key`)

#### 1. Buat Order Pembayaran Baru
* **URL:** `POST /api/orders`
* **Header:** `X-API-Key: ak_xxxxxxxxxxxx`
* **Body:**
```json
{
  "reference_id": "INV-2026-0001",
  "amount": 50000,
  "expiry_minutes": 15,
  "metadata": "User: John Doe | Pulsa 50k"
}
```

* **Parameter:**
  - `amount` *(int64, Wajib)*: Nominal dasar tagihan dalam Rupiah.
  - `reference_id` *(string, Opsional)*: No. invoice / ID unik dari website Anda.
  - `expiry_minutes` *(int, Opsional)*: Durasi kedaluwarsa QR dalam menit (default: 15 menit).
  - `metadata` *(string, Opsional)*: Catatan tambahan transaksi.

* **Respons (201 Created):**
```json
{
  "success": true,
  "data": {
    "order_id": "ord_oVVqMw7cBx",
    "reference_id": "INV-2026-0001",
    "original_amount": 50000,
    "unique_code": 362,
    "total_amount": 50362,
    "status": "PENDING",
    "qr_url": "/api/orders/ord_oVVqMw7cBx/qr.png",
    "expires_at": "2026-10-02T16:30:00+07:00",
    "expires_in_seconds": 900
  }
}
```

#### 2. Dapatkan Gambar QR Code PNG
* **URL:** `GET /api/orders/:id/qr.png`
* **Header:** `X-API-Key: ak_xxxxxxxxxxxx`
* **Content-Type:** `image/png`
* **Catatan:** Bisa langsung disematkan pada tag HTML `<img src="http://localhost:3200/api/orders/ord_xxx/qr.png" />`.

#### 3. Cek Status Order
* **URL:** `GET /api/orders/:id`
* **Header:** `X-API-Key: ak_xxxxxxxxxxxx`
* **Respons (200 OK):**
```json
{
  "success": true,
  "data": {
    "order_id": "ord_oVVqMw7cBx",
    "reference_id": "INV-2026-0001",
    "original_amount": 50000,
    "unique_code": 362,
    "total_amount": 50362,
    "status": "PAID",
    "paid_at": "2026-10-02T16:18:24+07:00",
    "shopee_tx_id": "118902602337672307",
    "created_at": "2026-10-02T16:15:00+07:00"
  }
}
```
*Nilai status:* `PENDING`, `PAID`, `EXPIRED`, `CANCELLED`.

#### 4. Force Check Transaksi (Manual Trigger)
Jika pembeli menekan tombol *"Saya Sudah Bayar"* di website Anda, endpoint ini langsung memicu pengecekan ke ShopeePay tanpa menunggu interval polling.
* **URL:** `POST /api/orders/:id/check`
* **Header:** `X-API-Key: ak_xxxxxxxxxxxx`

#### 5. Batalkan Order
Membatalkan pesanan dan langsung me-release nominal unik agar bisa dipakai transaksi lain.
* **URL:** `POST /api/orders/:id/cancel`
* **Header:** `X-API-Key: ak_xxxxxxxxxxxx`

---

## 🔔 Webhook Notifikasi

Ketika pembayaran berhasil diverifikasi oleh poller, PaymentG mengirim HTTP POST request ke `webhook_url` web Anda.

### Headers Webhook:
```http
Content-Type: application/json
X-Webhook-Event: payment.success
X-Webhook-Signature: sha256=<hex_hmac_sha256>
```

### Payload Webhook:
```json
{
  "event": "payment.success",
  "order_id": "ord_oVVqMw7cBx",
  "reference_id": "INV-2026-0001",
  "original_amount": 50000,
  "unique_code": 362,
  "total_amount": 50362,
  "paid_at": "2026-10-02T16:18:24+07:00",
  "shopee_tx_id": "118902602337672307"
}
```

### Verifikasi Webhook di Website Anda (PHP):
```php
<?php
// webhook.php di web toko Anda
$secret = "G4WK3Sb0Pd1ZmHSd2TzSunEl6rv08omQ"; // webhook_secret saat buat app

$rawBody = file_get_contents("php://input");
$signatureHeader = $_SERVER['HTTP_X_WEBHOOK_SIGNATURE'] ?? '';

// Verifikasi HMAC-SHA256 signature
$expectedSignature = "sha256=" . hash_hmac('sha256', $rawBody, $secret);

if (!hash_equals($expectedSignature, $signatureHeader)) {
    http_response_code(401);
    die("Invalid signature");
}

$data = json_decode($rawBody, true);
if ($data['event'] === 'payment.success') {
    $orderId = $data['order_id'];
    $invoice = $data['reference_id'];
    $totalPaid = $data['total_amount'];
    
    // TODO: Update status pesanan di database toko Anda menjadi SUDAH BAYAR
    // order_set_paid($invoice, $totalPaid);
}

http_response_code(200);
echo json_encode(["status" => "ok"]);
```

---

## 🔖 1-Click Bookmarklet Ambil Token ShopeePay

Untuk mempermudah update token tanpa buka Inspect Element:
1. Buat Bookmark baru di browser Anda.
2. Beri nama: **Update Token PaymentG**.
3. Isi URL dengan script di bawah ini:
```javascript
javascript:void(function(){var d=window.injectData||window['injectData'];if(d&&d.User&&d.User.token){fetch('http://localhost:3200/api/config/token',{method:'PUT',headers:{'Content-Type':'application/json','X-Admin-Key':'adm_secret_paymentg_2026'},body:JSON.stringify({token:d.User.token})}).then(r=>r.json()).then(j=>{alert('Token ShopeePay berhasil dikirim ke PaymentG!')}).catch(e=>alert('Gagal mengirim token: '+e))}else{alert('Token tidak ditemukan. Pastikan Anda sedang membuka partner.shopee.co.id!')}}())
```
4. Setiap kali login di `https://partner.shopee.co.id/`, cukup klik bookmarklet ini dan token otomatis terkirim ke PaymentG!

---

## 🤖 Otomasi Auto-Refresh Token via Playwright (Headless)

Selain bookmarklet, tersedia script otomatisasi Playwright yang hemat RAM dan anti-deteksi bot:
* **Script:** [`refresh_token.py`](file:///C:/laragon/www/paymentg/refresh_token.py)
* **Pola:** Ephemeral (hanya jalan 3–5 detik saat dibutuhkan, idle RAM 0 MB).
* **Setup awal (Login 1x saja):**
  ```bash
  python refresh_token.py --setup
  ```
  Jendela browser akan terbuka. Login akun Shopee Partner Anda sekali saja. Session cookies akan tersimpan permanen di folder `data/browser_profile/`.
* **Jalan Otomatis:**
  Jika poller Go mendeteksi token mati/kadaluwarsa, server Go otomatis memanggil `python refresh_token.py` di latar belakang untuk memperbarui token ke database secara otomatis.

---

## 🖥️ Panduan Deploy VPS Linux

1. **Build binary untuk Linux di komputer lokal:**
   ```bash
   GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o paymentg .
   ```
2. **Kirim binary & `.env` ke VPS:**
   ```bash
   scp paymentg .env user@your-vps-ip:/opt/paymentg/
   ```
3. **Jalankan via Systemd (`/etc/systemd/system/paymentg.service`):**
   ```ini
   [Unit]
   Description=PaymentG ShopeePay QRIS Gateway
   After=network.target

   [Service]
   Type=simple
   WorkingDirectory=/opt/paymentg
   ExecStart=/opt/paymentg/paymentg
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
4. **Aktifkan:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now paymentg
   ```
