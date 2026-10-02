/**
 * PaymentG — Dedicated Sandbox Frontend App
 */

const DEFAULT_BACKEND = window.location.protocol.startsWith("http") ? window.location.origin : "http://localhost:3200";

const CONFIG = {
  API_URL: localStorage.getItem("paymentg_api_url") || DEFAULT_BACKEND,
  ADMIN_KEY: localStorage.getItem("paymentg_admin_key") || "adm_secret_paymentg_2026",
  API_KEY: localStorage.getItem("paymentg_sandbox_api_key") || ""
};

let allSandboxOrders = [];
let autoRefreshTimer = null;

document.addEventListener("DOMContentLoaded", () => {
  checkLockScreen();
  setupTabs();
  setupSettingsModal();
});

// --- 0. PIN Access Gate ---
function checkLockScreen() {
  const isUnlocked = sessionStorage.getItem("paymentg_sandbox_unlocked") === "true";
  const lockOverlay = document.getElementById("pin-lockscreen");

  if (!isUnlocked) {
    if (lockOverlay) lockOverlay.style.display = "flex";
    const field = document.getElementById("pin-field");
    if (field) setTimeout(() => field.focus(), 100);
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

// --- 1. Tabs Navigation ---
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

// --- 2. API Helper ---
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
    console.error(`Sandbox API Error (${endpoint}):`, err);
    return { ok: false, error: err.message };
  }
}

// --- 3. Data Loaders ---
async function loadAllSandboxData() {
  await loadSandboxOrders();
  await loadSandboxStats();
  await loadSandboxApps();
}

async function loadSandboxStats() {
  const res = await apiRequest("/api/sandbox/stats", "GET", null, true);
  if (res.ok && res.data.success) {
    const s = res.data.data;
    document.getElementById("stat-revenue").innerText = "Rp " + Number(s.total_revenue || 0).toLocaleString("id-ID");
    document.getElementById("stat-paid-count").innerText = s.paid_orders || 0;
    document.getElementById("stat-pending-count").innerText = s.pending_orders || 0;
    document.getElementById("stat-total-orders").innerText = s.total_orders || 0;
  }
}

async function loadSandboxOrders() {
  const tbody = document.getElementById("sandbox-orders-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/sandbox/orders?limit=100", "GET", null, true);
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--danger);">Gagal memuat data sandbox. Periksa server API.</td></tr>`;
    return;
  }

  allSandboxOrders = res.data.data || [];
  renderOrdersTable();
}

function renderOrdersTable() {
  const tbody = document.getElementById("sandbox-orders-tbody");
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
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-muted); padding: 24px;">Belum ada pesanan testing di sandbox.</td></tr>`;
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
          <div style="font-weight: 700; color: var(--text-main);">${o.id}</div>
          <div style="font-size: 11px; color: var(--text-muted);">${o.reference_id || '-'}</div>
        </td>
        <td>Rp ${Number(o.original_amount).toLocaleString("id-ID")}</td>
        <td><span style="color: var(--sandbox-accent); font-weight: 700;">+${o.unique_code}</span></td>
        <td><b style="font-size: 14px;">Rp ${Number(o.total_amount).toLocaleString("id-ID")}</b></td>
        <td><span class="badge ${badgeClass}">${o.status}</span></td>
        <td><code style="font-size: 11px; color: var(--text-muted);">${o.shopee_tx_id || '-'}</code></td>
        <td style="font-size: 11.5px; color: var(--text-muted);">${dateFormatted}</td>
        <td>
          <div style="display: flex; gap: 6px; align-items: center;">
            ${o.status === 'PENDING' ? `
              <button class="btn btn-pay-simulator btn-sm" onclick="simulatePay('${o.id}')" title="Simulasikan pembayaran lunas seketika">
                💳 Bayar Sekarang
              </button>
              <button class="btn btn-secondary btn-sm" style="color: var(--danger);" onclick="cancelSandboxOrder('${o.id}')" title="Batalkan">
                ✕
              </button>
            ` : `
              <span style="color: var(--primary); font-size: 12px; font-weight: 700;">✓ Lunas</span>
            `}
          </div>
        </td>
      </tr>
    `;
  }).join("");
}

// --- 4. Simulator Action (Pay Now) ---
async function simulatePay(orderId) {
  const btn = event ? event.target : null;
  if (btn) {
    btn.disabled = true;
    btn.innerText = "Memproses...";
  }

  const res = await apiRequest(`/api/sandbox/orders/${orderId}/pay`, "POST", {}, true);

  if (res.ok && res.data.success) {
    const data = res.data.data;
    
    // Tampilkan Webhook Inspector
    const inspector = document.getElementById("webhook-inspector-box");
    const badge = document.getElementById("webhook-status-badge");
    const details = document.getElementById("webhook-details");

    if (inspector) {
      inspector.style.display = "block";
      if (data.webhook_sent) {
        badge.className = "badge badge-paid";
        badge.innerText = "WEBHOOK SUKSES TERKIRIM (200 OK)";
        details.innerHTML = `
          <div><b>Order ID:</b> <code>${data.order.id}</code> | <b>Tx ID:</b> <code>${data.order.shopee_tx_id}</code></div>
          <div style="color: var(--primary); margin-top: 4px;">🎉 Notifikasi Webhook berhasil ditembakkan ke web toko Anda! Database toko Anda sekarang harusnya sudah berstatus LUNAS.</div>
        `;
      } else {
        badge.className = "badge badge-expired";
        badge.innerText = "WEBHOOK GAGAL / REFUSED";
        details.innerHTML = `
          <div><b>Order ID:</b> <code>${data.order.id}</code></div>
          <div style="color: var(--danger); margin-top: 4px;">⚠️ Gagal mengirim webhook ke toko: <code>${data.webhook_error || 'Target web toko tidak merespons'}</code></div>
          <div style="font-size: 11px; margin-top: 2px;">Pastikan server web toko Anda sedang aktif dan URL Webhook di tab "Kelola App Sandbox" sudah benar.</div>
        `;
      }
    }

    loadAllSandboxData();
    alert(`🎉 Simulasi Pembayaran Sukses!\n\nOrder: ${data.order.id}\nNominal: Rp ${Number(data.order.total_amount).toLocaleString("id-ID")}\nWebhook: ${data.webhook_sent ? 'Terkirim ✓' : 'Gagal (' + data.webhook_error + ')'}`);
  } else {
    alert("Gagal simulasi pembayaran: " + (res.data ? res.data.error : res.error));
  }

  if (btn) {
    btn.disabled = false;
    btn.innerText = "💳 Bayar Sekarang";
  }
}

async function cancelSandboxOrder(orderId) {
  if (!confirm(`Batalkan pesanan testing ${orderId}?`)) return;
  const res = await apiRequest(`/api/sandbox/orders/${orderId}/cancel`, "POST", {}, false);
  if (res.ok && res.data.success) {
    loadAllSandboxData();
  } else {
    alert("Gagal membatalkan order.");
  }
}

// --- 5. Generate QRIS Sandbox ---
async function submitCreateSandboxOrder(event) {
  event.preventDefault();
  const btn = document.getElementById("btn-generate-order");
  const amount = parseInt(document.getElementById("gen-amount").value);
  const refId = document.getElementById("gen-ref-id").value.trim() || ("SBX-" + Date.now());
  const expiry = parseInt(document.getElementById("gen-expiry").value) || 15;

  btn.disabled = true;
  btn.innerText = "Membuat QRIS Sandbox...";

  const res = await apiRequest("/api/sandbox/orders", "POST", {
    amount: amount,
    reference_id: refId,
    expiry_minutes: expiry
  }, false);

  btn.disabled = false;
  btn.innerText = "Generate QRIS Sandbox";

  if (res.ok && res.data.success) {
    const order = res.data.data;
    const box = document.getElementById("qr-result-box");
    box.style.display = "block";
    document.getElementById("qr-res-img").src = `${CONFIG.API_URL}${order.qr_url}`;
    document.getElementById("qr-res-total").innerText = "Rp " + Number(order.total_amount).toLocaleString("id-ID");

    const quickPayBtn = document.getElementById("btn-quick-pay-now");
    quickPayBtn.onclick = () => simulatePay(order.order_id);

    loadAllSandboxData();
  } else {
    alert("Gagal membuat order sandbox: " + (res.data ? res.data.error : res.error) + "\n\nPastikan Anda sudah membuat App Sandbox di tab 'Kelola App Sandbox'!");
  }
}

// --- 6. Apps Management ---
async function loadSandboxApps() {
  const tbody = document.getElementById("sandbox-apps-tbody");
  if (!tbody) return;

  const res = await apiRequest("/api/sandbox/apps", "GET", null, true);
  if (!res.ok || !res.data.success) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--danger);">Gagal memuat daftar app sandbox.</td></tr>`;
    return;
  }

  const apps = res.data.data || [];
  if (apps.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 20px;">Belum ada app sandbox yang didaftarkan.</td></tr>`;
    return;
  }

  tbody.innerHTML = apps.map(a => `
    <tr>
      <td><b>${a.name}</b></td>
      <td><code>${a.api_key}</code></td>
      <td style="font-size: 11.5px;">${a.webhook_url}</td>
      <td><span class="badge badge-paid">Aktif Sandbox</span></td>
      <td>
        <button class="btn btn-secondary btn-sm" style="color: var(--danger);" onclick="deleteSandboxApp('${a.id}')">Hapus</button>
      </td>
    </tr>
  `).join("");
}

async function submitCreateSandboxApp(event) {
  event.preventDefault();
  const name = document.getElementById("app-name").value.trim();
  const webhook = document.getElementById("app-webhook").value.trim();
  if (!name || !webhook) return;

  const res = await apiRequest("/api/sandbox/apps", "POST", { name, webhook_url: webhook }, true);
  if (res.ok && res.data.success) {
    const created = res.data.data;
    CONFIG.API_KEY = created.api_key;
    localStorage.setItem("paymentg_sandbox_api_key", created.api_key);
    alert(`App Sandbox berhasil dibuat!\n\nAPI Key: ${created.api_key}\nWebhook Secret: ${created.webhook_secret}\n\nKredensial otomatis tersimpan di sandbox dashboard!`);
    document.getElementById("app-name").value = "";
    document.getElementById("app-webhook").value = "";
    loadSandboxApps();
  } else {
    alert("Gagal membuat app sandbox: " + (res.data ? res.data.error : res.error));
  }
}

async function deleteSandboxApp(id) {
  if (!confirm("Hapus app sandbox ini?")) return;
  const res = await apiRequest(`/api/sandbox/apps/${id}`, "DELETE", null, true);
  if (res.ok) {
    loadSandboxApps();
  } else {
    alert("Gagal menghapus app sandbox.");
  }
}

// Auto Refresh Control
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

// Settings Modal
function setupSettingsModal() {
  const modal = document.getElementById("settings-modal");
  const openBtn = document.getElementById("btn-open-settings");

  if (openBtn) {
    openBtn.addEventListener("click", () => {
      document.getElementById("setting-api-url").value = CONFIG.API_URL;
      document.getElementById("setting-admin-key").value = CONFIG.ADMIN_KEY;
      document.getElementById("setting-api-key").value = CONFIG.API_KEY;
      modal.classList.add("active");
    });
  }

  const saveBtn = document.getElementById("btn-save-settings");
  if (saveBtn) {
    saveBtn.addEventListener("click", () => {
      CONFIG.API_URL = document.getElementById("setting-api-url").value.trim().replace(/\/$/, "");
      CONFIG.ADMIN_KEY = document.getElementById("setting-admin-key").value.trim();
      CONFIG.API_KEY = document.getElementById("setting-api-key").value.trim();

      localStorage.setItem("paymentg_api_url", CONFIG.API_URL);
      localStorage.setItem("paymentg_admin_key", CONFIG.ADMIN_KEY);
      localStorage.setItem("paymentg_sandbox_api_key", CONFIG.API_KEY);

      modal.classList.remove("active");
      loadAllSandboxData();
      alert("Pengaturan API Sandbox berhasil disimpan!");
    });
  }
}

function closeModal(id) {
  const modal = document.getElementById(id);
  if (modal) modal.classList.remove("active");
}
