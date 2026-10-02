/**
 * PaymentG — Dashboard Merchant QRIS
 * Pure Vanilla JS, zero build dependencies.
 */

// 1. Configuration & Host Auto-Detection
const DEFAULT_BACKEND = window.location.protocol.startsWith("http") ? window.location.origin : "http://localhost:3200";

const CONFIG = {
  API_URL: localStorage.getItem("paymentg_api_url") || DEFAULT_BACKEND,
  ADMIN_KEY: localStorage.getItem("paymentg_admin_key") || "adm_secret_paymentg_2026",
  API_KEY: localStorage.getItem("paymentg_api_key") || ""
};

// Global State
let allOrders = [];
let allApps = [];
let autoRefreshTimer = null;
let currentCreatedOrderId = null;
let currentCreatedOrderData = null;
let qrCheckTimer = null;

// Initialize on Load
document.addEventListener("DOMContentLoaded", () => {
  checkLockScreen();
  setupTabs();
  setupSettingsModal();
});

// ==========================================
// 0. PIN Access Gate (Session Auth)
// ==========================================
function checkLockScreen() {
  const isUnlocked = sessionStorage.getItem("paymentg_unlocked") === "true";
  const lockOverlay = document.getElementById("pin-lockscreen");

  if (!isUnlocked) {
    if (lockOverlay) lockOverlay.style.display = "flex";
    const field = document.getElementById("pin-field");
    if (field) setTimeout(() => field.focus(), 150);
  } else {
    if (lockOverlay) lockOverlay.style.display = "none";
    initDashboard();
  }
}

async function submitPIN(e) {
  e.preventDefault();
  const pinInput = document.getElementById("pin-field");
  const errElem = document.getElementById("pin-error-msg");
  const btn = document.getElementById("btn-unlock-pin");
  const pin = pinInput.value.trim();

  if (!pin) return;

  btn.disabled = true;
  btn.innerText = "Memverifikasi...";
  errElem.style.display = "none";

  try {
    const res = await fetch(`${CONFIG.API_URL}/api/auth/pin`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pin })
    });
    const json = await res.json();

    if (res.ok && json.success) {
      sessionStorage.setItem("paymentg_unlocked", "true");
      if (json.data && json.data.admin_key) {
        CONFIG.ADMIN_KEY = json.data.admin_key;
        localStorage.setItem("paymentg_admin_key", json.data.admin_key);
      }
      document.getElementById("pin-lockscreen").style.display = "none";
      initDashboard();
    } else {
      errElem.innerText = json.error || "PIN akses salah!";
      errElem.style.display = "block";
      pinInput.value = "";
      pinInput.focus();
    }
  } catch (err) {
    // Fallback verification if backend endpoint fails
    if (pin === "2207") {
      sessionStorage.setItem("paymentg_unlocked", "true");
      document.getElementById("pin-lockscreen").style.display = "none";
      initDashboard();
    } else {
      errElem.innerText = "Gagal menghubungi server API (" + err.message + ")";
      errElem.style.display = "block";
    }
  } finally {
    btn.disabled = false;
    btn.innerText = "Buka Akses Dashboard";
  }
}

function initDashboard() {
  loadAllData();
  loadApps();

  const autoCheckbox = document.getElementById("auto-refresh-toggle");
  if (autoCheckbox) {
    autoCheckbox.addEventListener("change", (e) => {
      if (e.target.checked) startAutoRefresh();
      else stopAutoRefresh();
    });
    startAutoRefresh();
  }
}

// ==========================================
// 1. Navigation Tabs
// ==========================================
function setupTabs() {
  document.querySelectorAll(".tab-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".tab-btn").forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach(c => c.classList.remove("active"));
      btn.classList.add("active");
      const target = document.getElementById(btn.dataset.tab);
      if (target) target.classList.add("active");

      if (btn.dataset.tab === "tab-orders") loadOrders();
      if (btn.dataset.tab === "tab-apps") loadApps();
      if (btn.dataset.tab === "tab-dashboard") loadDashboardStats();
      if (btn.dataset.tab === "tab-generator") refreshAppSelectors();
    });
  });
}

// ==========================================
// 2. HTTP API Client Helper
// ==========================================
async function apiRequest(endpoint, method = "GET", body = null, useAdminKey = true) {
  const url = `${CONFIG.API_URL}${endpoint}`;
  const headers = { "Content-Type": "application/json" };
  
  if (useAdminKey && CONFIG.ADMIN_KEY) {
    headers["X-Admin-Key"] = CONFIG.ADMIN_KEY;
  } else if (!useAdminKey && CONFIG.API_KEY) {
    headers["X-API-Key"] = CONFIG.API_KEY;
  }

  const options = { method, headers };
  if (body) options.body = JSON.stringify(body);

  try {
    const res = await fetch(url, options);
    const data = await res.json();
    return { ok: res.ok, status: res.status, data };
  } catch (err) {
    console.error(`API Error (${endpoint}):`, err);
    return { ok: false, error: err.message };
  }
}

// ==========================================
// 3. Data Loaders & Health
// ==========================================
async function loadAllData() {
  await checkHealth();
  await loadDashboardStats();
  await loadOrders();
}

async function checkHealth() {
  const navBadge = document.getElementById("server-status-badge");
  const banner = document.getElementById("session-banner");
  const bannerIcon = document.getElementById("session-banner-icon");
  const bannerTitle = document.getElementById("session-banner-title");
  const bannerDesc = document.getElementById("session-banner-desc");

  const res = await apiRequest("/api/health", "GET", null, false);

  if (res.ok && res.data.success) {
    const health = res.data.data;
    const isOnline = health.token_valid;

    // Navbar Pill
    if (navBadge) {
      navBadge.innerHTML = `
        <span class="status-dot ${isOnline ? 'online' : 'degraded'}"></span>
        <span class="status-text">${isOnline ? 'Gateway Aktif' : 'Token Expired'}</span>
      `;
    }

    // Health Session Banner
    if (banner) {
      if (isOnline) {
        banner.className = "health-banner connected";
        if (bannerIcon) bannerIcon.innerText = "🟢";
        if (bannerTitle) bannerTitle.innerText = "Sesi ShopeePay Terhubung (Login Aktif)";
        const age = health.token_age_hours ? health.token_age_hours.toFixed(1) + " jam lalu" : "Baru saja";
        const pollTime = health.last_poll_at ? new Date(health.last_poll_at).toLocaleTimeString("id-ID") : "Aktif";
        if (bannerDesc) bannerDesc.innerText = `Token update: ${age} • Sinkronisasi terakhir: ${pollTime} • Order Pending: ${health.pending_orders}`;
      } else {
        banner.className = "health-banner disconnected";
        if (bannerIcon) bannerIcon.innerText = "🔴";
        if (bannerTitle) bannerTitle.innerText = "Sesi ShopeePay Terputus / Perlu Login Ulang!";
        if (bannerDesc) bannerDesc.innerText = health.token_error || "Sesi login ShopeePay telah kadaluwarsa. Klik 'Auto-Login Playwright' atau perbarui token.";
      }
    }
  } else {
    if (navBadge) {
      navBadge.innerHTML = `
        <span class="status-dot offline"></span>
        <span class="status-text">Server Offline</span>
      `;
    }
    if (banner) {
      banner.className = "health-banner warning";
      if (bannerIcon) bannerIcon.innerText = "⚠️";
      if (bannerTitle) bannerTitle.innerText = "Tidak Dapat Menghubungi Server Backend";
      if (bannerDesc) bannerDesc.innerText = "Pastikan executable backend PaymentG aktif di " + CONFIG.API_URL;
    }
  }
}

async function loadDashboardStats() {
  const res = await apiRequest("/api/stats", "GET", null, true);
  if (res.ok && res.data.success) {
    const s = res.data.data;
    document.getElementById("stat-revenue").innerText = "Rp " + Number(s.total_revenue || 0).toLocaleString("id-ID");
    document.getElementById("stat-paid-count").innerText = s.paid_orders || 0;
    document.getElementById("stat-pending-count").innerText = s.pending_orders || 0;
    document.getElementById("stat-total-orders").innerText = s.total_orders || 0;
  }
}

// ==========================================
// 4. Mutasi Transaksi (Orders)
// ==========================================
async function loadOrders() {
  const tbody = document.getElementById("orders-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/orders?limit=100", "GET", null, true);
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--danger); padding: 24px;">Gagal memuat data mutasi. Periksa koneksi backend.</td></tr>`;
    return;
  }

  allOrders = res.data.data || [];
  renderOrdersTable();
}

function renderOrdersTable() {
  const tbody = document.getElementById("orders-tbody");
  if (!tbody) return;

  const filterStatus = document.getElementById("filter-status") ? document.getElementById("filter-status").value : "ALL";
  const search = document.getElementById("search-order") ? document.getElementById("search-order").value.toLowerCase().trim() : "";

  const filtered = allOrders.filter(o => {
    const matchesStatus = filterStatus === "ALL" || o.status === filterStatus;
    const matchesSearch = !search || 
      (o.id && o.id.toLowerCase().includes(search)) ||
      (o.reference_id && o.reference_id.toLowerCase().includes(search)) ||
      (o.shopee_tx_id && o.shopee_tx_id.toLowerCase().includes(search));
    return matchesStatus && matchesSearch;
  });

  if (filtered.length === 0) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-muted); padding: 32px;">Tidak ada riwayat mutasi yang cocok.</td></tr>`;
    return;
  }

  tbody.innerHTML = filtered.map(o => {
    let badgeClass = "badge-pending";
    if (o.status === "PAID") badgeClass = "badge-paid";
    else if (o.status === "EXPIRED") badgeClass = "badge-expired";
    else if (o.status === "CANCELLED") badgeClass = "badge-cancelled";

    const dateFormatted = o.created_at ? new Date(o.created_at).toLocaleString("id-ID", { dateStyle: "short", timeStyle: "short" }) : "-";

    return `
      <tr>
        <td>
          <div style="font-weight: 700; color: var(--text-main); font-family: monospace;">${o.id}</div>
          <div style="font-size: 11px; color: var(--text-muted); font-family: monospace;">${o.reference_id || '-'}</div>
        </td>
        <td class="font-mono">Rp ${Number(o.original_amount).toLocaleString("id-ID")}</td>
        <td><span style="color: var(--primary); font-weight: 700; font-family: monospace;">+${o.unique_code}</span></td>
        <td><b class="font-mono" style="font-size: 14px; color: var(--text-main);">Rp ${Number(o.total_amount).toLocaleString("id-ID")}</b></td>
        <td><span class="badge ${badgeClass}">${o.status}</span></td>
        <td><code class="font-mono" style="font-size: 11px; color: var(--text-muted);">${o.shopee_tx_id || '-'}</code></td>
        <td style="font-size: 11.5px; color: var(--text-muted);">${dateFormatted}</td>
        <td style="text-align: right;">
          <div style="display: inline-flex; gap: 4px; justify-content: flex-end;">
            ${o.status === 'PENDING' ? `
              <button class="btn btn-secondary btn-sm" onclick="showQRModal('${o.id}', ${o.total_amount}, '${o.status}')">QR</button>
              <button class="btn btn-primary btn-sm" onclick="forceCheckOrder('${o.id}')">Cek</button>
              <button class="btn btn-secondary btn-sm" style="color: var(--danger);" title="Batalkan" onclick="cancelOrder('${o.id}')">✕</button>
            ` : `
              <button class="btn btn-secondary btn-sm" onclick="showQRModal('${o.id}', ${o.total_amount}, '${o.status}')">Detail</button>
            `}
          </div>
        </td>
      </tr>
    `;
  }).join("");
}

// ==========================================
// 5. Buat QRIS Tagihan (Generator)
// ==========================================
function refreshAppSelectors() {
  const genSelect = document.getElementById("gen-app-select");
  const settingSelect = document.getElementById("setting-app-picker");
  
  if (!genSelect) return;

  if (allApps.length === 0) {
    genSelect.innerHTML = `<option value="">⚠️ Belum ada web toko. Buat dulu di tab 'Kelola Toko'!</option>`;
    if (settingSelect) settingSelect.innerHTML = `<option value="">⚠️ Belum ada web toko.</option>`;
    return;
  }

  let optionsHtml = allApps.map(a => `<option value="${a.api_key}">${a.name} (${a.api_key.substring(0, 10)}...)</option>`).join("");
  genSelect.innerHTML = optionsHtml;
  if (settingSelect) settingSelect.innerHTML = `<option value="">Pilih web toko...</option>` + optionsHtml;

  // Auto pick first store if CONFIG.API_KEY is not set
  if (!CONFIG.API_KEY && allApps.length > 0) {
    CONFIG.API_KEY = allApps[0].api_key;
    localStorage.setItem("paymentg_api_key", CONFIG.API_KEY);
    genSelect.value = CONFIG.API_KEY;
  } else {
    genSelect.value = CONFIG.API_KEY;
  }
}

function handleAppSelectChange() {
  const select = document.getElementById("gen-app-select");
  if (select && select.value) {
    CONFIG.API_KEY = select.value;
    localStorage.setItem("paymentg_api_key", select.value);
    showToast("Menggunakan API Key: " + select.options[select.selectedIndex].text);
  }
}

async function submitCreateOrder(event) {
  event.preventDefault();

  if (!CONFIG.API_KEY) {
    alert("Silakan daftarkan toko terlebih dahulu di tab 'Kelola Toko & API' untuk mendapatkan API Key.");
    return;
  }

  const btn = document.getElementById("btn-generate-order");
  const amount = parseInt(document.getElementById("gen-amount").value);
  const refId = document.getElementById("gen-ref-id").value.trim() || ("INV-" + Date.now());
  const expiry = parseInt(document.getElementById("gen-expiry").value) || 15;
  const metadata = document.getElementById("gen-metadata").value.trim();

  btn.disabled = true;
  btn.innerHTML = `<span>⏳</span> Membuat QRIS Dinamis...`;

  const res = await apiRequest("/api/orders", "POST", {
    amount: amount,
    reference_id: refId,
    expiry_minutes: expiry,
    metadata: metadata
  }, false);

  btn.disabled = false;
  btn.innerHTML = `<span>⚡</span> Generate QRIS Tagihan`;

  if (res.ok && res.data.success) {
    const order = res.data.data;
    currentCreatedOrderId = order.order_id;
    currentCreatedOrderData = order;
    displayGeneratedQR(order);
    loadAllData();
  } else {
    alert("Gagal membuat QRIS: " + (res.data ? res.data.error : res.error));
  }
}

function displayGeneratedQR(order) {
  const box = document.getElementById("qr-result-box");
  box.style.display = "block";
  box.scrollIntoView({ behavior: "smooth", block: "nearest" });

  document.getElementById("qr-res-total").innerText = "Rp " + Number(order.total_amount).toLocaleString("id-ID");
  document.getElementById("qr-res-total-calc").innerText = "Rp " + Number(order.total_amount).toLocaleString("id-ID");
  document.getElementById("qr-res-base").innerText = "Rp " + Number(order.original_amount).toLocaleString("id-ID");
  document.getElementById("qr-res-unique").innerText = "+" + order.unique_code;
  document.getElementById("qr-res-id").innerText = order.order_id;
  
  // Public QR URL
  const qrImgUrl = `${CONFIG.API_URL}${order.qr_url}`;
  document.getElementById("qr-res-img").src = qrImgUrl;

  const statusBadge = document.getElementById("qr-res-status");
  statusBadge.className = "badge badge-pending";
  statusBadge.innerText = "⏳ MENUNGGU PEMBAYARAN...";

  // Realtime Polling
  if (qrCheckTimer) clearInterval(qrCheckTimer);
  qrCheckTimer = setInterval(async () => {
    if (!currentCreatedOrderId) return;
    const res = await apiRequest(`/api/orders/${currentCreatedOrderId}`, "GET", null, false);
    if (res.ok && res.data.success && res.data.data.status === "PAID") {
      clearInterval(qrCheckTimer);
      statusBadge.className = "badge badge-paid";
      statusBadge.innerText = "🎉 PEMBAYARAN LUNAS!";
      loadAllData();
      showToast("🎉 Pembayaran Rp " + Number(order.total_amount).toLocaleString("id-ID") + " LUNAS!");
    }
  }, 3000);
}

async function forceCheckCurrentOrder() {
  if (!currentCreatedOrderId) return;
  const res = await apiRequest(`/api/orders/${currentCreatedOrderId}/check`, "POST", {}, false);
  if (res.ok && res.data.success) {
    const st = res.data.data ? res.data.data.status : "PAID";
    showToast("Status Pesanan: " + st);
    if (st === "PAID") {
      const badge = document.getElementById("qr-res-status");
      if (badge) {
        badge.className = "badge badge-paid";
        badge.innerText = "🎉 PEMBAYARAN LUNAS!";
      }
    }
    loadAllData();
  } else {
    alert("Hasil cek: belum ada mutasi cocok.");
  }
}

function copyPaymentInstruction() {
  if (!currentCreatedOrderData) return;
  const o = currentCreatedOrderData;
  const text = `Halo Kak! Silakan selesaikan pembayaran:\n\n` +
    `🛒 No. Pesanan: ${o.order_id}\n` +
    `💵 Total Transfer: Rp ${Number(o.total_amount).toLocaleString("id-ID")}\n` +
    `⚠️ Catatan: Wajib transfer TEPAT hingga 3 digit terakhir (+${o.unique_code}) agar terverifikasi otomatis.\n\n` +
    `Scan QRIS melalui aplikasi ShopeePay, BCA, GoPay, OVO, atau Mobile Banking apa saja. Terima kasih!`;
  
  copyToClipboard(text);
}

// ==========================================
// 6. Kelola Toko & API Keys (Tab Apps)
// ==========================================
async function loadApps() {
  const tbody = document.getElementById("apps-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/apps", "GET", null, true);
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--danger); padding: 24px;">Gagal memuat daftar toko.</td></tr>`;
    return;
  }

  allApps = res.data.data || [];
  refreshAppSelectors();

  if (allApps.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--text-muted); padding: 32px;">Belum ada web toko yang didaftarkan. Gunakan form di sebelah kiri untuk membuat.</td></tr>`;
    return;
  }

  tbody.innerHTML = allApps.map(a => {
    const rawSecret = a.webhook_secret || "";
    const maskedSecret = rawSecret ? "••••••••••••••••" : "-";

    return `
      <tr>
        <td>
          <div style="font-weight: 800; color: var(--text-main);">${a.name}</div>
          <div style="font-size: 11px; color: var(--text-muted); font-family: monospace;">${a.id}</div>
        </td>
        <td>
          <div class="table-key-pill">
            <code class="font-mono">${a.api_key}</code>
            <button class="btn btn-secondary btn-xs" title="Salin API Key" onclick="copyToClipboard('${a.api_key}', this)">📋</button>
          </div>
        </td>
        <td>
          <div class="table-secret-pill">
            <span id="secret-text-${a.id}" class="font-mono">${maskedSecret}</span>
            <button class="btn btn-secondary btn-xs" title="Lihat/Sembunyikan Secret" onclick="toggleSecretVisibility('${a.id}', '${rawSecret}', this)">👁️</button>
            <button class="btn btn-secondary btn-xs" title="Salin Webhook Secret" onclick="copyToClipboard('${rawSecret}', this)">📋</button>
          </div>
        </td>
        <td>
          <span style="font-size: 11.5px; font-family: monospace; color: var(--text-muted);" title="${a.webhook_url}">
            ${a.webhook_url.length > 32 ? a.webhook_url.substring(0, 32) + '...' : a.webhook_url}
          </span>
        </td>
        <td>
          <span class="badge ${a.is_active ? 'badge-paid' : 'badge-expired'}">${a.is_active ? 'Aktif' : 'Non-aktif'}</span>
        </td>
        <td style="text-align: right;">
          <button class="btn btn-danger btn-xs" onclick="deleteApp('${a.id}', '${a.name}')">Hapus</button>
        </td>
      </tr>
    `;
  }).join("");
}

function toggleSecretVisibility(appId, secret, btn) {
  const elem = document.getElementById(`secret-text-${appId}`);
  if (!elem) return;

  if (elem.innerText.includes("•••")) {
    elem.innerText = secret;
    btn.innerText = "🔒";
  } else {
    elem.innerText = "••••••••••••••••";
    btn.innerText = "👁️";
  }
}

async function submitCreateApp(event) {
  event.preventDefault();
  const name = document.getElementById("app-name").value.trim();
  const webhook = document.getElementById("app-webhook").value.trim();
  if (!name || !webhook) return;

  const btn = document.getElementById("btn-create-app");
  btn.disabled = true;
  btn.innerText = "Mendaftarkan Toko...";

  const res = await apiRequest("/api/apps", "POST", { name, webhook_url: webhook }, true);
  btn.disabled = false;
  btn.innerHTML = `<span>➕</span> Daftarkan Web Toko & Dapatkan Kunci`;

  if (res.ok && res.data.success) {
    const created = res.data.data;
    
    // Auto set as active API key
    CONFIG.API_KEY = created.api_key;
    localStorage.setItem("paymentg_api_key", created.api_key);

    // Populate Modal
    document.getElementById("created-modal-name").innerText = created.name;
    document.getElementById("created-modal-apikey").value = created.api_key;
    document.getElementById("created-modal-secret").value = created.webhook_secret;

    const envSnippet = 
`# Konfigurasi PaymentG (${created.name})
PAYMENTG_API_URL=${CONFIG.API_URL}
PAYMENTG_API_KEY=${created.api_key}
PAYMENTG_WEBHOOK_SECRET=${created.webhook_secret}`;

    document.getElementById("created-modal-env").innerText = envSnippet;

    // Open Modal
    document.getElementById("created-app-modal").classList.add("active");

    // Reset Form
    document.getElementById("app-name").value = "";
    document.getElementById("app-webhook").value = "";

    loadApps();
  } else {
    alert("Gagal membuat web toko: " + (res.data ? res.data.error : res.error));
  }
}

function copyEnvSnippet(btn) {
  const code = document.getElementById("created-modal-env").innerText;
  copyToClipboard(code, btn);
}

async function deleteApp(id, name) {
  if (!confirm(`Yakin ingin menghapus toko "${name}"?\nSemua tagihan terkait toko ini akan ikut dihapus.`)) return;
  const res = await apiRequest(`/api/apps/${id}`, "DELETE", null, true);
  if (res.ok) {
    showToast("Toko berhasil dihapus!");
    loadApps();
  } else {
    alert("Gagal menghapus toko.");
  }
}

// ==========================================
// 7. Clipboard & Toast Helpers
// ==========================================
function copyToClipboard(text, btnElement = null) {
  if (!text) return;

  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(() => {
      onCopySuccess(btnElement);
    }).catch(() => {
      fallbackCopyText(text, btnElement);
    });
  } else {
    fallbackCopyText(text, btnElement);
  }
}

function fallbackCopyText(text, btnElement) {
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  try {
    document.execCommand("copy");
    onCopySuccess(btnElement);
  } catch (err) {
    alert("Gagal menyalin. Silakan salin manual.");
  }
  document.body.removeChild(textarea);
}

function onCopySuccess(btnElement) {
  showToast("📋 Berhasil disalin ke clipboard!");
  if (btnElement) {
    const originalText = btnElement.innerText;
    btnElement.innerText = "✓";
    setTimeout(() => {
      btnElement.innerText = originalText;
    }, 1500);
  }
}

let toastTimer = null;
function showToast(message) {
  const toast = document.getElementById("toast");
  const msgElem = document.getElementById("toast-message");
  if (!toast) return;

  msgElem.innerText = message;
  toast.classList.add("active");

  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toast.classList.remove("active");
  }, 2600);
}

// ==========================================
// 8. Order Actions & Modal Details
// ==========================================
async function forceCheckOrder(orderId) {
  const res = await apiRequest(`/api/orders/${orderId}/check`, "POST", {}, false);
  if (res.ok && res.data.success) {
    const st = res.data.data ? res.data.data.status : "Terverifikasi";
    showToast(`Status Order: ${st}`);
    loadAllData();
  } else {
    alert("Gagal cek order: " + (res.data ? res.data.error : res.error));
  }
}

async function cancelOrder(orderId) {
  if (!confirm(`Yakin ingin membatalkan tagihan ${orderId}?`)) return;
  const res = await apiRequest(`/api/orders/${orderId}/cancel`, "POST", {}, false);
  if (res.ok && res.data.success) {
    showToast(`Order ${orderId} dibatalkan.`);
    loadAllData();
  } else {
    alert("Gagal membatalkan order.");
  }
}

function showQRModal(orderId, totalAmount, status = "PENDING") {
  const modal = document.getElementById("qr-modal");
  document.getElementById("modal-qr-img").src = `${CONFIG.API_URL}/api/orders/${orderId}/qr.png`;
  document.getElementById("modal-qr-id").innerText = orderId;
  document.getElementById("modal-qr-amount").innerText = "Rp " + Number(totalAmount).toLocaleString("id-ID");
  
  const statusElem = document.getElementById("modal-qr-status");
  if (statusElem) {
    statusElem.innerText = status;
    statusElem.className = `badge ${status === 'PAID' ? 'badge-paid' : (status === 'PENDING' ? 'badge-pending' : 'badge-expired')}`;
  }

  modal.classList.add("active");
}

// ==========================================
// 9. Token & Playwright Operations
// ==========================================
async function triggerPlaywrightRefresh() {
  const btn = document.getElementById("btn-playwright");
  if (btn) {
    btn.disabled = true;
    btn.innerText = "⏳ Membuka Browser...";
  }

  const res = await apiRequest("/api/token/refresh", "POST", {}, true);
  if (btn) {
    btn.disabled = false;
    btn.innerText = "🤖 Auto-Login Playwright";
  }

  if (res.ok && res.data.success) {
    showToast("Hasil: " + res.data.data);
    loadAllData();
  } else {
    alert("Gagal auto-refresh: " + (res.data ? res.data.error : res.error));
  }
}

async function manualUpdateToken() {
  const token = prompt("Masukkan Token ShopeePay baru (diawali B:...):");
  if (!token) return;

  const res = await apiRequest("/api/config/token", "PUT", { token: token.trim() }, true);
  if (res.ok && res.data.success) {
    showToast("Token ShopeePay berhasil diperbarui!");
    loadAllData();
  } else {
    alert("Gagal memperbarui token: " + (res.data ? res.data.error : res.error));
  }
}

// ==========================================
// 10. Settings Modal
// ==========================================
function setupSettingsModal() {
  const modal = document.getElementById("settings-modal");
  const openBtn = document.getElementById("btn-open-settings");
  const closeBtn = document.getElementById("btn-close-settings");

  if (openBtn) {
    openBtn.addEventListener("click", () => {
      document.getElementById("setting-api-url").value = CONFIG.API_URL;
      document.getElementById("setting-admin-key").value = CONFIG.ADMIN_KEY;
      document.getElementById("setting-api-key").value = CONFIG.API_KEY;
      refreshAppSelectors();
      modal.classList.add("active");
    });
  }

  if (closeBtn) {
    closeBtn.addEventListener("click", () => modal.classList.remove("active"));
  }

  const saveBtn = document.getElementById("btn-save-settings");
  if (saveBtn) {
    saveBtn.addEventListener("click", () => {
      CONFIG.API_URL = document.getElementById("setting-api-url").value.trim().replace(/\/$/, "");
      CONFIG.ADMIN_KEY = document.getElementById("setting-admin-key").value.trim();
      CONFIG.API_KEY = document.getElementById("setting-api-key").value.trim();

      localStorage.setItem("paymentg_api_url", CONFIG.API_URL);
      localStorage.setItem("paymentg_admin_key", CONFIG.ADMIN_KEY);
      localStorage.setItem("paymentg_api_key", CONFIG.API_KEY);

      modal.classList.remove("active");
      showToast("Pengaturan API berhasil disimpan!");
      loadAllData();
    });
  }
}

function syncSelectedAppToApiKey(apiKey) {
  if (apiKey) {
    document.getElementById("setting-api-key").value = apiKey;
  }
}

function togglePasswordVisibility(fieldId, btn) {
  const input = document.getElementById(fieldId);
  if (!input) return;
  if (input.type === "password") {
    input.type = "text";
    btn.innerText = "🔒";
  } else {
    input.type = "password";
    btn.innerText = "👁️";
  }
}

function resetSettingsToDefault() {
  if (!confirm("Kembalikan URL Backend ke asal (" + DEFAULT_BACKEND + ")?")) return;
  document.getElementById("setting-api-url").value = DEFAULT_BACKEND;
  document.getElementById("setting-admin-key").value = "adm_secret_paymentg_2026";
  showToast("Pengaturan dikembalikan ke default");
}

function closeModal(id) {
  const modal = document.getElementById(id);
  if (modal) modal.classList.remove("active");
}

// Auto Refresh Timers
function startAutoRefresh() {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer);
  autoRefreshTimer = setInterval(() => {
    loadAllData();
  }, 5000);
}

function stopAutoRefresh() {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer);
  autoRefreshTimer = null;
}
