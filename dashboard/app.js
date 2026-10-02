/**
 * PaymentG — Static Frontend Dashboard App
 * Pure Vanilla JS, zero build dependencies.
 */

// 1. Static Default Configuration (Dapat disesuaikan langsung di sini atau lewat UI Settings)
const CONFIG = {
  API_URL: localStorage.getItem("paymentg_api_url") || "http://localhost:3200",
  ADMIN_KEY: localStorage.getItem("paymentg_admin_key") || "adm_secret_paymentg_2026",
  API_KEY: localStorage.getItem("paymentg_api_key") || "ak_iFV5BsdprjxCb4AnpkCkrYzc"
};

// State
let allOrders = [];
let autoRefreshTimer = null;
let currentCreatedOrderId = null;
let qrCheckTimer = null;

// Initialize
document.addEventListener("DOMContentLoaded", () => {
  setupTabs();
  setupSettingsModal();
  loadAllData();
  
  // Auto refresh interval 5 detik
  const autoCheckbox = document.getElementById("auto-refresh-toggle");
  if (autoCheckbox) {
    autoCheckbox.addEventListener("change", (e) => {
      if (e.target.checked) startAutoRefresh();
      else stopAutoRefresh();
    });
    startAutoRefresh();
  }
});

// --- Tabs Navigation ---
function setupTabs() {
  document.querySelectorAll(".tab-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".tab-btn").forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach(c => c.classList.remove("active"));
      btn.classList.add("active");
      const target = document.getElementById(btn.dataset.tab);
      if (target) target.classList.add("active");

      // Auto load tab data
      if (btn.dataset.tab === "tab-orders") loadOrders();
      if (btn.dataset.tab === "tab-apps") loadApps();
      if (btn.dataset.tab === "tab-dashboard") loadDashboardStats();
    });
  });
}

// --- API Helpers ---
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

// --- Data Loaders ---
async function loadAllData() {
  await checkHealth();
  await loadDashboardStats();
  await loadOrders();
}

// 1. Healthcheck
async function checkHealth() {
  const badge = document.getElementById("server-status-badge");
  const tokenBadge = document.getElementById("token-status-display");
  const res = await apiRequest("/api/health", "GET", null, false);

  if (res.ok && res.data.success) {
    const health = res.data.data;
    if (badge) {
      badge.innerHTML = `<span class="status-dot ${health.token_valid ? 'online' : 'degraded'}"></span> ${health.token_valid ? 'Online' : 'Token Expired'}`;
    }
    if (tokenBadge) {
      if (health.token_valid) {
        tokenBadge.innerHTML = `<span class="badge badge-paid">Aktif</span> (${health.token_age_hours ? health.token_age_hours.toFixed(1) + ' jam' : 'OK'})`;
      } else {
        tokenBadge.innerHTML = `<span class="badge badge-cancelled">Kadaluwarsa</span>`;
      }
    }
  } else {
    if (badge) badge.innerHTML = `<span class="status-dot offline"></span> Offline`;
    if (tokenBadge) tokenBadge.innerHTML = `<span class="badge badge-cancelled">Server Disconnected</span>`;
  }
}

// 2. Stats
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

// 3. Orders List
async function loadOrders() {
  const tbody = document.getElementById("orders-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/orders?limit=100", "GET", null, true);
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--danger);">Gagal memuat data. Periksa X-Admin-Key & koneksi server.</td></tr>`;
    return;
  }

  allOrders = res.data.data || [];
  renderOrdersTable();
}

function renderOrdersTable() {
  const tbody = document.getElementById("orders-tbody");
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
    tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted); padding: 24px;">Tidak ada transaksi yang cocok.</td></tr>`;
    return;
  }

  tbody.innerHTML = filtered.map(o => {
    let badgeClass = "badge-pending";
    if (o.status === "PAID") badgeClass = "badge-paid";
    else if (o.status === "EXPIRED") badgeClass = "badge-expired";
    else if (o.status === "CANCELLED") badgeClass = "badge-cancelled";

    const dateFormatted = o.created_at ? new Date(o.created_at).toLocaleString("id-ID") : "-";

    return `
      <tr>
        <td>
          <div style="font-weight: 600;">${o.id}</div>
          <div style="font-size: 11.5px; color: var(--text-muted);">${o.reference_id || '-'}</div>
        </td>
        <td>Rp ${Number(o.original_amount).toLocaleString("id-ID")}</td>
        <td><span style="color: var(--primary); font-weight: 600;">+${o.unique_code}</span></td>
        <td><b style="font-size: 14.5px;">Rp ${Number(o.total_amount).toLocaleString("id-ID")}</b></td>
        <td><span class="badge ${badgeClass}">${o.status}</span></td>
        <td><code style="font-size: 11.5px;">${o.shopee_tx_id || '-'}</code></td>
        <td style="font-size: 12px; color: var(--text-muted);">${dateFormatted}</td>
        <td>
          <div style="display: flex; gap: 6px;">
            ${o.status === 'PENDING' ? `
              <button class="btn btn-secondary btn-sm" onclick="showQRModal('${o.id}', ${o.total_amount})">QR</button>
              <button class="btn btn-primary btn-sm" onclick="forceCheckOrder('${o.id}')">Cek</button>
              <button class="btn btn-secondary btn-sm" style="color: var(--danger);" onclick="cancelOrder('${o.id}')">Batal</button>
            ` : `
              <button class="btn btn-secondary btn-sm" onclick="showQRModal('${o.id}', ${o.total_amount})">Detail</button>
            `}
          </div>
        </td>
      </tr>
    `;
  }).join("");
}

// 4. Create New Order (Generate QRIS)
async function submitCreateOrder(event) {
  event.preventDefault();
  const btn = document.getElementById("btn-generate-order");
  const amount = parseInt(document.getElementById("gen-amount").value);
  const refId = document.getElementById("gen-ref-id").value.trim() || ("INV-" + Date.now());
  const expiry = parseInt(document.getElementById("gen-expiry").value) || 15;
  const metadata = document.getElementById("gen-metadata").value.trim();

  btn.disabled = true;
  btn.innerText = "Membuat QRIS...";

  const res = await apiRequest("/api/orders", "POST", {
    amount: amount,
    reference_id: refId,
    expiry_minutes: expiry,
    metadata: metadata
  }, false); // use client API key

  btn.disabled = false;
  btn.innerText = "Generate QRIS Tagihan";

  if (res.ok && res.data.success) {
    const order = res.data.data;
    currentCreatedOrderId = order.order_id;
    displayGeneratedQR(order);
    loadAllData();
  } else {
    alert("Gagal membuat QRIS: " + (res.data ? res.data.error : res.error));
  }
}

function displayGeneratedQR(order) {
  const box = document.getElementById("qr-result-box");
  box.style.display = "block";
  document.getElementById("qr-res-total").innerText = "Rp " + Number(order.total_amount).toLocaleString("id-ID");
  document.getElementById("qr-res-unique").innerText = order.unique_code;
  document.getElementById("qr-res-id").innerText = order.order_id;
  
  // Public QR URL (bisa langsung dimuat tanpa auth header)
  const qrImgUrl = `${CONFIG.API_URL}${order.qr_url}`;
  document.getElementById("qr-res-img").src = qrImgUrl;

  const statusBadge = document.getElementById("qr-res-status");
  statusBadge.className = "badge badge-pending";
  statusBadge.innerText = "MENUNGGU PEMBAYARAN...";

  // Realtime Polling for this created QR
  if (qrCheckTimer) clearInterval(qrCheckTimer);
  qrCheckTimer = setInterval(async () => {
    if (!currentCreatedOrderId) return;
    const res = await apiRequest(`/api/orders/${currentCreatedOrderId}`, "GET", null, false);
    if (res.ok && res.data.success && res.data.data.status === "PAID") {
      clearInterval(qrCheckTimer);
      statusBadge.className = "badge badge-paid";
      statusBadge.innerText = "🎉 PEMBAYARAN LUNAS!";
      loadAllData();
      alert("Pembayaran sebesar Rp " + Number(order.total_amount).toLocaleString("id-ID") + " LUNAS!");
    }
  }, 3000);
}

// 5. Actions on Orders
async function forceCheckOrder(orderId) {
  const res = await apiRequest(`/api/orders/${orderId}/check`, "POST", {}, false);
  if (res.ok && res.data.success) {
    alert("Hasil Cek: Status " + (res.data.data ? res.data.data.status : "Terverifikasi"));
    loadAllData();
  } else {
    alert("Gagal cek order: " + (res.data ? res.data.error : res.error));
  }
}

async function cancelOrder(orderId) {
  if (!confirm(`Yakin ingin membatalkan order ${orderId}?`)) return;
  const res = await apiRequest(`/api/orders/${orderId}/cancel`, "POST", {}, false);
  if (res.ok && res.data.success) {
    loadAllData();
  } else {
    alert("Gagal membatalkan order.");
  }
}

function showQRModal(orderId, totalAmount) {
  const modal = document.getElementById("qr-modal");
  document.getElementById("modal-qr-img").src = `${CONFIG.API_URL}/api/orders/${orderId}/qr.png`;
  document.getElementById("modal-qr-id").innerText = orderId;
  document.getElementById("modal-qr-amount").innerText = "Rp " + Number(totalAmount).toLocaleString("id-ID");
  modal.classList.add("active");
}

// 6. Apps Management
async function loadApps() {
  const tbody = document.getElementById("apps-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/apps", "GET", null, true);
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--danger);">Gagal memuat daftar App.</td></tr>`;
    return;
  }

  const apps = res.data.data || [];
  if (apps.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 20px;">Belum ada app/web yang didaftarkan.</td></tr>`;
    return;
  }

  tbody.innerHTML = apps.map(a => `
    <tr>
      <td><b>${a.name}</b></td>
      <td><code>${a.api_key}</code></td>
      <td style="font-size: 12px;">${a.webhook_url}</td>
      <td><span class="badge ${a.is_active ? 'badge-paid' : 'badge-expired'}">${a.is_active ? 'Aktif' : 'Non-aktif'}</span></td>
      <td>
        <button class="btn btn-secondary btn-sm" style="color: var(--danger);" onclick="deleteApp('${a.id}')">Hapus</button>
      </td>
    </tr>
  `).join("");
}

async function submitCreateApp(event) {
  event.preventDefault();
  const name = document.getElementById("app-name").value.trim();
  const webhook = document.getElementById("app-webhook").value.trim();
  if (!name || !webhook) return;

  const res = await apiRequest("/api/apps", "POST", { name, webhook_url: webhook }, true);
  if (res.ok && res.data.success) {
    const created = res.data.data;
    alert(`App berhasil dibuat!\n\nAPI Key: ${created.api_key}\nWebhook Secret: ${created.webhook_secret}\n\nHarap simpan API Key ini!`);
    document.getElementById("app-name").value = "";
    document.getElementById("app-webhook").value = "";
    loadApps();
  } else {
    alert("Gagal membuat app: " + (res.data ? res.data.error : res.error));
  }
}

async function deleteApp(id) {
  if (!confirm("Hapus app ini? Semua pesanan terkait akan ikut terhapus.")) return;
  const res = await apiRequest(`/api/apps/${id}`, "DELETE", null, true);
  if (res.ok) {
    loadApps();
  } else {
    alert("Gagal menghapus app.");
  }
}

// 7. Token Operations
async function triggerPlaywrightRefresh() {
  const btn = document.getElementById("btn-playwright");
  if (btn) {
    btn.disabled = true;
    btn.innerText = "Sedang Membuka Browser...";
  }

  const res = await apiRequest("/api/token/refresh", "POST", {}, true);
  if (btn) {
    btn.disabled = false;
    btn.innerText = "🤖 Auto-Refresh via Playwright";
  }

  if (res.ok && res.data.success) {
    alert("Hasil: " + res.data.data);
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
    alert("Token ShopeePay berhasil diperbarui!");
    loadAllData();
  } else {
    alert("Gagal memperbarui token: " + (res.data ? res.data.error : res.error));
  }
}

// --- Auto Refresh Control ---
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

// --- Settings Modal ---
function setupSettingsModal() {
  const modal = document.getElementById("settings-modal");
  const openBtn = document.getElementById("btn-open-settings");
  const closeBtn = document.getElementById("btn-close-settings");

  if (openBtn) {
    openBtn.addEventListener("click", () => {
      document.getElementById("setting-api-url").value = CONFIG.API_URL;
      document.getElementById("setting-admin-key").value = CONFIG.ADMIN_KEY;
      document.getElementById("setting-api-key").value = CONFIG.API_KEY;
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
      loadAllData();
      alert("Pengaturan API berhasil disimpan!");
    });
  }
}

function closeModal(id) {
  const modal = document.getElementById(id);
  if (modal) modal.classList.remove("active");
}
