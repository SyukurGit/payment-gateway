/**
 * TokoTest — Client-side E-Commerce Engine
 * Terintegrasi langsung dengan PaymentG QRIS API
 */

// --- 1. Master Data Produk Dummy ---
const PRODUCTS = [
  {
    id: "steam-50k",
    name: "Steam Wallet Code IDR 50.000",
    price: 50000,
    desc: "Kode voucher resmi Steam Wallet Indonesia untuk membeli game, DLC, dan item komunitas di Steam.",
    thumbClass: "product-thumb-steam",
    icon: "🎮",
    tag: "INSTANT",
    voucherPrefix: "STEAM-IDR"
  },
  {
    id: "mlbb-86dm",
    name: "Mobile Legends 86 Diamonds",
    price: 20000,
    desc: "Top up Diamond MLBB instan legal. Masukkan ID Akun & Zone ID untuk aktivasi otomatis.",
    thumbClass: "product-thumb-mlbb",
    icon: "💎",
    tag: "POPULER",
    voucherPrefix: "MLBB-TOKEN"
  },
  {
    id: "spotify-1m",
    name: "Spotify Premium Individual 1 Bulan",
    price: 45000,
    desc: "Langganan Spotify Premium resmi tanpa jeda iklan, audio ultra high quality, dan bebas download offline.",
    thumbClass: "product-thumb-spotify",
    icon: "🎵",
    tag: "TERLARIS",
    voucherPrefix: "SPOTIFY-PREM"
  }
];

// --- 2. State Aplikasi ---
let selectedProduct = null;
let currentActiveOrder = null;
let pollingTimer = null;
let countdownTimer = null;

// --- 3. Inisialisasi Saat Dokumen Siap ---
document.addEventListener("DOMContentLoaded", () => {
  renderBrandInfo();
  renderProducts();
  checkConfigNotice();
  renderHistoryBanner();
});

// Helper Escape HTML
function escapeHtml(str) {
  if (!str) return "";
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function showToast(msg) {
  const t = document.getElementById("toast");
  if (!t) return;
  t.innerText = msg;
  t.style.display = "block";
  setTimeout(() => { t.style.display = "none"; }, 3500);
}

// Render Nama Toko & Header
function renderBrandInfo() {
  const brandElem = document.getElementById("brand-name");
  if (brandElem) brandElem.innerText = window.STORE_CONFIG.STORE_NAME;
}

// Peringatan jika API Key belum disetel
function checkConfigNotice() {
  const banner = document.getElementById("config-warning-banner");
  if (!banner) return;
  
  const key = window.STORE_CONFIG.API_KEY || "";
  if (key.includes("ganti_dengan_key") || key === "") {
    banner.style.display = "flex";
  } else {
    banner.style.display = "none";
  }
}

// --- 4. Render Katalog Produk ---
function renderProducts() {
  const grid = document.getElementById("product-grid");
  if (!grid) return;

  grid.innerHTML = PRODUCTS.map(p => `
    <div class="product-card">
      <div class="product-thumb ${p.thumbClass}">
        <span class="product-thumb-icon">${p.icon}</span>
        <span class="badge badge-primary product-tag">${p.tag}</span>
      </div>
      <div class="product-body">
        <h3 class="product-title">${escapeHtml(p.name)}</h3>
        <p class="product-desc">${escapeHtml(p.desc)}</p>
        <div class="product-footer">
          <div class="price-row">
            <span class="price-label">Harga</span>
            <span class="price-val">Rp ${p.price.toLocaleString("id-ID")}</span>
          </div>
          <div class="action-buttons">
            <button class="btn btn-secondary btn-sm" onclick="showCartLoginAlert('${p.id}')">
              🛒 Keranjang
            </button>
            <button class="btn btn-primary btn-sm" onclick="openCheckoutModal('${p.id}')">
              ⚡ Beli Sekarang
            </button>
          </div>
        </div>
      </div>
    </div>
  `).join("");
}

// --- 5. Cart Login Alert Modal ---
function showCartLoginAlert(productId) {
  const p = PRODUCTS.find(x => x.id === productId);
  const modal = document.getElementById("cart-alert-modal");
  const descElem = document.getElementById("cart-alert-desc");
  if (descElem && p) {
    descElem.innerText = `Untuk memasukkan "${p.name}" ke keranjang belanja, Anda wajib login ke akun member.`;
  }
  if (modal) modal.style.display = "flex";
}

function closeCartAlertModal() {
  const modal = document.getElementById("cart-alert-modal");
  if (modal) modal.style.display = "none";
}

// --- 6. Checkout Modal & Form ---
function openCheckoutModal(productId) {
  closeCartAlertModal();
  selectedProduct = PRODUCTS.find(x => x.id === productId);
  if (!selectedProduct) return;

  // Cek apakah API Key sudah diisi
  const key = window.STORE_CONFIG.API_KEY || "";
  if (key.includes("ganti_dengan_key") || key === "") {
    openSettingsModal();
    showToast("⚠️ Mohon isi API Key toko Anda terlebih dahulu!");
    return;
  }

  // Isi data modal
  document.getElementById("checkout-product-title").innerText = selectedProduct.name;
  document.getElementById("checkout-product-price").innerText = `Rp ${selectedProduct.price.toLocaleString("id-ID")}`;
  document.getElementById("checkout-product-icon").innerText = selectedProduct.icon;

  const modal = document.getElementById("checkout-modal");
  if (modal) modal.style.display = "flex";
}

function closeCheckoutModal() {
  const modal = document.getElementById("checkout-modal");
  if (modal) modal.style.display = "none";
}

// Submit Checkout Form
async function submitCheckout(event) {
  event.preventDefault();
  if (!selectedProduct) return;

  const btn = document.getElementById("btn-submit-order");
  const buyerName = document.getElementById("buyer-name").value.trim();
  const buyerEmail = document.getElementById("buyer-email").value.trim();
  const buyerPhone = document.getElementById("buyer-phone").value.trim();

  if (!buyerName || !buyerEmail || !buyerPhone) {
    alert("Mohon lengkapi Nama, Email, dan No. WhatsApp Anda!");
    return;
  }

  btn.disabled = true;
  btn.innerText = "Membuat Tagihan QRIS...";

  const invoiceNumber = `INV-${Date.now().toString().slice(-6)}${Math.floor(Math.random() * 900 + 100)}`;

  try {
    const res = await fetch(`${window.STORE_CONFIG.API_URL}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-API-Key": window.STORE_CONFIG.API_KEY
      },
      body: JSON.stringify({
        reference_id: invoiceNumber,
        amount: selectedProduct.price,
        expiry_minutes: 15,
        metadata: JSON.stringify({
          product_id: selectedProduct.id,
          product_name: selectedProduct.name,
          buyer_name: buyerName,
          email: buyerEmail,
          phone: buyerPhone
        })
      })
    });

    const json = await res.json();

    if (!res.ok || !json.success) {
      throw new Error(json.error || "Gagal membuat tagihan QRIS ke server gateway.");
    }

    const orderData = json.data;

    // Susun objek order aktif
    currentActiveOrder = {
      order_id: orderData.order_id,
      reference_id: orderData.reference_id || invoiceNumber,
      product: selectedProduct,
      original_amount: orderData.original_amount,
      unique_code: orderData.unique_code,
      total_amount: orderData.total_amount,
      qr_url: orderData.qr_url,
      expires_at: orderData.expires_at,
      buyer: { name: buyerName, email: buyerEmail, phone: buyerPhone },
      status: "PENDING",
      created_at: new Date().toISOString()
    };

    // Tutup modal checkout, buka modal pembayaran QRIS
    closeCheckoutModal();
    openPaymentModal(currentActiveOrder);

  } catch (err) {
    alert("Error Pembayaran: " + err.message + "\n\nPastikan URL PaymentG aktif dan API Key Anda valid.");
  } finally {
    btn.disabled = false;
    btn.innerText = "Bayar Tagihan QRIS Sekarang";
  }
}

// --- 7. Modal Pembayaran QRIS & Polling Realtime ---
function openPaymentModal(order) {
  const modal = document.getElementById("payment-modal");
  if (!modal) return;

  // Resolve Full QR Image URL
  let fullQrUrl = order.qr_url;
  try {
    fullQrUrl = new URL(order.qr_url, window.STORE_CONFIG.API_URL).href;
  } catch (e) {
    fullQrUrl = `${window.STORE_CONFIG.API_URL}${order.qr_url}`;
  }

  document.getElementById("pay-invoice-id").innerText = order.reference_id;
  document.getElementById("pay-product-name").innerText = order.product.name;
  document.getElementById("pay-total-amount").innerText = `Rp ${order.total_amount.toLocaleString("id-ID")}`;
  document.getElementById("pay-unique-code").innerText = `+${order.unique_code}`;
  document.getElementById("pay-qr-img").src = fullQrUrl;

  modal.style.display = "flex";

  // Mulai Countdown Timer 15 Menit
  startCountdown(order.expires_at);

  // Mulai Auto-Polling setiap 3 Detik
  startAutoPolling(order.order_id);
}

function closePaymentModal() {
  stopPolling();
  stopCountdown();
  const modal = document.getElementById("payment-modal");
  if (modal) modal.style.display = "none";
}

// Countdown Timer
function startCountdown(expiresAtStr) {
  stopCountdown();
  const targetTime = new Date(expiresAtStr).getTime();
  const timerElem = document.getElementById("pay-timer");

  countdownTimer = setInterval(() => {
    const now = Date.now();
    const diff = Math.max(0, Math.floor((targetTime - now) / 1000));

    if (diff <= 0) {
      stopCountdown();
      stopPolling();
      if (timerElem) timerElem.innerText = "⚠️ Tagihan Kedaluwarsa. Silakan checkout ulang.";
      return;
    }

    const mins = Math.floor(diff / 60);
    const secs = diff % 60;
    if (timerElem) {
      timerElem.innerText = `Selesaikan pembayaran dalam ${mins}:${secs < 10 ? '0' : ''}${secs}`;
    }
  }, 1000);
}

function stopCountdown() {
  if (countdownTimer) {
    clearInterval(countdownTimer);
    countdownTimer = null;
  }
}

// Auto Polling Status Pesanan ke PaymentG
function startAutoPolling(orderId) {
  stopPolling();
  pollingTimer = setInterval(async () => {
    await checkOrderStatus(orderId, false);
  }, 3000);
}

function stopPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer);
    pollingTimer = null;
  }
}

// Cek Status (Otomatis & Tombol Manual)
async function checkOrderStatus(orderId, isManualClick = false) {
  const btn = document.getElementById("btn-manual-check");
  if (isManualClick && btn) {
    btn.disabled = true;
    btn.innerText = "Memeriksa mutasi...";
  }

  try {
    // Jika manual click, kita panggil endpoint check untuk memicu cek mutasi instan di backend
    const endpoint = isManualClick 
      ? `${window.STORE_CONFIG.API_URL}/orders/${orderId}/check`
      : `${window.STORE_CONFIG.API_URL}/orders/${orderId}`;
    
    const method = isManualClick ? "POST" : "GET";

    const res = await fetch(endpoint, {
      method: method,
      headers: {
        "X-API-Key": window.STORE_CONFIG.API_KEY
      }
    });

    const json = await res.json();

    if (res.ok && json.success) {
      const status = json.data.status;

      if (status === "PAID") {
        stopPolling();
        stopCountdown();
        handlePaymentSuccess(json.data);
      } else if (status === "EXPIRED" || status === "CANCELLED") {
        stopPolling();
        stopCountdown();
        alert(`Status pesanan: ${status}. Silakan buat pesanan baru.`);
        closePaymentModal();
      } else if (isManualClick) {
        showToast("⏳ Pembayaran belum terdeteksi. Silakan transfer tepat sesuai nominal!");
      }
    }
  } catch (err) {
    console.warn("Polling status error:", err);
  } finally {
    if (isManualClick && btn) {
      btn.disabled = false;
      btn.innerText = "🔄 Cek Status Pembayaran Sekarang";
    }
  }
}

// Trigger Manual Button
function manualCheckCurrentPayment() {
  if (!currentActiveOrder) return;
  checkOrderStatus(currentActiveOrder.order_id, true);
}

// --- 8. Penanganan Pembayaran Lunas & Pengiriman Voucher ---
function handlePaymentSuccess(gatewayOrder) {
  if (!currentActiveOrder) return;

  // Buat Token Voucher Dummy Otomatis
  const randomChars = () => Math.random().toString(36).substring(2, 6).toUpperCase();
  const voucherCode = `${currentActiveOrder.product.voucherPrefix}-${randomChars()}-${randomChars()}-${randomChars()}`;

  // Update Data Order Lengkap
  const completedOrder = {
    ...currentActiveOrder,
    status: "PAID",
    paid_at: gatewayOrder.paid_at || new Date().toISOString(),
    shopee_tx_id: gatewayOrder.shopee_tx_id || "TX-" + Date.now(),
    voucher_code: voucherCode
  };

  // Simpan ke LocalStorage Browser
  saveOrderToStorage(completedOrder);

  // Tutup Modal Bayar
  closePaymentModal();

  // Tampilkan Toast
  showToast("🎉 Pembayaran Berhasil Diverifikasi!");

  // Redirect ke Halaman Resi / Invoice Mandiri
  setTimeout(() => {
    window.location.href = `order.html?inv=${completedOrder.reference_id}`;
  }, 1000);
}

// --- 9. LocalStorage Persistence (Riwayat Belanja Tanpa Login) ---
function getOrdersFromStorage() {
  try {
    const raw = localStorage.getItem("tokotest_orders");
    return raw ? JSON.parse(raw) : [];
  } catch (e) {
    return [];
  }
}

function saveOrderToStorage(order) {
  const orders = getOrdersFromStorage();
  const existingIdx = orders.findIndex(x => x.reference_id === order.reference_id);
  if (existingIdx >= 0) {
    orders[existingIdx] = order;
  } else {
    orders.unshift(order);
  }
  localStorage.setItem("tokotest_orders", JSON.stringify(orders));
  renderHistoryBanner();
}

// Floating / Top Banner Riwayat Belanja
function renderHistoryBanner() {
  const banner = document.getElementById("history-banner");
  const countElem = document.getElementById("history-count");
  if (!banner) return;

  const orders = getOrdersFromStorage();
  if (orders.length > 0) {
    banner.style.display = "flex";
    if (countElem) countElem.innerText = orders.length;
  } else {
    banner.style.display = "none";
  }
}

// Modal Riwayat Belanja
function openHistoryModal() {
  const modal = document.getElementById("history-modal");
  const listElem = document.getElementById("history-list");
  if (!modal || !listElem) return;

  const orders = getOrdersFromStorage();

  if (orders.length === 0) {
    listElem.innerHTML = `<div style="text-align: center; color: var(--text-muted); padding: 30px;">Belum ada riwayat pesanan tersimpan di browser ini.</div>`;
  } else {
    listElem.innerHTML = orders.map(o => `
      <div class="history-item">
        <div>
          <div style="font-weight: 700; font-size: 14px;">${escapeHtml(o.product.name)}</div>
          <div style="font-size: 12px; color: var(--text-muted); font-family: monospace;">${escapeHtml(o.reference_id)} • ${new Date(o.created_at).toLocaleDateString("id-ID")}</div>
          <div style="font-size: 13px; font-weight: 700; color: #38bdf8; margin-top: 4px;">Rp ${Number(o.total_amount).toLocaleString("id-ID")}</div>
        </div>
        <div style="text-align: right;">
          <span class="badge ${o.status === 'PAID' ? 'badge-success' : 'badge-warning'}">${o.status}</span>
          <div style="margin-top: 8px;">
            <a href="order.html?inv=${encodeURIComponent(o.reference_id)}" class="btn btn-secondary btn-sm" style="text-decoration: none;">
              Lihat Resi ➜
            </a>
          </div>
        </div>
      </div>
    `).join("");
  }

  modal.style.display = "flex";
}

function closeHistoryModal() {
  const modal = document.getElementById("history-modal");
  if (modal) modal.style.display = "none";
}

// --- 10. Modal Pengaturan API (ENV Editor di Browser) ---
function openSettingsModal() {
  document.getElementById("cfg-api-url").value = window.STORE_CONFIG.API_URL;
  document.getElementById("cfg-api-key").value = window.STORE_CONFIG.API_KEY;
  document.getElementById("cfg-store-name").value = window.STORE_CONFIG.STORE_NAME;

  const modal = document.getElementById("settings-modal");
  if (modal) modal.style.display = "flex";
}

function closeSettingsModal() {
  const modal = document.getElementById("settings-modal");
  if (modal) modal.style.display = "none";
}

function setPresetConfig(mode) {
  if (mode === "sandbox") {
    document.getElementById("cfg-api-url").value = "http://localhost:3200/api/sandbox";
    showToast("Preset Sandbox Lokal dipilih.");
  } else if (mode === "live") {
    document.getElementById("cfg-api-url").value = "http://localhost:3200/api";
    showToast("Preset Live Lokal dipilih.");
  }
}

function saveSettings(event) {
  event.preventDefault();
  const url = document.getElementById("cfg-api-url").value.trim();
  const key = document.getElementById("cfg-api-key").value.trim();
  const name = document.getElementById("cfg-store-name").value.trim();

  if (!url || !key) {
    alert("URL dan API Key tidak boleh kosong!");
    return;
  }

  localStorage.setItem("tokotest_api_url", url);
  localStorage.setItem("tokotest_api_key", key);
  if (name) localStorage.setItem("tokotest_store_name", name);

  window.STORE_CONFIG.API_URL = url;
  window.STORE_CONFIG.API_KEY = key;
  window.STORE_CONFIG.STORE_NAME = name || window.STORE_CONFIG.STORE_NAME;

  closeSettingsModal();
  renderBrandInfo();
  checkConfigNotice();
  showToast("✅ Pengaturan API berhasil disimpan!");
}
