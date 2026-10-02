# 🔌 PaymentG Integration Guide (AI Agent & Developer)

Panduan integrasi teknis presisi untuk **AI Coding Agent** (Antigravity, Cursor, Claude Code, Copilot) maupun **Software Engineer** yang menghubungkan website e-commerce dengan gateway pembayaran QRIS **PaymentG**.

---

## 📌 Context for AI Agent

> **Agent Instruction:**
> When integrating an e-commerce website with PaymentG:
> 1. Store credentials in `.env` using `PAYMENTG_API_URL` and `PAYMENTG_API_KEY`.
> 2. Implement the checkout order creation calling `POST {PAYMENTG_API_URL}/orders`.
> 3. Display the QRIS image and highlight the **EXACT `total_amount`** (including the unique code).
> 4. Create an idempotent Webhook receiver endpoint that listens for `payment.success` and returns HTTP 200 OK.
> 5. Enable seamless switching between Sandbox and Live simply by swapping `.env` values.

---

## 🔄 Zero-Code Switching (Sandbox ⇄ Live)

Integrasi PaymentG dirancang dengan arsitektur **Zero-Code Switching**. Kode checkout dan webhook di toko Anda **100% sama**, Anda hanya perlu mengganti nilai file `.env` di website toko:

### Mode Testing (Sandbox Simulator)
```env
PAYMENTG_API_URL=https://paymentg.syukurapi.online/api/sandbox
PAYMENTG_API_KEY=ak_sbx_xxxxxxxxxxxxxxxxxxxxxxxx
```
*(Bisa disimulasikan lunas 1-klik di dashboard `/sandbox` tanpa uang asli).*

### Mode Production (Uang Asli)
```env
PAYMENTG_API_URL=https://paymentg.syukurapi.online/api
PAYMENTG_API_KEY=ak_xxxxxxxxxxxxxxxxxxxxxxxx
```
*(Terhubung langsung ke mutasi uang riil ShopeePay).*

---

## 📡 Kontrak API Toko

Semua request menggunakan format JSON standar dan menyertakan header `X-API-Key`.

### 1. Buat Tagihan QRIS (`POST {PAYMENTG_API_URL}/orders`)

Dipanggil oleh backend toko saat pembeli menekan tombol *"Bayar dengan QRIS"*.

#### Request
- **Method:** `POST`
- **Headers:**
  ```http
  Content-Type: application/json
  X-API-Key: {PAYMENTG_API_KEY}
  ```
- **Body:**
  ```json
  {
    "reference_id": "INV-2026-0001",
    "amount": 50000,
    "expiry_minutes": 15,
    "metadata": "User: Budi | Paket Pro"
  }
  ```
  - `amount` *(integer, Wajib)*: Nominal dasar tagihan dalam Rupiah.
  - `reference_id` *(string, Wajib)*: ID pesanan / nomor invoice unik dari database toko Anda.
  - `expiry_minutes` *(integer, Opsional)*: Durasi aktif tagihan dalam menit (default: 15).
  - `metadata` *(string, Opsional)*: Keterangan tambahan transaksi.

#### Response (HTTP 201 Created)
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

> ⚠️ **KRITIS UNTUK FRONTEND TOKO:**
> Pembeli **wajib** mentransfer tepat sejumlah **`total_amount`** (Rp 50.237). 3-digit kode unik (`237`) digunakan poller otomatis untuk mengidentifikasi pembayaran pembeli secara presisi.

---

### 2. Tampilkan Gambar QR Code (`GET /qr.png`)

URL gambar QRIS bersifat publik dan dapat langsung disematkan pada tag HTML tanpa header otorisasi:

```html
<!-- Live Mode -->
<img src="https://paymentg.syukurapi.online/api/orders/{order_id}/qr.png" alt="Scan QRIS" width="280" />

<!-- Sandbox Mode -->
<img src="https://paymentg.syukurapi.online/api/sandbox/orders/{order_id}/qr.png" alt="Scan QRIS" width="280" />
```

---

### 3. Cek Status Pesanan (`GET {PAYMENTG_API_URL}/orders/:id`)

Dipanggil untuk memeriksa status terkini (misal via interval polling client-side).

- **Method:** `GET`
- **Headers:** `X-API-Key: {PAYMENTG_API_KEY}`
- **Response (HTTP 200 OK):**
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
  *Nilai status:* `PENDING`, `PAID`, `EXPIRED`, `CANCELLED`.

---

### 4. Tombol "Saya Sudah Bayar" (`POST {PAYMENTG_API_URL}/orders/:id/check`)

Jika pembeli menekan tombol konfirmasi bayar di web toko, panggil endpoint ini untuk langsung memicu verifikasi mutasi ShopeePay tanpa menunggu interval polling rutin.

- **Method:** `POST`
- **Headers:** `X-API-Key: {PAYMENTG_API_KEY}`

---

## 🔔 Webhook Notifikasi (Wajib Ada di Web Toko)

Saat transaksi dinyatakan lunas, PaymentG mengirimkan HTTP POST otomatis ke `webhook_url` yang didaftarkan.

### Format Request dari PaymentG
- **Headers:**
  ```http
  Content-Type: application/json
  X-Webhook-Event: payment.success
  ```
- **Body JSON:**
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

### Kewajiban Endpoint Toko
1. Periksa apakah `event === "payment.success"`.
2. Temukan pesanan di database toko berdasarkan `reference_id`.
3. Jika status pesanan belum `PAID`, ubah menjadi `PAID` dan aktifkan pesanan/layanan pembeli.
4. Kembalikan status HTTP `200 OK` dengan respons JSON `{ "status": "ok" }`.

---

## 💻 Template Kode Implementasi Siap Pakai

### 1. PHP / Laravel (`PaymentWebhookController.php`)

```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use App\Models\Order;

class PaymentWebhookController extends Controller
{
    public function handle(Request $request)
    {
        $payload = $request->json()->all();

        if (($payload['event'] ?? '') !== 'payment.success') {
            return response()->json(['status' => 'ignored'], 200);
        }

        $invoiceId  = $payload['reference_id'];
        $shopeeTxId = $payload['shopee_tx_id'];

        $order = Order::where('invoice_number', $invoiceId)->first();
        if ($order && $order->status !== 'PAID') {
            $order->update([
                'status'        => 'PAID',
                'paid_at'       => now(),
                'shopee_tx_id'  => $shopeeTxId,
            ]);

            // TODO: Kirim notifikasi / proses pengiriman barang
        }

        return response()->json(['status' => 'ok'], 200);
    }
}
```

---

### 2. Node.js / Express (`routes/webhook.js`)

```javascript
const express = require('express');
const router = express.Router();
const db = require('../db'); // Database toko Anda

router.post('/api/webhook/paymentg', express.json(), async (req, res) => {
    const payload = req.body;

    if (payload.event === 'payment.success') {
        const invoiceId = payload.reference_id;
        const shopeeTxId = payload.shopee_tx_id;

        // Update database pesanan web toko
        await db.orders.update({
            where: { invoiceId: invoiceId },
            data: { status: 'PAID', paidAt: new Date(), txId: shopeeTxId }
        });

        console.log(`[PAYMENT SUCCESS] Invoice ${invoiceId} lunas!`);
    }

    return res.status(200).json({ status: 'ok' });
});

module.exports = router;
```

---

### 3. Python / FastAPI (`main.py`)

```python
from fastapi import FastAPI, Request
from pydantic import BaseModel
from typing import Optional

app = FastAPI()

class WebhookPayload(BaseModel):
    event: str
    order_id: str
    reference_id: str
    original_amount: int
    unique_code: int
    total_amount: int
    paid_at: str
    shopee_tx_id: str

@app.post("/api/webhook/paymentg")
async def handle_payment_webhook(payload: WebhookPayload):
    if payload.event == "payment.success":
        # TODO: Update pesanan di database toko Anda
        print(f"Invoice {payload.reference_id} lunas via Shopee Tx {payload.shopee_tx_id}")
    
    return {"status": "ok"}
```

---

## 🧪 Alur Pengujian Cepat (Sandbox Simulator)

1. **Buka Simulator:** Masuk ke `https://paymentg.syukurapi.online/sandbox/` (PIN: `2207`).
2. **Daftarkan Web Toko:** Buka tab **"🔑 Kelola Web Toko"**, masukkan nama toko dan URL webhook Anda. Salin `API Key` yang muncul.
3. **Simulasi Checkout:** Jalankan checkout di web toko Anda menggunakan API Key sandbox.
4. **Klik Bayar:** Kembali ke `/sandbox/` tab **"📜 Mutasi Transaksi"**, tagihan akan berstatus `PENDING`. Klik tombol **`[💳 Bayar Sekarang]`**.
5. **Cek Webhook Inspector:** Kotak log akan menampilkan status pengiriman webhook ke server toko Anda (misal `HTTP 200 OK`). Halaman toko Anda otomatis terupdate lunas!

---

## ✅ Integration Checklist for AI Agent

- [ ] Variabel `PAYMENTG_API_URL` dan `PAYMENTG_API_KEY` tersimpan aman di `.env`.
- [ ] Endpoint pembuatan order toko memanggil `{PAYMENTG_API_URL}/orders`.
- [ ] Tampilan antarmuka pembayaran menonjolkan **`total_amount`** (bukan harga awal).
- [ ] Webhook receiver memvalidasi event `payment.success` dan membalas `HTTP 200 OK`.
- [ ] Tersedia polling status / tombol *"Saya Sudah Bayar"* untuk kenyamanan pembeli.
- [ ] Webhook URL telah didaftarkan di dashboard PaymentG.
