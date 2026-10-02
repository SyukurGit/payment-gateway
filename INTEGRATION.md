# 🤖 PaymentG Integration Guide for AI Agents & Developers

Panduan ini dirancang khusus sebagai **instruksi lengkap dan presisi untuk AI Coding Agent (seperti Antigravity, Cursor, Claude Code, Copilot, dll.)** maupun software engineer yang akan mengintegrasikan sistem pembayaran QRIS PaymentG ke dalam website jual-beli (e-commerce).

---

## 📌 Context for AI Agent
> **Instruction for AI Assistant:**
> When building or modifying a website to support PaymentG QRIS payments, follow this specification strictly.
> 1. Store credentials in `.env`.
> 2. Create the Order creation endpoint that calls PaymentG `POST /api/orders`.
> 3. Display the QR code and the **EXACT total amount** (including the unique code) to the customer.
> 4. Implement the Webhook receiver endpoint with **HMAC-SHA256 signature verification**.
> 5. Implement client-side polling or status checking to update the UI in real-time when paid.

---

## ⚙️ 1. Environment Variables (Kredensial Toko)

Tambahkan variabel lingkungan berikut pada file `.env` di website toko Anda:

```env
# ==============================================================================
# PILIH SALAH SATU MODE: TESTING (SANDBOX) vs PRODUCTION (LIVE)
# ==============================================================================

# 🧪 OPSI A: MODE TESTING (SANDBOX SIMULATOR)
# Gunakan ini saat develop toko baru. Tidak ada uang asli, bisa klik bayar lewat web!
PAYMENTG_BASE_URL=https://paymentg.syukurapi.online/api/sandbox
PAYMENTG_API_KEY=ak_sbx_xxxxxxxxxxxxxxxxxxxxxxxx
PAYMENTG_WEBHOOK_SECRET=sbx_sec_xxxxxxxxxxxxxxxxxxxxxxxx

# 🚀 OPSI B: MODE LIVE (UANG ASLI)
# Cukup tukar nilai ini saat toko Anda sudah siap jualan uang asli (kode toko 100% sama!):
# PAYMENTG_BASE_URL=https://paymentg.syukurapi.online/api
# PAYMENTG_API_KEY=ak_xxxxxxxxxxxxxxxxxxxxxxxx
# PAYMENTG_WEBHOOK_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

---

## 🌐 2. API Contract (Spesifikasi Request & Response)

Semua komunikasi dari Web Toko ke PaymentG menggunakan format **JSON** melalui HTTP REST.

### A. Buat Tagihan QRIS (`POST /api/orders`)
Dipanggil oleh backend web toko Anda saat pembeli memilih metode bayar QRIS dan menekan tombol *"Bayar"*.

* **URL:** `{PAYMENTG_BASE_URL}/api/orders`
* **Method:** `POST`
* **Headers:**
  ```http
  Content-Type: application/json
  X-API-Key: {PAYMENTG_API_KEY}
  ```
* **Request Body (JSON):**
  ```json
  {
    "amount": 50000,
    "reference_id": "INV-2026-0001",
    "expiry_minutes": 15,
    "metadata": "User: Budi | Produk: Item ABC"
  }
  ```
  * `amount` *(integer, wajib)*: Harga asli produk dalam Rupiah (tanpa titik/koma).
  * `reference_id` *(string, opsional)*: Nomor Invoice / Order ID unik dari database web toko Anda.
  * `expiry_minutes` *(integer, opsional, default: 15)*: Durasi kedaluwarsa pesanan dalam menit.
  * `metadata` *(string, opsional)*: Catatan tambahan transaksi.

* **Response Success (HTTP 201 Created):**
  ```json
  {
    "success": true,
    "data": {
      "order_id": "ord_oVVqMw7cBx",
      "reference_id": "INV-2026-0001",
      "original_amount": 50000,
      "unique_code": 237,
      "total_amount": 50237,
      "status": "PENDING",
      "qr_url": "/api/orders/ord_oVVqMw7cBx/qr.png",
      "expires_at": "2026-10-02T16:30:00Z",
      "expires_in_seconds": 900
    }
  }
  ```
* **Data Penting untuk Frontend:**
  * `total_amount`: **Nominal wajib yang harus dibayar pembeli** (Rp 50.237). Harus ditonjolkan di UI toko!
  * `qr_url`: Gambar QRIS PNG. URL lengkapnya adalah `{PAYMENTG_BASE_URL}{qr_url}`.

---

### B. Ambil Gambar QRIS (`GET /api/orders/:id/qr.png`)
URL publik untuk menampilkan gambar QR code langsung di HTML tanpa memerlukan header otorisasi.

* **URL:** `{PAYMENTG_BASE_URL}/api/orders/{order_id}/qr.png`
* **Method:** `GET`
* **Content-Type:** `image/png`
* **Contoh di HTML Frontend:**
  ```html
  <img src="https://paymentg.syukurapi.online/api/orders/ord_oVVqMw7cBx/qr.png" alt="Scan QRIS" width="240" />
  ```

---

### C. Cek Status Pesanan Manual (`GET /api/orders/:id`)
Dipanggil oleh frontend atau backend web toko untuk memeriksa apakah pesanan sudah lunas.

* **URL:** `{PAYMENTG_BASE_URL}/api/orders/{order_id}`
* **Method:** `GET`
* **Headers:** `X-API-Key: {PAYMENTG_API_KEY}`
* **Response (HTTP 200 OK):**
  ```json
  {
    "success": true,
    "data": {
      "order_id": "ord_oVVqMw7cBx",
      "reference_id": "INV-2026-0001",
      "original_amount": 50000,
      "unique_code": 237,
      "total_amount": 50237,
      "status": "PAID",
      "paid_at": "2026-10-02T16:18:24Z",
      "shopee_tx_id": "122722636469377153",
      "created_at": "2026-10-02T16:15:00Z"
    }
  }
  ```
  *Nilai `status`:*
  * `PENDING` ➔ Menunggu pembayaran pembeli.
  * `PAID` ➔ Sudah dibayar lunas & diverifikasi masuk ShopeePay.
  * `EXPIRED` ➔ Sudah lewat batas waktu (misal >15 menit).
  * `CANCELLED` ➔ Dibatalkan.

---

### D. Trigger Verifikasi Instan / Tombol "Saya Sudah Bayar" (`POST /api/orders/:id/check`)
Jika pembeli menekan tombol *"Saya Sudah Bayar"* di web Anda, panggil endpoint ini untuk langsung memeriksa mutasi ShopeePay secara instan tanpa menunggu siklus polling otomatis.

* **URL:** `{PAYMENTG_BASE_URL}/api/orders/{order_id}/check`
* **Method:** `POST`
* **Headers:** `X-API-Key: {PAYMENTG_API_KEY}`
* **Response (HTTP 200 OK):** Mengembalikan objek order terbaru (status `PAID` atau tetap `PENDING` jika belum terdeteksi).

---

### E. Batalkan Pesanan (`POST /api/orders/:id/cancel`)
Jika pembeli menekan tombol *"Ganti Metode Pembayaran"* atau *"Batal"*. Nominal unik akan langsung dilepas agar bisa dipakai transaksi lain.

* **URL:** `{PAYMENTG_BASE_URL}/api/orders/{order_id}/cancel`
* **Method:** `POST`
* **Headers:** `X-API-Key: {PAYMENTG_API_KEY}`

---

## 🔔 3. Webhook Receiver Specification (Wajib Diimplementasikan di Web Toko)

PaymentG akan otomatis mengirimkan notifikasi HTTP POST ke URL Webhook website Anda saat pembayaran berhasil diverifikasi.

### Header yang Dikirim oleh PaymentG:
```http
Content-Type: application/json
X-Webhook-Event: payment.success
X-Webhook-Signature: sha256=<hex_hmac_sha256>
```

### Payload Body yang Dikirim oleh PaymentG:
```json
{
  "event": "payment.success",
  "order_id": "ord_oVVqMw7cBx",
  "reference_id": "INV-2026-0001",
  "original_amount": 50000,
  "unique_code": 237,
  "total_amount": 50237,
  "paid_at": "2026-10-02T16:18:24Z",
  "shopee_tx_id": "122722636469377153"
}
```

### 🔒 Aturan Keamanan Verifikasi Tanda Tangan (HMAC-SHA256):
Backend web toko Anda **wajib memverifikasi signature** sebelum mengubah status pesanan di database:
1. Ambil **RAW request body** (teks JSON asli mentah, bukan yang sudah di-parse).
2. Hitung HMAC-SHA256 dari teks mentah tersebut menggunakan `PAYMENTG_WEBHOOK_SECRET`.
3. Bandingkan dengan header `X-Webhook-Signature` menggunakan algoritma *timing-safe equal*.

---

## 💻 4. Template Kode Implementasi Siap Pakai

### Pilihan A: Implementasi PHP / Laravel

#### 1. Webhook Controller (`PaymentWebhookController.php`)
```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use App\Models\Order; // Model pesanan toko Anda

class PaymentWebhookController extends Controller
{
    public function handle(Request $request)
    {
        $secret = env('PAYMENTG_WEBHOOK_SECRET');
        $signatureHeader = $request->header('X-Webhook-Signature');
        $rawPayload = $request->getContent();

        // 1. Verifikasi HMAC-SHA256
        $expectedSignature = 'sha256=' . hash_hmac('sha256', $rawPayload, $secret);
        if (!hash_equals($expectedSignature, (string)$signatureHeader)) {
            return response()->json(['error' => 'Invalid signature'], 401);
        }

        // 2. Baca data pesanan
        $data = json_decode($rawPayload, true);
        if (($data['event'] ?? '') === 'payment.success') {
            $invoiceNumber = $data['reference_id'];
            $shopeeTxId    = $data['shopee_tx_id'];
            $totalPaid     = $data['total_amount'];

            // 3. Update database pesanan web Anda
            $order = Order::where('invoice_number', $invoiceNumber)->first();
            if ($order && $order->status !== 'PAID') {
                $order->update([
                    'status'        => 'PAID',
                    'paid_at'       => now(),
                    'payment_tx_id' => $shopeeTxId,
                ]);

                // TODO: Kirim notifikasi WhatsApp / Email ke pembeli atau aktifkan produk
            }
        }

        return response()->json(['status' => 'ok']);
    }
}
```

---

### Pilihan B: Implementasi Node.js / Express / Next.js

#### Webhook Handler (`webhook.js` / Route API)
```javascript
const express = require('express');
const crypto = require('crypto');
const router = express.Router();

// PENTING: Gunakan express.raw({ type: 'application/json' }) untuk membaca raw body
router.post('/api/webhook/paymentg', express.raw({ type: 'application/json' }), async (req, res) => {
    const secret = process.env.PAYMENTG_WEBHOOK_SECRET;
    const signature = req.headers['x-webhook-signature'];
    const rawBody = req.body.toString('utf-8');

    // 1. Verifikasi HMAC
    const expectedSignature = 'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex');
    
    if (signature !== expectedSignature) {
        return res.status(401).json({ error: 'Invalid signature' });
    }

    // 2. Proses pembayaran
    const payload = JSON.parse(rawBody);
    if (payload.event === 'payment.success') {
        const invoiceId = payload.reference_id;
        const shopeeTxId = payload.shopee_tx_id;
        
        console.log(`[PAYMENT SUCCESS] Invoice ${invoiceId} lunas! Shopee TX: ${shopeeTxId}`);
        // TODO: Update database toko Anda -> set status = 'PAID'
    }

    return res.status(200).json({ status: 'ok' });
});

module.exports = router;
```

---

## 🎨 5. Alur UI Checkout Frontend yang Direkomendasikan

Saat pembeli berada di halaman pembayaran:
1. **Tampilkan Nominal Tepat:**
   * Tampilkan nominal dengan jelas, beri label penekanan: *"Transfer Tepat Rp 50.237 (termasuk 3 digit kode unik)"*.
2. **Tampilkan Gambar QRIS:**
   * `<img src="http://api-url/api/orders/{order_id}/qr.png" width="260" />`
3. **Pasang Realtime Polling (Interval 3 Detik):**
   ```javascript
   const timer = setInterval(async () => {
       const res = await fetch(`/api/orders/status?order_id=${orderId}`);
       const data = await res.json();
       if (data.status === 'PAID') {
           clearInterval(timer);
           // Sembunyikan QRIS, tampilkan pesan sukses / redirect
           window.location.href = `/checkout/success?invoice=${invoiceId}`;
       }
   }, 3000);
   ```
4. **Pasang Countdown Timer (15 Menit):**
   * Jika waktu habis, ubah tombol menjadi *"Tagihan Kedaluwarsa, Buat Tagihan Baru"*.

---

## 🧪 6. Cara Menguji Pembayaran Tanpa Uang Asli (Mode Sandbox)

Saat Anda sedang mengembangkan website toko, Anda tidak perlu melakukan transfer uang asli berulang kali! Cukup gunakan fitur **Sandbox Simulator**:

1. **Buka Dashboard Sandbox di Browser:**
   Akses `https://paymentg.syukurapi.online/sandbox/` (atau `http://localhost:3200/sandbox/`). Masukkan PIN `2207`.
2. **Daftarkan App Testing:**
   Buka tab **"🔑 Kelola App Sandbox"**, daftarkan toko testing Anda dan webhook URL-nya (misal `http://localhost:8000/api/webhook` atau `https://webhook.site/...`). Salin `API Key (ak_sbx_...)` dan `Webhook Secret` ke `.env` web toko Anda.
3. **Lakukan Checkout di Web Toko:**
   Web toko Anda akan memanggil `POST {PAYMENTG_BASE_URL}/orders` (ingat: `PAYMENTG_BASE_URL` diisi dengan akhiran `/api/sandbox`).
4. **Simulasikan Pembayaran Lunas dengan 1 Klik:**
   Buka kembali halaman `/sandbox/` di tab **"📜 Mutasi Transaksi Sandbox"**. Tagihan Anda akan muncul dengan status `PENDING`.
   Klik tombol hijau: **`[💳 Bayar Sekarang]`**!
5. **Lihat Hasilnya:**
   * PaymentG akan seketika menembakkan Webhook nyata ber-signature HMAC-SHA256 ke web toko Anda.
   * Webhook inspector di halaman `/sandbox/` akan memperlihatkan status respons dari server toko Anda (misal HTTP 200 OK).
   * Halaman checkout pembeli di web toko Anda otomatis mendeteksi lunas dan melakukan redirect sukses!

---

## 🚀 Checklist untuk AI Agent

Saat mengintegrasikan ke web toko baru, pastikan hal berikut tercentang:
- [ ] File `.env` sudah memuat `PAYMENTG_BASE_URL`, `PAYMENTG_API_KEY`, dan `PAYMENTG_WEBHOOK_SECRET`.
- [ ] Endpoint pembuatan order memanggil `POST {PAYMENTG_BASE_URL}/orders` dan menangani error jika server pembayaran offline.
- [ ] Halaman checkout menampilkan `total_amount` (nominal akhir yang sudah ada kode uniknya), bukan harga aslinya.
- [ ] Endpoint Webhook memvalidasi header `X-Webhook-Signature` dengan `hash_equals` / timing-safe crypto.
- [ ] Webhook membalas HTTP `200 OK` agar server PaymentG tahu notifikasi sudah sampai.
- [ ] URL Webhook toko Anda didaftarkan di Dashboard PaymentG (Tab *"Kelola Web App"* untuk Live, atau Tab *"Kelola App Sandbox"* di `/sandbox` untuk Testing).
