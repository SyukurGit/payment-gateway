/**
 * PaymentG — Dedicated Sandbox Frontend App
 * Pure Vanilla JS, zero build dependencies.
 */

// 1. Configuration & Host Auto-Detection
const DEFAULT_BACKEND = window.location.protocol.startsWith("http") ? window.location.origin : "http://localhost:3200";

const CONFIG = {
  API_URL: localStorage.getItem("paymentg_api_url") || DEFAULT_BACKEND,
  ADMIN_KEY: localStorage.getItem("paymentg_admin_key") || "adm_secret_paymentg_2026"
};

// Global State
let allSandboxOrders = [];
let allSandboxApps = [];
let autoRefreshTimer = null;

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
  const isUnlocked = sessionStorage.getItem("paymentg_sandbox_unlocked") === "true";
  const lockOverlay = document.getElementById("pin-lockscreen");

  if (!isUnlocked) {
    if (lockOverlay) lockOverlay.style.display = "flex";
    const field = document.getElementById("pin-field");
    if (field) setTimeout(() => field.focus(), 150);
  } else {
    if (lockOverlay) lockOverlay.style.display = "none";
    initSandbox();
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
    const res = await fetch(`${CONFIG.API_URL}/api/sandbox/auth/pin`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pin })
    });
    const json = await res.json();

    if (res.ok && json.success) {
      sessionStorage.setItem("paymentg_sandbox_unlocked", "true");
      if (json.data && json.data.admin_key) {
        CONFIG.ADMIN_KEY = json.data.admin_key;
        localStorage.setItem("paymentg_admin_key", json.data.admin_key);
      }
      document.getElementById("pin-lockscreen").style.display = "none";
      initSandbox();
    } else {
      errElem.innerText = json.error || "PIN akses salah!";
      errElem.style.display = "block";
      pinInput.value = "";
      pinInput.focus();
    }
  } catch (err) {
    if (pin === "2207") {
      sessionStorage.setItem("paymentg_sandbox_unlocked", "true");
      document.getElementById("pin-lockscreen").style.display = "none";
      initSandbox();
    } else {
      errElem.innerText = "Gagal menghubungi server API (" + err.message + ")";
      errElem.style.display = "block";
    }
  } finally {
    btn.disabled = false;
    btn.innerText = "Buka Simulator";
  }
}

function initSandbox() {
  loadAllSandboxData();

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

      if (btn.dataset.tab === "tab-sandbox-orders") loadSandboxOrders();
      if (btn.dataset.tab === "tab-sandbox-stats") loadSandboxStats();
      if (btn.dataset.tab === "tab-sandbox-apps") loadSandboxApps();
    });
  });
}

// ==========================================
// 2. HTTP API Client Helper
// ==========================================
async function apiRequest(endpoint, method = "GET", body = null) {
  const url = `${CONFIG.API_URL}${endpoint}`;
  const headers = { 
    "Content-Type": "application/json",
    "X-Admin-Key": CONFIG.ADMIN_KEY
  };

  const options = { method, headers };
  if (body) options.body = JSON.stringify(body);

  try {
    const res = await fetch(url, options);
    const data = await res.json();
    return { ok: res.ok, status: res.status, data };
  } catch (err) {
    console.error(`Sandbox API Error (${endpoint}):`, err);
    return { ok: false, error: err.message };
  }
}

// ==========================================
// 3. Data Loaders
// ==========================================
async function loadAllSandboxData() {
  await loadSandboxOrders();
  await loadSandboxStats();
  await loadSandboxApps();
}

async function loadSandboxStats() {
  const res = await apiRequest("/api/sandbox/stats", "GET");
  if (res.ok && res.data.success) {
    const s = res.data.data;
    document.getElementById("stat-revenue").innerText = "Rp " + Number(s.total_revenue || 0).toLocaleString("id-ID");
    document.getElementById("stat-paid-count").innerText = s.paid_orders || 0;
    document.getElementById("stat-pending-count").innerText = s.pending_orders || 0;
    document.getElementById("stat-total-orders").innerText = s.total_orders || 0;
  }
}

// ==========================================
// 4. Mutasi Transaksi & Simulator
// ==========================================
async function loadSandboxOrders() {
  const tbody = document.getElementById("sandbox-orders-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/sandbox/orders?limit=100", "GET");
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--danger); padding: 24px;">Gagal memuat data sandbox. Periksa server API.</td></tr>`;
    return;
  }

  allSandboxOrders = res.data.data || [];
  renderOrdersTable();
}

function renderOrdersTable() {
  const tbody = document.getElementById("sandbox-orders-tbody");
  if (!tbody) return;

  const filterStatus = document.getElementById("filter-status") ? document.getElementById("filter-status").value : "ALL";
  const search = document.getElementById("search-order") ? document.getElementById("search-order").value.toLowerCase().trim() : "";

  const filtered = allSandboxOrders.filter(o => {
    const matchesStatus = filterStatus === "ALL" || o.status === filterStatus;
    const matchesSearch = !search || 
      (o.id && o.id.toLowerCase().includes(search)) ||
      (o.reference_id && o.reference_id.toLowerCase().includes(search)) ||
      (o.shopee_tx_id && o.shopee_tx_id.toLowerCase().includes(search));
    return matchesStatus && matchesSearch;
  });

  if (filtered.length === 0) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-muted); padding: 32px;">Belum ada pesanan testing di sandbox. Lakukan checkout di web toko Anda!</td></tr>`;
    return;
  }

  tbody.innerHTML = filtered.map(o => {
    let badgeClass = "badge-pending";
    if (o.status === "PAID") badgeClass = "badge-paid";
    else if (o.status === "CANCELLED") badgeClass = "badge-cancelled";

    const dateFormatted = o.created_at ? new Date(o.created_at).toLocaleString("id-ID", { dateStyle: "short", timeStyle: "short" }) : "-";

    return `
      <tr>
        <td>
          <div style="font-weight: 700; color: var(--text-main); font-family: monospace;">${o.id}</div>
          <div style="font-size: 11px; color: var(--text-muted); font-family: monospace;">${o.reference_id || '-'}</div>
        </td>
        <td class="font-mono">Rp ${Number(o.original_amount).toLocaleString("id-ID")}</td>
        <td><span style="color: #d97706; font-weight: 700; font-family: monospace;">+${o.unique_code}</span></td>
        <td><b class="font-mono" style="font-size: 14px; color: var(--text-main);">Rp ${Number(o.total_amount).toLocaleString("id-ID")}</b></td>
        <td><span class="badge ${badgeClass}">${o.status}</span></td>
        <td><code class="font-mono" style="font-size: 11px; color: var(--text-muted);">${o.shopee_tx_id || '-'}</code></td>
        <td style="font-size: 11.5px; color: var(--text-muted);">${dateFormatted}</td>
        <td style="text-align: right;">
          <div style="display: inline-flex; gap: 6px; justify-content: flex-end; align-items: center;">
            ${o.status === 'PENDING' ? `
              <button class="btn btn-pay-simulator btn-sm" onclick="simulatePay('${o.id}', this)" title="Simulasikan pembayaran lunas seketika">
                💳 Bayar Sekarang
              </button>
              <button class="btn btn-secondary btn-sm" onclick="showQRModal('${o.id}', ${o.total_amount}, '${o.status}')">
                QR
              </button>
              <button class="btn btn-secondary btn-sm" style="color: var(--danger);" onclick="cancelSandboxOrder('${o.id}')" title="Batalkan">
                ✕
              </button>
            ` : `
              <span style="color: #10b981; font-size: 12px; font-weight: 700;">✓ Lunas (Simulasi)</span>
              <button class="btn btn-secondary btn-xs" onclick="showQRModal('${o.id}', ${o.total_amount}, '${o.status}')">
                Detail
              </button>
            `}
          </div>
        </td>
      </tr>
    `;
  }).join("");
}

// --- Simulator Action: 1-Click Pay Now ---
async function simulatePay(orderId, btnElement) {
  if (btnElement) {
    btnElement.disabled = true;
    btnElement.innerText = "Memproses...";
  }

  const res = await apiRequest(`/api/sandbox/orders/${orderId}/pay`, "POST", {});

  if (btnElement) {
    btnElement.disabled = false;
    btnElement.innerText = "💳 Bayar Sekarang";
  }

  if (res.ok && res.data.success) {
    const data = res.data.data;
    
    // Tampilkan Webhook Inspector Card
    const inspector = document.getElementById("webhook-inspector-box");
    const badge = document.getElementById("webhook-status-badge");
    const details = document.getElementById("webhook-details");

    if (inspector) {
      inspector.style.display = "block";
      if (data.webhook_sent) {
        badge.className = "badge badge-paid";
        badge.innerText = "WEBHOOK SUKSES TERKIRIM (200 OK)";
        details.innerHTML = `
          <div><b>Order ID:</b> <code>${data.order.id}</code> | <b>Simulasi Tx ID:</b> <code>${data.order.shopee_tx_id}</code></div>
          <div style="color: #065f46; margin-top: 4px; font-weight: 600;">🎉 Notifikasi Webhook berhasil diterima web toko Anda! Database toko Anda sekarang telah lunas.</div>
        `;
      } else {
        badge.className = "badge badge-expired";
        badge.innerText = "WEBHOOK GAGAL / REFUSED";
        details.innerHTML = `
          <div><b>Order ID:</b> <code>${data.order.id}</code></div>
          <div style="color: var(--danger); margin-top: 4px; font-weight: 600;">⚠️ Gagal mengirim webhook ke web toko: <code>${data.webhook_error || 'Target web toko tidak merespons'}</code></div>
          <div style="font-size: 11.5px; margin-top: 2px;">Pastikan server web toko Anda aktif dan URL Webhook di tab "Kelola Web Toko Sandbox" sudah benar.</div>
        `;
      }
      inspector.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }

    showToast("🎉 Pembayaran simulasi sukses & webhook ditembak!");
    loadAllSandboxData();
  } else {
    alert("Gagal simulasi pembayaran: " + (res.data ? res.data.error : res.error));
  }
}

async function cancelSandboxOrder(orderId) {
  if (!confirm(`Batalkan pesanan testing ${orderId}?`)) return;
  const res = await apiRequest(`/api/sandbox/orders/${orderId}/cancel`, "POST", {});
  if (res.ok && res.data.success) {
    showToast(`Order ${orderId} dibatalkan.`);
    loadAllSandboxData();
  } else {
    alert("Gagal membatalkan order.");
  }
}

// ==========================================
// 5. Kelola Web Toko Sandbox (Tab Apps)
// ==========================================
async function loadSandboxApps() {
  const tbody = document.getElementById("sandbox-apps-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/sandbox/apps", "GET");
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--danger); padding: 24px;">Gagal memuat daftar app sandbox.</td></tr>`;
    return;
  }

  allSandboxApps = res.data.data || [];

  if (allSandboxApps.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 32px;">Belum ada web toko testing yang didaftarkan. Gunakan form di sebelah kiri untuk membuat.</td></tr>`;
    return;
  }

  tbody.innerHTML = allSandboxApps.map(a => `
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
        <span style="font-size: 11.5px; font-family: monospace; color: var(--text-muted);" title="${a.webhook_url}">
          ${a.webhook_url.length > 36 ? a.webhook_url.substring(0, 36) + '...' : a.webhook_url}
        </span>
      </td>
      <td>
        <span class="badge badge-paid">Aktif Sandbox</span>
      </td>
      <td style="text-align: right;">
        <button class="btn btn-danger btn-xs" onclick="deleteSandboxApp('${a.id}', '${a.name}')">Hapus</button>
      </td>
    </tr>
  `).join("");
}

async function submitCreateSandboxApp(event) {
  event.preventDefault();
  const name = document.getElementById("app-name").value.trim();
  const webhook = document.getElementById("app-webhook").value.trim();
  if (!name || !webhook) return;

  const btn = document.getElementById("btn-create-app");
  btn.disabled = true;
  btn.innerText = "Mendaftarkan Toko Testing...";

  const res = await apiRequest("/api/sandbox/apps", "POST", { name, webhook_url: webhook });
  btn.disabled = false;
  btn.innerHTML = `<span>➕</span> Dapatkan API Key Sandbox`;

  if (res.ok && res.data.success) {
    const created = res.data.data;

    // Populate Modal
    document.getElementById("created-modal-name").innerText = created.name;
    document.getElementById("created-modal-apikey").value = created.api_key;

    const envSnippet = 
`# Konfigurasi PaymentG Sandbox (${created.name})
PAYMENTG_API_URL=${CONFIG.API_URL}/api/sandbox
PAYMENTG_API_KEY=${created.api_key}`;

    document.getElementById("created-modal-env").innerText = envSnippet;

    // Open Modal
    document.getElementById("created-app-modal").classList.add("active");

    // Reset Form
    document.getElementById("app-name").value = "";
    document.getElementById("app-webhook").value = "";

    loadSandboxApps();
  } else {
    alert("Gagal membuat app sandbox: " + (res.data ? res.data.error : res.error));
  }
}

function copyEnvSnippet(btn) {
  const code = document.getElementById("created-modal-env").innerText;
  copyToClipboard(code, btn);
}

async function deleteSandboxApp(id, name) {
  if (!confirm(`Hapus toko testing "${name}"?`)) return;
  const res = await apiRequest(`/api/sandbox/apps/${id}`, "DELETE");
  if (res.ok) {
    showToast("Toko sandbox berhasil dihapus!");
    loadSandboxApps();
  } else {
    alert("Gagal menghapus toko sandbox.");
  }
}

// ==========================================
// 6. Clipboard & Toast Helpers
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
// 7. Modal QR Sandbox
// ==========================================
function showQRModal(orderId, totalAmount, status = "PENDING") {
  const modal = document.getElementById("qr-modal");
  document.getElementById("modal-qr-img").src = `${CONFIG.API_URL}/api/sandbox/orders/${orderId}/qr.png`;
  document.getElementById("modal-qr-id").innerText = orderId;
  document.getElementById("modal-qr-amount").innerText = "Rp " + Number(totalAmount).toLocaleString("id-ID");
  
  const statusElem = document.getElementById("modal-qr-status");
  if (statusElem) {
    statusElem.innerText = status;
    statusElem.className = `badge ${status === 'PAID' ? 'badge-paid' : 'badge-pending'}`;
  }

  modal.classList.add("active");
}

// ==========================================
// 8. Settings Modal
// ==========================================
function setupSettingsModal() {
  const modal = document.getElementById("settings-modal");
  const openBtn = document.getElementById("btn-open-settings");
  const closeBtn = document.getElementById("btn-close-settings");

  if (openBtn) {
    openBtn.addEventListener("click", () => {
      document.getElementById("setting-api-url").value = CONFIG.API_URL;
      document.getElementById("setting-admin-key").value = CONFIG.ADMIN_KEY;
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

      localStorage.setItem("paymentg_api_url", CONFIG.API_URL);
      localStorage.setItem("paymentg_admin_key", CONFIG.ADMIN_KEY);

      modal.classList.remove("active");
      showToast("Pengaturan Sandbox disimpan!");
      loadAllSandboxData();
    });
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
    loadSandboxOrders();
    loadSandboxStats();
  }, 4000);
}

function stopAutoRefresh() {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer);
  autoRefreshTimer = null;
}
