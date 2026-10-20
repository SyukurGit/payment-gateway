/**
 * Multipay Gateway — Master Dashboard Controller
 * Vanilla ES6 SPA Controller for 6 Isolated Channels & Standardized 4-Kuadran UX
 */

// Channel Metadata
const CHANNEL_META = {
  qris: {
    title: 'ShopeePay Dynamic QRIS',
    icon: '📱',
    currency: 'IDR',
    min: 1000,
    step: 1,
    defaultAmount: 25000,
    refPrefix: 'QRS',
    color: 'shopee',
    desc: 'EMVCo Dynamic TLV QRIS dengan kode unik 1 - 999 IDR'
  },
  binance: {
    title: 'Binance Pay SAPI (Personal)',
    icon: '🟡',
    currency: 'USDT',
    min: 0.01,
    step: 0.01,
    defaultAmount: 10.00,
    refPrefix: 'BNB',
    color: 'gold',
    desc: 'Milli-units matcher (0.001 USDT) dengan clock drift auto-correction'
  },
  usdt: {
    title: 'USDT BEP-20 On-Chain (BSC)',
    icon: '🟢',
    currency: 'USDT',
    min: 0.01,
    step: 0.01,
    defaultAmount: 15.00,
    refPrefix: 'BSC',
    color: 'usdt',
    desc: 'Non-custodial watch-only dengan multi-node RPC failover'
  }
};

// Global State
const state = {
  adminKey: sessionStorage.getItem('multipay_admin_key') || '',
  activeTab: 'overview',
  overviewIsSandbox: false,
  autoRefresh: true,
  refreshTimer: null,
  apps: [],
  selectedAppId: '',
  activeLiveKey: 'ak_live_multipay_store_2026',
  activeSandboxKey: 'ak_test_multipay_store_2026',
  activeWebhookSecret: 'sec_multipay_default_2026',
  docTabs: {},        // key -> 'curl' | 'req' | 'res' | 'wh' | 'php' | 'node'
  activeInvoices: {}, // key -> order object
  countdownTimers: {} // key -> interval ID
};

// =========================================================================
// 1. Initialization & Session Management
// =========================================================================
document.addEventListener('DOMContentLoaded', () => {
  mountChannelViews();
  checkAuthSession();
});

function getHeaders(customHeaders = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...customHeaders
  };
  if (state.adminKey) {
    headers['X-Admin-Key'] = state.adminKey;
  }
  return headers;
}

async function checkAuthSession() {
  if (!state.adminKey) {
    showPinModal();
    return;
  }

  try {
    const res = await fetch('/api/admin/stats', {
      headers: getHeaders()
    });

    if (res.status === 401 || res.status === 403) {
      state.adminKey = '';
      sessionStorage.removeItem('multipay_admin_key');
      showPinModal();
      return;
    }

    closePinModal();
    initDashboard();
  } catch (err) {
    console.error('Session check failed:', err);
    showPinModal();
  }
}

function showPinModal() {
  const modal = document.getElementById('pinModal');
  if (modal) {
    modal.classList.remove('hidden');
    const input = document.getElementById('adminPinInput');
    if (input) {
      input.value = '';
      input.focus();
    }
  }
}

function closePinModal() {
  const modal = document.getElementById('pinModal');
  if (modal) modal.classList.add('hidden');
}

async function handleLogin(e) {
  e.preventDefault();
  const pinInput = document.getElementById('adminPinInput');
  const errorEl = document.getElementById('pinError');
  const submitBtn = e.target.querySelector('button[type="submit"]');
  const pin = pinInput.value.trim();

  if (!pin) return;
  if (errorEl) errorEl.classList.add('hidden');
  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Memverifikasi...</span>';
  }

  try {
    const res = await fetch('/api/admin/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pin: pin })
    });

    const data = await res.json();
    if (res.ok && data.success) {
      const key = data.admin_key || data.token;
      state.adminKey = key;
      sessionStorage.setItem('multipay_admin_key', key);
      closePinModal();
      initDashboard();
      showToast('Login berhasil!', 'success');
    } else {
      if (errorEl) {
        errorEl.textContent = data.error || 'PIN Admin salah.';
        errorEl.classList.remove('hidden');
      }
      pinInput.value = '';
      pinInput.focus();
    }
  } catch (err) {
    if (errorEl) {
      errorEl.textContent = 'Gagal menghubungi server.';
      errorEl.classList.remove('hidden');
    }
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>Buka Dashboard</span>';
    }
  }
}

function handleLogout() {
  state.adminKey = '';
  sessionStorage.removeItem('multipay_admin_key');
  clearInterval(state.refreshTimer);
  showPinModal();
  showToast('Sesi telah dikunci.', 'info');
}

function initDashboard() {
  loadConfig();
  loadApps();
  switchTab(state.activeTab || 'overview');
  startAutoRefresh();
}

// =========================================================================
// 2. Programmatic 6-View DOM Mounting (Symmetrical 4-Kuadran Architecture)
// =========================================================================
function mountChannelViews() {
  const container = document.getElementById('channelViewsContainer');
  if (!container) return;

  const channelKeys = [
    { channel: 'qris', isSandbox: false },
    { channel: 'qris', isSandbox: true },
    { channel: 'binance', isSandbox: false },
    { channel: 'binance', isSandbox: true },
    { channel: 'usdt', isSandbox: false },
    { channel: 'usdt', isSandbox: true }
  ];

  container.innerHTML = channelKeys.map(({ channel, isSandbox }) => {
    const meta = CHANNEL_META[channel];
    const mode = isSandbox ? 'sandbox' : 'live';
    const key = `${channel}-${mode}`;
    const modeBadge = isSandbox
      ? `<span class="px-2.5 py-0.5 rounded-lg text-[10px] font-mono font-bold bg-yellow-950/80 text-yellow-400 border border-yellow-800/60 uppercase">SANDBOX (UJI COBA)</span>`
      : `<span class="px-2.5 py-0.5 rounded-lg text-[10px] font-mono font-bold bg-emerald-950 text-emerald-400 border border-emerald-800 uppercase">LIVE (PRODUKSI)</span>`;

    const endpointPath = isSandbox ? `/${channel}/api/sandbox/orders` : `/${channel}/api/orders`;

    // Live specific runtime CRUD forms
    let runtimeConfigHTML = '';
    if (!isSandbox) {
      if (channel === 'qris') {
        runtimeConfigHTML = `
          <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-4">
            <div class="flex justify-between items-center">
              <h4 class="text-xs font-bold text-white flex items-center gap-2">
                <span class="text-shopee">⚙️</span> Runtime CRUD ShopeePay Token & QRIS
              </h4>
              <span class="text-[10px] text-muted font-mono">Direct SQLite (No Restart)</span>
            </div>
            <form onsubmit="handleSaveQRISConfig(event)" class="space-y-3">
              <div>
                <label class="block text-[11px] font-semibold text-muted mb-1">ShopeePay Bearer Token / Refresh Cookie</label>
                <textarea id="qrisTokenInput" rows="2" placeholder="eyJhbGciOi..."
                          class="w-full bg-darker border border-borderDark rounded-xl p-2.5 text-xs text-white font-mono focus:border-shopee"></textarea>
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-muted mb-1">Static Merchant QRIS String</label>
                <textarea id="qrisStringInput" rows="2" placeholder="000201010211266..."
                          class="w-full bg-darker border border-borderDark rounded-xl p-2.5 text-xs text-white font-mono focus:border-shopee"></textarea>
              </div>
              <button type="submit" class="w-full bg-shopee hover:bg-shopeeHover text-white font-bold py-2 px-3 rounded-xl text-xs transition shadow flex items-center justify-center gap-2">
                <span>💾</span>
                <span>Simpan Konfigurasi ShopeePay</span>
              </button>
            </form>
          </div>
        `;
      } else if (channel === 'binance') {
        runtimeConfigHTML = `
          <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
            <div class="flex justify-between items-center">
              <h4 class="text-xs font-bold text-white flex items-center gap-2">
                <span class="text-gold">⚙️</span> Runtime CRUD Binance SAPI & Pay ID
              </h4>
              <button onclick="handleTestBinance()" type="button" class="text-[10px] bg-gold hover:bg-yellow-500 text-black font-bold px-2.5 py-1 rounded-lg transition">
                🧪 Tes SAPI
              </button>
            </div>
            <form onsubmit="handleSaveBinanceConfig(event)" class="space-y-2.5">
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-[10px] font-semibold text-muted mb-1">Binance Pay ID / UID</label>
                  <input type="text" id="binancePayIDInput" placeholder="12345678" required
                         class="w-full bg-darker border border-borderDark rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:border-gold">
                </div>
                <div>
                  <label class="block text-[10px] font-semibold text-muted mb-1">Nama Akun Toko</label>
                  <input type="text" id="binanceAccountNameInput" placeholder="StoreOfficial"
                         class="w-full bg-darker border border-borderDark rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:border-gold">
                </div>
              </div>
              <div>
                <label class="block text-[10px] font-semibold text-muted mb-1">Binance API Key (Read-Only)</label>
                <input type="password" id="binanceAPIKeyInput" placeholder="API Key..."
                       class="w-full bg-darker border border-borderDark rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:border-gold">
              </div>
              <div>
                <label class="block text-[10px] font-semibold text-muted mb-1">Binance Secret Key</label>
                <input type="password" id="binanceSecretKeyInput" placeholder="Secret Key..."
                       class="w-full bg-darker border border-borderDark rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:border-gold">
              </div>
              <button type="submit" class="w-full bg-gold hover:bg-yellow-500 text-black font-bold py-2 px-3 rounded-xl text-xs transition shadow flex items-center justify-center gap-2">
                <span>💾</span>
                <span>Simpan Pengaturan Binance Pay</span>
              </button>
            </form>
          </div>
        `;
      } else if (channel === 'usdt') {
        runtimeConfigHTML = `
          <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
            <div class="flex justify-between items-center">
              <h4 class="text-xs font-bold text-white flex items-center gap-2">
                <span class="text-usdt">⚙️</span> Runtime CRUD Wallet BEP-20 & RPC
              </h4>
              <button onclick="handleTestUSDT()" type="button" class="text-[10px] bg-usdt hover:bg-usdtHover text-white font-bold px-2.5 py-1 rounded-lg transition">
                🧪 Tes RPC
              </button>
            </div>
            <form onsubmit="handleSaveUSDTConfig(event)" class="space-y-2.5">
              <div>
                <label class="block text-[10px] font-semibold text-muted mb-1">Alamat Wallet Penerima (BEP-20)</label>
                <input type="text" id="usdtWalletInput" placeholder="0x..." required
                       class="w-full bg-darker border border-borderDark rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:border-usdt">
              </div>
              <div>
                <label class="block text-[10px] font-semibold text-muted mb-1">Daftar RPC Endpoints (Dipisah Koma)</label>
                <textarea id="usdtRPCInput" rows="2" placeholder="https://bsc-rpc.publicnode.com,https://1rpc.io/bnb"
                          class="w-full bg-darker border border-borderDark rounded-xl p-2.5 text-xs text-white font-mono focus:border-usdt"></textarea>
              </div>
              <button type="submit" class="w-full bg-usdt hover:bg-usdtHover text-white font-bold py-2 px-3 rounded-xl text-xs transition shadow flex items-center justify-center gap-2">
                <span>💾</span>
                <span>Simpan Alamat Wallet & RPC</span>
              </button>
            </form>
          </div>
        `;
      }
    }

    return `
      <div id="view-${key}" class="tab-view space-y-6 hidden animate-fade">

        <!-- ======================================================== -->
        <!-- HEADER / CREDENTIALS BAR                                 -->
        <!-- ======================================================== -->
        <div class="bg-card border border-borderDark rounded-2xl p-4 flex flex-col lg:flex-row justify-between items-start lg:items-center gap-4 shadow">
          <div class="flex items-center space-x-3">
            <div class="w-12 h-12 rounded-xl bg-cardHover border border-borderDark flex items-center justify-center text-2xl shrink-0">
              ${meta.icon}
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-base font-bold text-white">${meta.title}</h3>
                ${modeBadge}
              </div>
              <p class="text-xs text-muted mt-0.5">${meta.desc}</p>
            </div>
          </div>

          <!-- Quick Credentials Display -->
          <div class="flex flex-wrap items-center gap-2 text-xs font-mono">
            <!-- Endpoint Pill -->
            <div class="bg-darker border border-borderDark px-3 py-1.5 rounded-xl flex items-center gap-2">
              <span class="text-muted text-[10px]">ENDPOINT:</span>
              <span class="text-white font-semibold">${endpointPath}</span>
              <button onclick="copyToClipboard(window.location.origin + '${endpointPath}')" class="text-muted hover:text-white" title="Salin Full URL">📋</button>
            </div>

            <!-- API Key Pill -->
            <div class="bg-darker border border-borderDark px-3 py-1.5 rounded-xl flex items-center gap-2">
              <span class="text-muted text-[10px]">${isSandbox ? 'SANDBOX KEY' : 'LIVE KEY'}:</span>
              <span id="key-val-${key}" class="font-semibold ${isSandbox ? 'text-yellow-400' : 'text-emerald-400'} truncate max-w-[140px]">-</span>
              <button onclick="copyActiveApiKey('${key}', ${isSandbox})" class="text-muted hover:text-white" title="Salin API Key">📋</button>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- ROW 1: KUADRAN 1 (GENERATOR) & KUADRAN 2 (DOCS)          -->
        <!-- ======================================================== -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">

          <!-- KUADRAN 1: Universal Invoice Generator & Simulator -->
          <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-4">
            <div class="flex justify-between items-center">
              <h4 class="text-sm font-bold text-white flex items-center gap-2">
                <span>⚡</span> Generator & Uji Tagihan (${isSandbox ? 'Sandbox' : 'Live'})
              </h4>
              <span class="text-[10px] text-muted font-mono font-semibold">${meta.currency}</span>
            </div>
            <p class="text-xs text-muted">
              ${isSandbox
                ? 'Generate tagihan simulasi instan untuk menguji alur pembayaran, perolehan QR, dan callback webhook toko Anda.'
                : 'Generate tagihan riil dan pantau alokasi kode unik serta status mutasi masuk secara langsung.'}
            </p>

            <form onsubmit="handleCreateInvoice('${channel}', ${isSandbox}, event)" class="space-y-3">
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block text-[11px] font-semibold text-muted mb-1">Nominal (${meta.currency})</label>
                  <input type="number" id="gen-amount-${key}" min="${meta.min}" step="${meta.step}" value="${meta.defaultAmount}" required
                         class="w-full bg-darker border border-borderDark rounded-xl px-3 py-2 text-xs text-white font-mono focus:border-gold">
                </div>
                <div>
                  <label class="block text-[11px] font-semibold text-muted mb-1">Masa Berlaku</label>
                  <select id="gen-expiry-${key}" class="w-full bg-darker border border-borderDark rounded-xl px-2 py-2 text-xs text-white font-mono focus:border-gold">
                    <option value="5">5 Menit</option>
                    <option value="15" selected>15 Menit</option>
                    <option value="30">30 Menit</option>
                    <option value="60">60 Menit</option>
                  </select>
                </div>
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-muted mb-1">Ref ID Toko (Auto-Generated)</label>
                <input type="text" id="gen-ref-${key}" value="${meta.refPrefix}-${isSandbox ? 'SBX' : 'LIVE'}-${Date.now().toString().slice(-4)}" required
                       class="w-full bg-darker border border-borderDark rounded-xl px-3 py-2 text-xs text-white font-mono focus:border-gold">
              </div>
              <button type="submit" class="w-full bg-cardHover hover:bg-borderDark border border-borderDark text-white font-bold py-2.5 px-4 rounded-xl text-xs transition shadow flex items-center justify-center gap-2">
                <span>⚡</span>
                <span>Buat Invoice ${isSandbox ? 'Sandbox' : 'Live'}</span>
              </button>
            </form>

            <!-- Result Invoice Card (Hidden by default) -->
            <div id="gen-res-${key}" class="hidden p-4 rounded-xl bg-darker border border-borderDark space-y-3 animate-fade">
              <div class="flex items-center justify-between text-xs">
                <div>
                  <span class="text-muted block text-[10px]">TOTAL DITAGIH (+KODE UNIK)</span>
                  <div class="flex items-center gap-2">
                    <b id="gen-total-${key}" class="text-base font-mono text-white">-</b>
                    <button id="gen-copy-amt-${key}" onclick="copyActiveInvoiceAmount('${key}')" class="text-muted hover:text-white text-xs" title="Salin Nominal">📋</button>
                  </div>
                </div>
                <div class="text-right">
                  <span class="text-muted block text-[10px]">STATUS</span>
                  <div class="flex items-center gap-1.5 justify-end">
                    <span id="gen-status-${key}" class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-yellow-950 text-yellow-400 border border-yellow-800">PENDING</span>
                  </div>
                </div>
              </div>

              <!-- Countdown Timer -->
              <div class="flex items-center justify-between bg-card px-3 py-1.5 rounded-lg border border-borderDark text-xs">
                <span id="gen-timer-${key}" class="font-mono text-muted text-[11px]">⏱️ Menunggu tagihan dibuat...</span>
                <span class="text-[10px] text-muted font-mono">Auto-Release on Expire</span>
              </div>

              <!-- Visual QR Code Preview -->
              <div class="flex flex-col items-center justify-center p-3 bg-white rounded-xl">
                <img id="gen-qr-${key}" src="" alt="QR Tagihan" class="w-44 h-44 object-contain">
                <div class="flex items-center gap-1.5 mt-1.5">
                  <span class="text-[10px] text-slate-800 font-mono font-semibold" id="gen-orderid-${key}">-</span>
                  <button onclick="copyActiveInvoiceID('${key}')" class="text-slate-600 hover:text-black text-xs" title="Salin Order ID">📋</button>
                </div>
              </div>

              <!-- Action Buttons -->
              ${isSandbox
                ? `<button id="gen-pay-btn-${key}" onclick="handlePayActiveOrder('${channel}', '${key}')" class="w-full bg-success hover:bg-emerald-600 text-black font-bold py-2.5 px-4 rounded-xl text-xs transition shadow flex items-center justify-center gap-1.5">
                     <span>✅</span>
                     <span>1-Klik Bayar Lunas (Trigger Webhook Toko)</span>
                   </button>`
                : `<div class="grid grid-cols-2 gap-2">
                     <button id="gen-check-btn-${key}" onclick="handleCheckActiveOrder('${channel}', '${key}')" class="bg-cardHover hover:bg-borderDark text-white font-semibold py-2 px-3 rounded-xl text-xs border border-borderDark transition">
                       🔍 Cek Mutasi
                     </button>
                     <button id="gen-cancel-btn-${key}" onclick="handleCancelActiveOrder('${channel}', '${key}')" class="bg-red-950/40 hover:bg-red-900/60 text-red-400 font-semibold py-2 px-3 rounded-xl text-xs border border-red-900/60 transition">
                       ✕ Batalkan
                     </button>
                   </div>`}
            </div>
          </div>

          <!-- KUADRAN 2: Docs Siap Tempel (In-Page Copy-Paste Code) -->
          <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-4">
            <div class="flex justify-between items-center">
              <div>
                <h4 class="text-sm font-bold text-white flex items-center gap-2">
                  <span>📖</span> Docs Siap Tempel (Plug & Play)
                </h4>
                <p class="text-xs text-muted">Salin langsung snippet kode integrasi ke backend toko Anda.</p>
              </div>
              <button onclick="copyDocSnippet('${key}')" class="text-xs bg-darker hover:bg-cardHover border border-borderDark px-3 py-1.5 rounded-xl text-white transition flex items-center gap-1 shadow">
                <span>📋</span>
                <span>Salin Snippet</span>
              </button>
            </div>

            <!-- Doc Tabs -->
            <div class="flex flex-wrap border-b border-borderDark pb-2 gap-1 text-xs">
              <button onclick="switchDocTab('${key}', 'curl')" id="doc-btn-${key}-curl" class="doc-tab-btn active">cURL</button>
              <button onclick="switchDocTab('${key}', 'req')" id="doc-btn-${key}-req" class="doc-tab-btn">Request JSON</button>
              <button onclick="switchDocTab('${key}', 'res')" id="doc-btn-${key}-res" class="doc-tab-btn">Response JSON</button>
              <button onclick="switchDocTab('${key}', 'wh')" id="doc-btn-${key}-wh" class="doc-tab-btn">Webhook Callback</button>
              <button onclick="switchDocTab('${key}', 'php')" id="doc-btn-${key}-php" class="doc-tab-btn">PHP (HMAC)</button>
              <button onclick="switchDocTab('${key}', 'node')" id="doc-btn-${key}-node" class="doc-tab-btn">Node.js (HMAC)</button>
            </div>

            <!-- Code Pre Block -->
            <pre class="bg-darker border border-borderDark rounded-xl p-3 text-[11px] text-slate-300 font-mono overflow-x-auto max-h-[360px] select-all leading-relaxed"><code id="doc-pre-${key}">Memuat snippet...</code></pre>

            <div class="p-3 rounded-xl bg-cardHover/40 border border-borderDark text-[11px] text-muted space-y-1">
              <div class="flex items-center gap-1.5 text-white font-semibold">
                <span>💡</span>
                <span>Jaminan Plug & Play</span>
              </div>
              <p>Format request, response, dan payload webhook antara <b>Sandbox</b> dan <b>Live</b> 100% identik. Anda cukup mengganti URL endpoint dan API Key tanpa mengubah logika backend toko.</p>
            </div>
          </div>

        </div>

        <!-- ======================================================== -->
        <!-- ROW 2: KUADRAN 4 (OPERASIONAL / CONFIG / EDUKASI SANDBOX) -->
        <!-- ======================================================== -->
        ${isSandbox ? `
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Panel Panduan & Checklist Sandbox -->
            <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
              <h4 class="text-xs font-bold text-white flex items-center gap-2">
                <span class="text-yellow-400">🧪</span> Panduan Alur Pengujian Sandbox
              </h4>
              <div class="space-y-2 text-xs text-muted leading-relaxed">
                <div class="flex items-start gap-2.5 p-2 rounded-xl bg-darker border border-borderDark">
                  <span class="w-5 h-5 rounded-full bg-yellow-950 text-yellow-400 font-bold flex items-center justify-center shrink-0 text-[10px]">1</span>
                  <div>
                    <b class="text-white">Buat Tagihan Uji Coba</b>
                    <p class="text-[11px]">Gunakan form di Kuadran 1 atau cURL di Kuadran 2 dengan API Key Sandbox (<code class="text-yellow-400 font-mono">ak_test_...</code>).</p>
                  </div>
                </div>
                <div class="flex items-start gap-2.5 p-2 rounded-xl bg-darker border border-borderDark">
                  <span class="w-5 h-5 rounded-full bg-yellow-950 text-yellow-400 font-bold flex items-center justify-center shrink-0 text-[10px]">2</span>
                  <div>
                    <b class="text-white">Simulasikan Pembayaran Lunas</b>
                    <p class="text-[11px]">Klik tombol hijau <b class="text-success">"1-Klik Bayar Lunas"</b> di Kuadran 1 atau tombol petir di baris transaksi.</p>
                  </div>
                </div>
                <div class="flex items-start gap-2.5 p-2 rounded-xl bg-darker border border-borderDark">
                  <span class="w-5 h-5 rounded-full bg-yellow-950 text-yellow-400 font-bold flex items-center justify-center shrink-0 text-[10px]">3</span>
                  <div>
                    <b class="text-white">Verifikasi Webhook Toko</b>
                    <p class="text-[11px]">Sistem seketika mengirim POST callback ber-signature HMAC-SHA256 ke webhook URL toko Anda untuk validasi pesanan otomatis.</p>
                  </div>
                </div>
                <div class="flex items-start gap-2.5 p-2 rounded-xl bg-darker border border-borderDark">
                  <span class="w-5 h-5 rounded-full bg-yellow-950 text-yellow-400 font-bold flex items-center justify-center shrink-0 text-[10px]">4</span>
                  <div>
                    <b class="text-white">Transisi ke Live (Plug & Play)</b>
                    <p class="text-[11px]">Ganti endpoint dari <code class="text-yellow-400 font-mono">/sandbox/orders</code> ke <code class="text-emerald-400 font-mono">/orders</code> dan gunakan Live Key (<code class="text-emerald-400 font-mono">ak_live_...</code>).</p>
                  </div>
                </div>
              </div>
            </div>

            <!-- Panel Arsitektur & Keamanan Sandbox -->
            <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
              <h4 class="text-xs font-bold text-white flex items-center gap-2">
                <span class="text-yellow-400">🛡️</span> Arsitektur Fisik Isolasi Database
              </h4>
              <div class="space-y-3 text-xs text-muted leading-relaxed">
                <p>Sistem ini menerapkan isolasi database fisik murni antara lingkungan pengujian dan produksi:</p>
                <div class="p-3 rounded-xl bg-darker border border-borderDark space-y-2 font-mono text-[11px]">
                  <div class="flex justify-between items-center">
                    <span class="text-muted">Database Sandbox:</span>
                    <span class="text-yellow-400 font-semibold">data/sandbox/payment.db</span>
                  </div>
                  <div class="flex justify-between items-center">
                    <span class="text-muted">Prefix API Key:</span>
                    <span class="text-yellow-400 font-semibold">ak_test_...</span>
                  </div>
                  <div class="flex justify-between items-center">
                    <span class="text-muted">Prefix Order ID:</span>
                    <span class="text-yellow-400 font-semibold">sbx_${channel}_...</span>
                  </div>
                  <div class="flex justify-between items-center">
                    <span class="text-muted">Pergerakan Saldo:</span>
                    <span class="text-emerald-400 font-semibold">Zero Financial Risk</span>
                  </div>
                </div>
                <p class="text-[11px]">Semua data pesanan simulasi tidak akan pernah bercampur dengan database live. Tim developer Anda bebas menguji coba kapan saja tanpa memengaruhi pembukuan omset riil.</p>
              </div>
            </div>
          </div>
        ` : `
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Panel Kiri: Form Runtime CRUD -->
            ${runtimeConfigHTML}

            <!-- Panel Kanan: Diagnostik / Edukasi / CEX Fee Rescue -->
            ${channel === 'qris' ? `
              <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
                <div class="flex justify-between items-center">
                  <h4 class="text-xs font-bold text-white flex items-center gap-2">
                    <span class="text-shopee">💡</span> Panduan ShopeePay & Alokasi Kode Unik
                  </h4>
                  <span class="text-[10px] bg-shopee/20 text-shopee border border-shopee/40 px-2 py-0.5 rounded">IDR 1 - 999</span>
                </div>
                <div class="space-y-2.5 text-xs text-muted leading-relaxed">
                  <p><b class="text-white">Mengapa ada Kode Unik?</b> Sistem menambahkan kode acak 1 - 999 IDR pada setiap tagihan dinamis agar mutasi masuk dapat dicocokkan seketika tanpa memerlukan bukti transfer manual.</p>
                  <div class="p-3 rounded-xl bg-darker border border-borderDark space-y-1.5 text-[11px]">
                    <div class="text-white font-semibold flex items-center gap-1.5">
                      <span>🔄</span> Siklus Hidup Kode Unik:
                    </div>
                    <ul class="list-disc list-inside space-y-1 text-slate-300">
                      <li>Dialokasikan secara thread-safe saat invoice dibuat.</li>
                      <li>Otomatis dibebaskan kembali saat transaksi <b>LUNAS</b>.</li>
                      <li>Otomatis dibebaskan saat waktu berlaku (15 menit) <b>KEDALUWARSA</b>.</li>
                    </ul>
                  </div>
                  <p class="text-[11px]"><b class="text-white">Sesi ShopeePay:</b> Token sesi aktif selama 24 jam. Pasang ekstensi Tampermonkey <code>paymentg-shopee-sync.user.js</code> pada browser merchant Anda untuk auto-refresh sesi secara berkala tanpa repot login ulang.</p>
                </div>
              </div>
            ` : channel === 'binance' ? `
              <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
                <div class="flex justify-between items-center">
                  <h4 class="text-xs font-bold text-white flex items-center gap-2">
                    <span class="text-gold">💡</span> Keunggulan Binance Pay SAPI & Micro-Code
                  </h4>
                  <span class="text-[10px] bg-yellow-950 text-yellow-400 border border-yellow-800 px-2 py-0.5 rounded">Zero Fee</span>
                </div>
                <div class="space-y-2.5 text-xs text-muted leading-relaxed">
                  <p><b class="text-white">Internal Zero-Fee Transfer:</b> Pembeli membayar langsung dari aplikasi Binance ke Binance Pay ID Anda tanpa potongan gas fee blockchain sama sekali.</p>
                  <div class="p-3 rounded-xl bg-darker border border-borderDark space-y-1.5 text-[11px]">
                    <div class="text-white font-semibold flex items-center gap-1.5">
                      <span>⚡</span> Mekanisme Micro-Code (Milli-Units):
                    </div>
                    <ul class="list-disc list-inside space-y-1 text-slate-300">
                      <li>Nominal mikro 0.001 - 0.099 USDT ditambahkan sebagai penanda invoice.</li>
                      <li>Server membaca mutasi SAPI setiap 10 detik dengan auto clock-drift correction.</li>
                      <li>Setelah lunas, slot nominal mikro langsung dilepas untuk pembeli berikutnya.</li>
                    </ul>
                  </div>
                  <p class="text-[11px]"><b class="text-white">Keamanan API:</b> Pastikan API Key Binance hanya memiliki izin <i>Read-Only</i> dan <i>Pay History Query</i>. Jangan aktifkan izin penarikan (Withdrawal) pada Binance API Management.</p>
                </div>
              </div>
            ` : `
              <!-- USDT BEP-20 Live: CEX Fee Rescue -->
              <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-3">
                <div class="flex justify-between items-center">
                  <div>
                    <h4 class="text-xs font-bold text-white flex items-center gap-2">
                      <span class="text-usdt">🛟</span> CEX Fee Rescue (Penyelamatan Mutasi Gantung)
                    </h4>
                    <p class="text-[11px] text-muted">Mutasi penarikan bursa (Indodax, Bybit, Binance) yang terpotong fee penarikan.</p>
                  </div>
                  <button onclick="loadUnmatchedTransactions()" class="text-xs bg-darker hover:bg-cardHover border border-borderDark px-2.5 py-1 rounded-lg text-white transition">🔄 Muat Ulang</button>
                </div>
                <div class="overflow-x-auto max-h-[220px]">
                  <table class="w-full text-left text-xs font-mono">
                    <thead class="bg-darker text-muted uppercase tracking-wider text-[10px] border-b border-borderDark sticky top-0">
                      <tr>
                        <th class="p-2">Waktu</th>
                        <th class="p-2">TX Hash</th>
                        <th class="p-2">Masuk</th>
                        <th class="p-2">Aksi</th>
                      </tr>
                    </thead>
                    <tbody id="unmatchedTbody" class="divide-y divide-borderDark/60">
                      <tr><td colspan="4" class="p-3 text-center text-muted">Tidak ada mutasi gantung saat ini.</td></tr>
                    </tbody>
                  </table>
                </div>
                <p class="text-[10px] text-muted">Jika pembeli lupa menambahkan fee penarikan bursa sehingga nominal masuk kurang dari tagihan, temukan mutasinya di sini dan klik "Pasangkan" ke Order ID pembeli.</p>
              </div>
            `}
          </div>
        `}

        <!-- ======================================================== -->
        <!-- ROW 3: KUADRAN 3 (RIWAYAT TRANSAKSI CHANNEL DEDICATED)   -->
        <!-- ======================================================== -->
        <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-4">
          <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
            <div>
              <h4 class="text-sm font-bold text-white flex items-center gap-2">
                <span>📜</span> Riwayat Transaksi ${meta.title} (${isSandbox ? 'Sandbox' : 'Live'})
              </h4>
              <p class="text-xs text-muted">Khusus transaksi channel ini yang tercatat di database ${isSandbox ? 'data/sandbox/payment.db' : 'data/payment.db'}.</p>
            </div>

            <div class="flex items-center gap-2">
              <select id="filter-${key}-status" onchange="loadChannelOrders('${channel}', ${isSandbox})" class="bg-darker border border-borderDark text-xs rounded-xl px-3 py-1.5 text-white">
                <option value="">Semua Status</option>
                <option value="PENDING">PENDING</option>
                <option value="PAID">PAID</option>
                <option value="EXPIRED">EXPIRED</option>
              </select>
              <button onclick="loadChannelOrders('${channel}', ${isSandbox})" class="text-xs bg-darker hover:bg-cardHover border border-borderDark px-3 py-1.5 rounded-xl text-white transition">
                🔄 Muat Ulang
              </button>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs font-mono">
              <thead class="bg-darker text-muted uppercase tracking-wider text-[11px] border-b border-borderDark">
                <tr>
                  <th class="p-3">Waktu</th>
                  <th class="p-3">Order ID</th>
                  <th class="p-3">Ref ID Toko</th>
                  <th class="p-3">Total Nominal</th>
                  <th class="p-3">Status</th>
                  <th class="p-3">TX Info / Hash</th>
                  <th class="p-3">Aksi</th>
                </tr>
              </thead>
              <tbody id="${key}-orders-tbody" class="divide-y divide-borderDark/60">
                <tr><td colspan="7" class="p-4 text-center text-muted">Memuat data transaksi...</td></tr>
              </tbody>
            </table>
          </div>
        </div>

      </div>
    `;
  }).join('');
}

// =========================================================================
// 3. Navigation & Tab Switching
// =========================================================================
function switchTab(tabName) {
  state.activeTab = tabName;

  // Update Desktop nav buttons
  document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.classList.remove('text-white', 'bg-borderDark/60');
    btn.classList.add('text-muted');
  });

  const activeBtn = document.getElementById(`nav-${tabName}`);
  if (activeBtn) {
    activeBtn.classList.remove('text-muted');
    activeBtn.classList.add('text-white', 'bg-borderDark/60');
  }

  // Update Mobile nav buttons
  document.querySelectorAll('.mob-nav-btn').forEach(btn => {
    btn.classList.remove('text-gold', 'text-white');
    btn.classList.add('text-muted');
  });

  const mobBtn = document.getElementById(`mob-nav-${tabName}`);
  if (mobBtn) {
    mobBtn.classList.remove('text-muted');
    mobBtn.classList.add('text-gold');
  }

  // Update Tab Views visibility
  document.querySelectorAll('.tab-view').forEach(view => {
    view.classList.add('hidden');
  });

  const targetView = document.getElementById(`view-${tabName}`);
  if (targetView) {
    targetView.classList.remove('hidden');
  }

  // Load specific tab data
  if (tabName === 'overview') {
    loadOverviewStats();
    loadOverviewOrders();
  } else if (tabName === 'apps') {
    loadApps();
  } else if (tabName === 'security') {
    loadConfig();
  } else {
    // One of the 6 channel views: format is {channel}-{live|sandbox}
    const parts = tabName.split('-');
    if (parts.length === 2) {
      const channel = parts[0];
      const isSandbox = parts[1] === 'sandbox';
      updateChannelCredentials(tabName, isSandbox);
      renderDocSnippet(tabName, channel, isSandbox);
      loadChannelOrders(channel, isSandbox);
      if (!isSandbox && channel === 'usdt') {
        loadUnmatchedTransactions();
      }
    }
  }
}

function toggleMobileMenuDrawer() {
  const drawer = document.getElementById('mobileMenuDrawer');
  if (drawer) {
    drawer.classList.toggle('hidden');
  }
}

function toggleAdminKeyVisibility() {
  const input = document.getElementById('secAdminKeyInput');
  if (input) {
    input.type = input.type === 'password' ? 'text' : 'password';
  }
}

function setOverviewMode(isSandbox) {
  state.overviewIsSandbox = isSandbox;
  const liveBtn = document.getElementById('overviewModeLive');
  const sbxBtn = document.getElementById('overviewModeSandbox');
  const labelMode = document.getElementById('overviewTableModeLabel');

  const labelQ = document.getElementById('labelQRISVolume');
  const labelB = document.getElementById('labelBinanceVolume');
  const labelU = document.getElementById('labelUSDTVolume');

  if (isSandbox) {
    liveBtn.className = 'px-4 py-1.5 rounded-lg text-xs font-semibold text-muted transition hover:text-white';
    sbxBtn.className = 'px-4 py-1.5 rounded-lg text-xs font-bold transition text-black bg-yellow-400 shadow';
    if (labelMode) {
      labelMode.textContent = 'Mode Sandbox (Uji Coba)';
      labelMode.className = 'text-yellow-400';
    }
    if (labelQ) labelQ.textContent = 'VOLUME TESTING QRIS';
    if (labelB) labelB.textContent = 'VOLUME TESTING BINANCE';
    if (labelU) labelU.textContent = 'VOLUME TESTING USDT';
  } else {
    liveBtn.className = 'px-4 py-1.5 rounded-lg text-xs font-bold transition text-black bg-emerald-400 shadow';
    sbxBtn.className = 'px-4 py-1.5 rounded-lg text-xs font-semibold text-muted transition hover:text-white';
    if (labelMode) {
      labelMode.textContent = 'Mode Live (Produksi)';
      labelMode.className = 'text-emerald-400';
    }
    if (labelQ) labelQ.textContent = 'OMSET SHOPEEPAY QRIS';
    if (labelB) labelB.textContent = 'OMSET BINANCE PAY';
    if (labelU) labelU.textContent = 'OMSET USDT BEP-20';
  }

  loadOverviewStats();
  loadOverviewOrders();
}

function toggleAutoRefresh(enabled) {
  state.autoRefresh = enabled;
  if (enabled) {
    startAutoRefresh();
    showToast('Auto-Sync 5s diaktifkan', 'info');
  } else {
    clearInterval(state.refreshTimer);
    showToast('Auto-Sync dijeda', 'info');
  }
}

function startAutoRefresh() {
  clearInterval(state.refreshTimer);
  if (!state.autoRefresh) return;

  state.refreshTimer = setInterval(() => {
    if (!state.adminKey) return;

    if (state.activeTab === 'overview') {
      loadOverviewStats(true);
      loadOverviewOrders(true);
    } else if (state.activeTab !== 'apps' && state.activeTab !== 'security') {
      const parts = state.activeTab.split('-');
      if (parts.length === 2) {
        loadChannelOrders(parts[0], parts[1] === 'sandbox', true);

        // Auto-poll active invoice in Kuadran 1 if pending
        const activeOrd = state.activeInvoices[state.activeTab];
        if (activeOrd && activeOrd.status === 'PENDING') {
          pollActiveInvoiceStatus(parts[0], parts[1] === 'sandbox', activeOrd.id, state.activeTab);
        }
      }
    }
  }, 5000);
}

async function pollActiveInvoiceStatus(channel, isSandbox, orderId, key) {
  try {
    const endpoint = `/${channel}/api/${isSandbox ? 'sandbox/' : ''}orders/${orderId}`;
    const res = await fetch(endpoint, {
      headers: { 'X-API-Key': isSandbox ? state.activeSandboxKey : state.activeLiveKey }
    });
    if (!res.ok) return;
    const body = await res.json();
    if (body.order && body.order.status === 'PAID') {
      state.activeInvoices[key] = body.order;
      const statusEl = document.getElementById(`gen-status-${key}`);
      if (statusEl) {
        statusEl.textContent = 'PAID (LUNAS)';
        statusEl.className = 'px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-emerald-950 text-emerald-400 border border-emerald-800';
      }
      const payBtn = document.getElementById(`gen-pay-btn-${key}`);
      if (payBtn) {
        payBtn.innerHTML = `<span>🎉</span><span>Tagihan Lunas & Webhook Terkirim!</span>`;
        payBtn.className = 'w-full bg-emerald-700 text-white font-bold py-2.5 px-4 rounded-xl text-xs transition cursor-default flex items-center justify-center gap-1.5';
      }
      showToast(`Tagihan ${orderId} telah terbayar lunas!`, 'success');
      loadChannelOrders(channel, isSandbox, true);
    }
  } catch (e) {
    // Silent polling
  }
}

// =========================================================================
// 4. In-Page Docs Siap Tempel Generator (Plug & Play)
// =========================================================================
function switchDocTab(key, docType) {
  state.docTabs[key] = docType;

  // Toggle button styling
  ['curl', 'req', 'res', 'wh', 'php', 'node'].forEach(t => {
    const btn = document.getElementById(`doc-btn-${key}-${t}`);
    if (btn) {
      if (t === docType) {
        btn.classList.add('active');
      } else {
        btn.classList.remove('active');
      }
    }
  });

  const parts = key.split('-');
  renderDocSnippet(key, parts[0], parts[1] === 'sandbox');
}

function renderDocSnippet(key, channel, isSandbox) {
  const preEl = document.getElementById(`doc-pre-${key}`);
  if (!preEl) return;

  const currentTab = state.docTabs[key] || 'curl';
  const origin = window.location.origin;
  const endpoint = isSandbox ? `${origin}/${channel}/api/sandbox/orders` : `${origin}/${channel}/api/orders`;
  const apiKey = isSandbox ? state.activeSandboxKey : state.activeLiveKey;
  const secretKey = state.activeWebhookSecret || 'whsec_demo_secret';
  const meta = CHANNEL_META[channel];

  let snippet = '';

  if (currentTab === 'curl') {
    snippet = `curl -X POST ${endpoint} \\
  -H "Content-Type: application/json" \\
  -H "X-API-Key: ${apiKey}" \\
  -d '{
    "reference_id": "INV-2026-001",
    "amount": ${meta.defaultAmount},
    "expiry_minutes": 15
  }'`;
  } else if (currentTab === 'req') {
    snippet = JSON.stringify({
      reference_id: "INV-2026-001",
      amount: meta.defaultAmount,
      expiry_minutes: 15,
      metadata: "user_id_123"
    }, null, 2);
  } else if (currentTab === 'res') {
    snippet = JSON.stringify({
      success: true,
      order: {
        id: isSandbox ? `sbx_${channel}_a1b2c3d4` : `ord_${channel}_a1b2c3d4`,
        channel: channel,
        reference_id: "INV-2026-001",
        currency: meta.currency,
        original_amount: meta.defaultAmount,
        unique_code: 42,
        total_amount: meta.currency === 'IDR' ? meta.defaultAmount + 42 : meta.defaultAmount + 0.042,
        total_amount_str: meta.currency === 'IDR' ? `${meta.defaultAmount + 42}` : `${meta.defaultAmount + 0.042}`,
        status: "PENDING",
        qr_url: `${origin}/${channel}/api/${isSandbox ? 'sandbox/' : ''}orders/ord_xxx/qr.png`,
        remaining_seconds: 900
      }
    }, null, 2);
  } else if (currentTab === 'wh') {
    snippet = JSON.stringify({
      event: "payment.success",
      is_sandbox: isSandbox,
      channel: channel,
      order_id: isSandbox ? `sbx_${channel}_a1b2c3d4` : `ord_${channel}_a1b2c3d4`,
      reference_id: "INV-2026-001",
      currency: meta.currency,
      original_amount: meta.defaultAmount,
      unique_code: 42,
      total_amount: meta.currency === 'IDR' ? meta.defaultAmount + 42 : meta.defaultAmount + 0.042,
      total_amount_str: meta.currency === 'IDR' ? `${meta.defaultAmount + 42}` : `${meta.defaultAmount + 0.042}`,
      paid_at: Date.now(),
      tx_id: isSandbox ? `SIM_${channel.toUpperCase()}_12345` : `TX_ONLINE_67890`,
      timestamp: Date.now()
    }, null, 2);
  } else if (currentTab === 'php') {
    snippet = `<?php
// Verifikasi Webhook Signature PaymentG (HMAC-SHA256)
$secret = '${secretKey}'; // Webhook Secret toko Anda dari tab Toko & API Keys
$signature = $_SERVER['HTTP_X_WEBHOOK_SIGNATURE'] ?? '';
$payload = file_get_contents('php://input');

$expected = hash_hmac('sha256', $payload, $secret);
if (!hash_equals($expected, $signature)) {
    http_response_code(401);
    exit(json_encode(['error' => 'Invalid signature']));
}

$data = json_decode($payload, true);
if ($data['event'] === 'payment.success') {
    $orderId     = $data['order_id'];
    $referenceId = $data['reference_id'];
    $totalAmount = $data['total_amount'];
    // Update status pesanan di database toko Anda menjadi PAID
}

http_response_code(200);
echo json_encode(['received' => true]);`;
  } else if (currentTab === 'node') {
    snippet = `// Verifikasi Webhook Signature PaymentG (Node.js / Express)
const crypto = require('crypto');
const express = require('express');
const app = express();

app.post('/webhook', express.raw({ type: 'application/json' }), (req, res) => {
  const secret = '${secretKey}'; // Webhook Secret toko Anda
  const signature = req.headers['x-webhook-signature'];
  const payload = req.body.toString('utf8');

  const expected = crypto.createHmac('sha256', secret).update(payload).digest('hex');
  if (signature !== expected) {
    return res.status(401).json({ error: 'Invalid signature' });
  }

  const data = JSON.parse(payload);
  if (data.event === 'payment.success') {
    console.log(\`Order \${data.order_id} lunas! Total: \${data.total_amount_str}\`);
    // Update status pesanan di database toko Anda menjadi PAID
  }

  res.status(200).json({ received: true });
});`;
  }

  preEl.textContent = snippet;
}

function copyDocSnippet(key) {
  const preEl = document.getElementById(`doc-pre-${key}`);
  if (preEl && preEl.textContent) {
    copyToClipboard(preEl.textContent);
  }
}

function updateChannelCredentials(key, isSandbox) {
  const keyValEl = document.getElementById(`key-val-${key}`);
  if (keyValEl) {
    keyValEl.textContent = isSandbox ? state.activeSandboxKey : state.activeLiveKey;
  }
}

function copyActiveApiKey(key, isSandbox) {
  const keyToCopy = isSandbox ? state.activeSandboxKey : state.activeLiveKey;
  copyToClipboard(keyToCopy);
}

// =========================================================================
// 5. Universal Invoice Generator & Action Handlers
// =========================================================================
async function handleCreateInvoice(channel, isSandbox, event) {
  event.preventDefault();
  const key = `${channel}-${isSandbox ? 'sandbox' : 'live'}`;
  const apiKey = isSandbox ? state.activeSandboxKey : state.activeLiveKey;

  const amountInput = document.getElementById(`gen-amount-${key}`);
  const expirySelect = document.getElementById(`gen-expiry-${key}`);
  const refInput = document.getElementById(`gen-ref-${key}`);
  const submitBtn = event.target.querySelector('button[type="submit"]');

  const amount = parseFloat(amountInput.value);
  const expiryMin = expirySelect ? (parseInt(expirySelect.value) || 15) : 15;
  const refId = refInput.value.trim();

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.dataset.origText = submitBtn.innerHTML;
    submitBtn.innerHTML = '<span>⏳</span><span>Membuat Tagihan...</span>';
  }

  try {
    const endpoint = `/${channel}/api/${isSandbox ? 'sandbox/' : ''}orders`;
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': apiKey
      },
      body: JSON.stringify({
        amount: amount,
        reference_id: refId,
        expiry_minutes: expiryMin
      })
    });

    const data = await res.json();
    if (!res.ok || !data.success) {
      showToast(data.error || 'Gagal membuat tagihan', 'error');
      return;
    }

    const ord = data.order;
    state.activeInvoices[key] = ord;

    // Render invoice visual
    const resCard = document.getElementById(`gen-res-${key}`);
    const totalEl = document.getElementById(`gen-total-${key}`);
    const statusEl = document.getElementById(`gen-status-${key}`);
    const qrEl = document.getElementById(`gen-qr-${key}`);
    const orderIdEl = document.getElementById(`gen-orderid-${key}`);

    if (totalEl) {
      totalEl.textContent = ord.currency === 'IDR' ? formatIDR(ord.total_amount) : `${ord.total_amount_str} USDT`;
    }
    if (statusEl) {
      statusEl.textContent = ord.status;
      statusEl.className = 'px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-yellow-950 text-yellow-400 border border-yellow-800';
    }
    if (qrEl) qrEl.src = ord.qr_url;
    if (orderIdEl) orderIdEl.textContent = `ID: ${ord.id}`;

    // Start countdown timer
    startInvoiceCountdown(key, ord.expires_at);

    if (isSandbox) {
      const payBtn = document.getElementById(`gen-pay-btn-${key}`);
      if (payBtn) {
        payBtn.disabled = false;
        payBtn.innerHTML = `<span>✅</span><span>1-Klik Bayar Lunas (Trigger Webhook Toko)</span>`;
        payBtn.className = 'w-full bg-success hover:bg-emerald-600 text-black font-bold py-2.5 px-4 rounded-xl text-xs transition shadow flex items-center justify-center gap-1.5';
      }
    }

    if (resCard) resCard.classList.remove('hidden');

    // Auto-update Ref ID Toko so repeated clicks do not create duplicate references
    const meta = CHANNEL_META[channel];
    refInput.value = `${meta.refPrefix}-${isSandbox ? 'SBX' : 'LIVE'}-${Date.now().toString().slice(-4)}`;

    showToast(`Invoice ${isSandbox ? 'Sandbox' : 'Live'} berhasil dibuat!`, 'success');
    loadChannelOrders(channel, isSandbox);
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = submitBtn.dataset.origText || `<span>⚡</span><span>Buat Invoice ${isSandbox ? 'Sandbox' : 'Live'}</span>`;
    }
  }
}

function startInvoiceCountdown(key, expiresAt) {
  if (state.countdownTimers[key]) {
    clearInterval(state.countdownTimers[key]);
  }

  function tick() {
    const remaining = Math.max(0, Math.floor((expiresAt - Date.now()) / 1000));
    const timerEl = document.getElementById(`gen-timer-${key}`);
    const statusEl = document.getElementById(`gen-status-${key}`);

    if (timerEl) {
      if (remaining > 0) {
        const m = Math.floor(remaining / 60);
        const s = remaining % 60;
        timerEl.textContent = `⏱️ Sisa Waktu: ${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
        timerEl.className = 'text-[11px] font-mono text-muted';
      } else {
        timerEl.textContent = '⏱️ Waktu Kedaluwarsa (Expired)';
        timerEl.className = 'text-[11px] font-mono text-danger font-semibold';
        if (statusEl && statusEl.textContent === 'PENDING') {
          statusEl.textContent = 'EXPIRED';
          statusEl.className = 'px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-red-950 text-red-400 border border-red-800';
        }
        clearInterval(state.countdownTimers[key]);
      }
    }
  }

  tick();
  state.countdownTimers[key] = setInterval(tick, 1000);
}

function copyActiveInvoiceAmount(key) {
  const ord = state.activeInvoices[key];
  if (ord) {
    const amt = ord.currency === 'IDR' ? ord.total_amount : ord.total_amount_str;
    copyToClipboard(amt.toString());
  }
}

function copyActiveInvoiceID(key) {
  const ord = state.activeInvoices[key];
  if (ord && ord.id) {
    copyToClipboard(ord.id);
  }
}

async function handlePayActiveOrder(channel, key) {
  const ord = state.activeInvoices[key];
  if (!ord || !ord.id) {
    showToast('Silakan buat invoice sandbox terlebih dahulu', 'error');
    return;
  }
  handlePayOrderRow(channel, true, ord.id, key);
}

async function handlePayOrderRow(channel, isSandbox, orderId, activeKey) {
  const payBtn = activeKey ? document.getElementById(`gen-pay-btn-${activeKey}`) : null;
  if (payBtn) {
    payBtn.disabled = true;
    payBtn.textContent = 'Memproses simulasi lunas...';
  }

  try {
    const res = await fetch(`/${channel}/api/sandbox/orders/${orderId}/pay`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': state.activeSandboxKey
      }
    });

    const data = await res.json();
    if (!res.ok || !data.success) {
      showToast(data.error || 'Simulasi gagal', 'error');
      if (payBtn) payBtn.disabled = false;
      return;
    }

    if (activeKey) {
      const statusEl = document.getElementById(`gen-status-${activeKey}`);
      if (statusEl) {
        statusEl.textContent = 'PAID (LUNAS)';
        statusEl.className = 'px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-emerald-950 text-emerald-400 border border-emerald-800';
      }
      if (payBtn) {
        payBtn.innerHTML = `<span>🎉</span><span>Berhasil Lunas & Webhook Terkirim!</span>`;
        payBtn.className = 'w-full bg-emerald-700 text-white font-bold py-2.5 px-4 rounded-xl text-xs transition cursor-default flex items-center justify-center gap-1.5';
      }
    }

    showToast(`Order ${orderId} lunas! Webhook sukses didispatch.`, 'success');
    loadChannelOrders(channel, isSandbox);
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
    if (payBtn) payBtn.disabled = false;
  }
}

async function handleCheckActiveOrder(channel, key) {
  const ord = state.activeInvoices[key];
  if (!ord || !ord.id) return;
  handleCheckOrderRow(channel, ord.id, key);
}

async function handleCheckOrderRow(channel, orderId, activeKey) {
  showToast(`Mengecek mutasi untuk ${orderId}...`, 'info');
  try {
    const res = await fetch(`/${channel}/api/orders/${orderId}/check`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': state.activeLiveKey
      }
    });
    const data = await res.json();
    if (data.is_paid) {
      showToast(`Tagihan ${orderId} sudah LUNAS!`, 'success');
      if (activeKey) {
        const statusEl = document.getElementById(`gen-status-${activeKey}`);
        if (statusEl) {
          statusEl.textContent = 'PAID (LUNAS)';
          statusEl.className = 'px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-emerald-950 text-emerald-400 border border-emerald-800';
        }
      }
    } else {
      showToast(`Status saat ini: ${data.status || 'PENDING'}`, 'info');
    }
    loadChannelOrders(channel, false);
  } catch (err) {
    showToast('Gagal cek mutasi: ' + err.message, 'error');
  }
}

async function handleCancelActiveOrder(channel, key) {
  const ord = state.activeInvoices[key];
  if (!ord || !ord.id) return;
  handleCancelOrderRow(channel, ord.id, key);
}

async function handleCancelOrderRow(channel, orderId, activeKey) {
  if (!confirm(`Batalkan invoice ${orderId}?`)) return;

  try {
    const res = await fetch(`/${channel}/api/orders/${orderId}/cancel`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': state.activeLiveKey
      }
    });
    const data = await res.json();
    if (res.ok && data.success) {
      showToast(`Invoice ${orderId} dibatalkan`, 'info');
      if (activeKey) {
        const statusEl = document.getElementById(`gen-status-${activeKey}`);
        if (statusEl) {
          statusEl.textContent = 'CANCELLED';
          statusEl.className = 'px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-red-950 text-red-400 border border-red-800';
        }
      }
      loadChannelOrders(channel, false);
    } else {
      showToast(data.error || 'Gagal membatalkan invoice', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  }
}

// =========================================================================
// 6. Dedicated Channel Order Table Loading (Live vs Sandbox Segregated)
// =========================================================================
async function loadChannelOrders(channel, isSandbox, silent = false) {
  const key = `${channel}-${isSandbox ? 'sandbox' : 'live'}`;
  const statusFilter = document.getElementById(`filter-${key}-status`)?.value || '';

  try {
    let url = `/api/admin/orders?channel=${channel}&sandbox=${isSandbox}&limit=30`;
    if (statusFilter) url += `&status=${statusFilter}`;

    const res = await fetch(url, { headers: getHeaders() });
    if (!res.ok) return;
    const body = await res.json();
    const orders = body.data || [];

    const tbody = document.getElementById(`${key}-orders-tbody`);
    if (!tbody) return;

    if (orders.length === 0) {
      tbody.innerHTML = `<tr><td colspan="7" class="p-4 text-center text-muted">Belum ada transaksi di database ${isSandbox ? 'data/sandbox/payment.db' : 'data/payment.db'}.</td></tr>`;
      return;
    }

    tbody.innerHTML = orders.map(ord => {
      const statusBadge = getStatusBadge(ord.status);
      const amountStr = ord.currency === 'IDR' ? formatIDR(ord.total_amount) : `${ord.total_amount_str} USDT`;
      const timeStr = formatDate(ord.created_at);
      const txInfo = ord.tx_hash || ord.tx_id || (ord.status === 'PENDING' ? 'Menunggu Pembayaran...' : '-');

      // Action column
      let actionHTML = '-';
      if (ord.status === 'PENDING') {
        if (isSandbox) {
          actionHTML = `
            <button onclick="handlePayOrderRow('${channel}', true, '${ord.id}')" class="bg-success hover:bg-emerald-600 text-black px-2.5 py-1 rounded-lg text-[10px] font-bold shadow transition flex items-center gap-1">
              <span>⚡</span><span>1-Klik Bayar</span>
            </button>
          `;
        } else {
          actionHTML = `
            <div class="flex items-center gap-1">
              <button onclick="handleCheckOrderRow('${channel}', '${ord.id}')" class="bg-cardHover hover:bg-borderDark text-white px-2 py-0.5 rounded text-[10px] border border-borderDark transition" title="Cek Mutasi">
                🔍
              </button>
              <button onclick="handleCancelOrderRow('${channel}', '${ord.id}')" class="bg-red-950/40 hover:bg-red-900/60 text-red-400 px-2 py-0.5 rounded text-[10px] border border-red-900/60 transition" title="Batalkan">
                ✕
              </button>
            </div>
          `;
        }
      }

      return `
        <tr class="hover:bg-cardHover/50 transition">
          <td class="p-3 text-muted">${timeStr}</td>
          <td class="p-3 font-bold text-white">${ord.id}</td>
          <td class="p-3 text-muted">${ord.reference_id}</td>
          <td class="p-3 font-bold text-white">${amountStr}</td>
          <td class="p-3">${statusBadge}</td>
          <td class="p-3 text-muted text-[11px] truncate max-w-[140px]" title="${txInfo}">
            ${channel === 'usdt' && ord.tx_hash && ord.tx_hash.startsWith('0x')
              ? `<a href="https://bscscan.com/tx/${ord.tx_hash}" target="_blank" class="text-usdt hover:underline font-mono">${ord.tx_hash.slice(0, 10)}...</a>`
              : txInfo}
          </td>
          <td class="p-3">${actionHTML}</td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    if (!silent) console.error(`Failed to load ${key} orders:`, err);
  }
}

// =========================================================================
// 7. Overview Tab: Metrics (Live vs Sandbox) & Global Orders
// =========================================================================
async function loadOverviewStats(silent = false) {
  try {
    const isSandbox = state.overviewIsSandbox;
    const res = await fetch(`/api/admin/stats?sandbox=${isSandbox}`, { headers: getHeaders() });
    if (!res.ok) return;
    const data = await res.json();

    // QRIS Metrics
    const qris = data.qris || {};
    document.getElementById('metricQRISVolume').textContent = formatIDR(qris.total_volume || 0);
    document.getElementById('metricQRISPaid').textContent = qris.paid_count || 0;
    document.getElementById('metricQRISPending').textContent = qris.pending_count || 0;

    // Binance Metrics
    const bnb = data.binance || {};
    document.getElementById('metricBinanceVolume').textContent = (bnb.total_volume || 0).toFixed(3) + ' USDT';
    document.getElementById('metricBinancePaid').textContent = bnb.paid_count || 0;
    document.getElementById('metricBinancePending').textContent = bnb.pending_count || 0;

    // USDT Metrics
    const usdt = data.usdt || {};
    document.getElementById('metricUSDTVolume').textContent = (usdt.total_volume || 0).toFixed(3) + ' USDT';
    document.getElementById('metricUSDTPaid').textContent = usdt.paid_count || 0;
    document.getElementById('metricUSDTPending').textContent = usdt.pending_count || 0;

    // Health Pills
    updateEngineHealthBadges();
  } catch (err) {
    if (!silent) console.error('Failed to load overview stats:', err);
  }
}

async function updateEngineHealthBadges() {
  try {
    const res = await fetch('/api/health');
    if (!res.ok) return;
    const h = await res.json();

    // Header Uptime Display
    const uptimeEl = document.getElementById('headerUptime');
    if (uptimeEl && h.uptime) {
      uptimeEl.innerHTML = `
        <span class="w-2 h-2 rounded-full bg-success inline-block pulse-green"></span>
        <span>Online (${h.uptime})</span>
      `;
    }

    const shopeeEl = document.getElementById('overviewShopeeStatus');
    const binanceEl = document.getElementById('overviewBinanceStatus');
    const usdtEl = document.getElementById('overviewUSDTStatus');

    if (shopeeEl && h.engines?.qris) {
      const q = h.engines.qris;
      if (q.token_active) {
        shopeeEl.textContent = `Online (${q.ttl_hours}h)`;
        shopeeEl.className = 'font-bold text-success';
      } else if (q.is_configured) {
        shopeeEl.textContent = 'Token Expired';
        shopeeEl.className = 'font-bold text-danger';
      } else {
        shopeeEl.textContent = 'Belum Config';
        shopeeEl.className = 'font-bold text-yellow-500';
      }

      const sideBadge = document.getElementById('badgeShopeeToken');
      if (sideBadge) {
        sideBadge.textContent = q.token_active ? `${q.ttl_hours}h` : (q.is_configured ? 'EXP' : 'OFF');
        sideBadge.className = q.token_active
          ? 'text-[10px] px-1.5 py-0.5 rounded font-mono bg-emerald-950 text-emerald-400 border border-emerald-800'
          : 'text-[10px] px-1.5 py-0.5 rounded font-mono bg-red-950 text-red-400 border border-red-800';
      }
    }

    if (binanceEl && h.engines?.binance) {
      const b = h.engines.binance;
      binanceEl.textContent = b.is_configured ? 'Siap (SAPI Aktif)' : 'Perlu Config';
      binanceEl.className = b.is_configured ? 'font-bold text-gold' : 'font-bold text-yellow-500';
    }

    if (usdtEl && h.engines?.usdt) {
      const u = h.engines.usdt;
      usdtEl.textContent = u.rpc_connected ? `Blok #${u.latest_block}` : 'RPC Disconnected';
      usdtEl.className = u.rpc_connected ? 'font-bold text-usdt' : 'font-bold text-danger';
    }
  } catch (err) {
    console.error('Failed to update health badges:', err);
  }
}

async function loadOverviewOrders(silent = false) {
  const channel = document.getElementById('filterOverviewChannel')?.value || '';
  const status = document.getElementById('filterOverviewStatus')?.value || '';
  const isSandbox = state.overviewIsSandbox;

  try {
    let url = `/api/admin/orders?sandbox=${isSandbox}&limit=30`;
    if (channel) url += `&channel=${channel}`;
    if (status) url += `&status=${status}`;

    const res = await fetch(url, { headers: getHeaders() });
    if (!res.ok) return;
    const body = await res.json();
    const orders = body.data || [];

    const tbody = document.getElementById('overviewOrdersTbody');
    if (!tbody) return;

    if (orders.length === 0) {
      tbody.innerHTML = `<tr><td colspan="7" class="p-6 text-center text-muted">Belum ada transaksi di database ${isSandbox ? 'sandbox' : 'live'}.</td></tr>`;
      return;
    }

    tbody.innerHTML = orders.map(ord => {
      const channelBadge = getChannelBadge(ord.channel);
      const statusBadge = getStatusBadge(ord.status);
      const amountStr = ord.currency === 'IDR' ? formatIDR(ord.total_amount) : `${ord.total_amount_str} USDT`;
      const timeStr = formatDate(ord.created_at);
      const txInfo = ord.tx_hash || ord.tx_id || (ord.status === 'PENDING' ? 'Menunggu Pembayaran...' : '-');

      return `
        <tr class="hover:bg-cardHover/50 transition">
          <td class="p-3 text-muted">${timeStr}</td>
          <td class="p-3">${channelBadge}</td>
          <td class="p-3 font-bold text-white">${ord.id}</td>
          <td class="p-3 text-muted">${ord.reference_id}</td>
          <td class="p-3 font-bold text-white">${amountStr}</td>
          <td class="p-3">${statusBadge}</td>
          <td class="p-3 text-muted text-[11px] truncate max-w-[140px]" title="${txInfo}">
            ${ord.channel === 'usdt' && ord.tx_hash && ord.tx_hash.startsWith('0x')
              ? `<a href="https://bscscan.com/tx/${ord.tx_hash}" target="_blank" class="text-usdt hover:underline font-mono">${ord.tx_hash.slice(0, 10)}...</a>`
              : txInfo}
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    if (!silent) console.error('Failed to load overview orders:', err);
  }
}

// =========================================================================
// 8. Runtime Configuration Management (Direct SQLite CRUD)
// =========================================================================
async function loadConfig() {
  try {
    const res = await fetch('/api/admin/config', { headers: getHeaders() });
    if (!res.ok) return;
    const body = await res.json();
    const data = body.data || {};

    // QRIS
    if (data.qris) {
      const tokInput = document.getElementById('qrisTokenInput');
      const qrsInput = document.getElementById('qrisStringInput');
      if (tokInput) tokInput.value = data.qris.shopee_token || '';
      if (qrsInput) qrsInput.value = data.qris.qris_string || '';
    }

    // Binance
    if (data.binance) {
      const pId = document.getElementById('binancePayIDInput');
      const aNm = document.getElementById('binanceAccountNameInput');
      const aKey = document.getElementById('binanceAPIKeyInput');
      const sKey = document.getElementById('binanceSecretKeyInput');
      if (pId) pId.value = data.binance.pay_id || '';
      if (aNm) aNm.value = data.binance.account_name || '';
      if (aKey) aKey.value = data.binance.api_key || '';
      if (sKey) sKey.value = data.binance.secret_key || '';
    }

    // USDT
    if (data.usdt) {
      const wInput = document.getElementById('usdtWalletInput');
      const rpcInput = document.getElementById('usdtRPCInput');
      if (wInput) wInput.value = data.usdt.receiver_wallet_address || '';
      if (rpcInput) rpcInput.value = data.usdt.bsc_rpc_urls || '';
    }

    // Security
    if (data.security) {
      const pinInput = document.getElementById('secAdminPINInput');
      const keyInput = document.getElementById('secAdminKeyInput');
      if (pinInput) pinInput.value = data.security.admin_pin || '';
      if (keyInput) keyInput.value = data.security.admin_key || '';
    }
  } catch (err) {
    console.error('Failed to load config:', err);
  }
}

async function handleSaveQRISConfig(e) {
  e.preventDefault();
  const submitBtn = e.target.querySelector('button[type="submit"]');
  const token = document.getElementById('qrisTokenInput')?.value.trim() || '';
  const qrisStr = document.getElementById('qrisStringInput')?.value.trim() || '';

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Menyimpan...</span>';
  }

  try {
    const res = await fetch('/api/admin/config/qris', {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify({ shopee_token: token, qris_string: qrisStr })
    });
    const data = await res.json();
    if (res.ok && data.success) {
      showToast(data.message || 'Konfigurasi ShopeePay berhasil disimpan!', 'success');
      loadConfig();
      updateEngineHealthBadges();
    } else {
      showToast(data.error || 'Gagal menyimpan konfigurasi ShopeePay', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>💾</span><span>Simpan Konfigurasi ShopeePay</span>';
    }
  }
}

async function handleSaveBinanceConfig(e) {
  e.preventDefault();
  const submitBtn = e.target.querySelector('button[type="submit"]');
  const payload = {
    pay_id: document.getElementById('binancePayIDInput')?.value.trim() || '',
    account_name: document.getElementById('binanceAccountNameInput')?.value.trim() || '',
    api_key: document.getElementById('binanceAPIKeyInput')?.value.trim() || '',
    secret_key: document.getElementById('binanceSecretKeyInput')?.value.trim() || ''
  };

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Menyimpan...</span>';
  }

  try {
    const res = await fetch('/api/admin/config/binance', {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok && data.success) {
      showToast(data.message || 'Konfigurasi Binance Pay berhasil disimpan!', 'success');
      updateEngineHealthBadges();
    } else {
      showToast(data.error || 'Gagal menyimpan konfigurasi Binance', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>💾</span><span>Simpan Pengaturan Binance Pay</span>';
    }
  }
}

async function handleSaveUSDTConfig(e) {
  e.preventDefault();
  const submitBtn = e.target.querySelector('button[type="submit"]');
  const payload = {
    receiver_wallet_address: document.getElementById('usdtWalletInput')?.value.trim() || '',
    bsc_rpc_urls: document.getElementById('usdtRPCInput')?.value.trim() || ''
  };

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Menyimpan...</span>';
  }

  try {
    const res = await fetch('/api/admin/config/usdt', {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok && data.success) {
      showToast(data.message || 'Konfigurasi USDT BEP-20 berhasil disimpan!', 'success');
      updateEngineHealthBadges();
    } else {
      showToast(data.error || 'Gagal menyimpan konfigurasi USDT', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>💾</span><span>Simpan Alamat Wallet & RPC</span>';
    }
  }
}

async function handleSaveSecurityConfig(e) {
  e.preventDefault();
  const submitBtn = e.target.querySelector('button[type="submit"]');
  const payload = {
    admin_pin: document.getElementById('secAdminPINInput').value.trim(),
    admin_key: document.getElementById('secAdminKeyInput').value.trim()
  };

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Menyimpan...</span>';
  }

  try {
    const res = await fetch('/api/admin/config/security', {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok && data.success) {
      state.adminKey = payload.admin_key;
      sessionStorage.setItem('multipay_admin_key', payload.admin_key);
      showToast('Kunci keamanan berhasil diperbarui!', 'success');
    } else {
      showToast(data.error || 'Gagal menyimpan konfigurasi keamanan', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>💾</span><span>Simpan Perubahan Keamanan</span>';
    }
  }
}

async function handleTestBinance() {
  showToast('Menguji koneksi ke Binance SAPI...', 'info');
  try {
    const res = await fetch('/api/admin/config/binance/test', {
      method: 'POST',
      headers: getHeaders()
    });
    const data = await res.json();
    if (data.success) {
      showToast(`Binance SAPI OK! Server Clock Offset: ${data.clock_offset}`, 'success');
    } else {
      showToast(data.error || 'Koneksi ke Binance SAPI gagal.', 'error');
    }
  } catch (err) {
    showToast('Gagal menghubungi endpoint uji: ' + err.message, 'error');
  }
}

async function handleTestUSDT() {
  showToast('Menguji latensi multi-node RPC BSC...', 'info');
  try {
    const res = await fetch('/api/admin/config/usdt/test', {
      method: 'POST',
      headers: getHeaders()
    });
    const data = await res.json();
    if (data.success) {
      showToast(`BSC RPC Aktif! Blok #${data.latest_block} (${data.latency_ms} ms)`, 'success');
    } else {
      showToast(data.error || 'Koneksi BSC RPC gagal.', 'error');
    }
  } catch (err) {
    showToast('Gagal menghubungi endpoint uji: ' + err.message, 'error');
  }
}

// =========================================================================
// 9. CEX Fee Rescue (Unmatched Transactions)
// =========================================================================
async function loadUnmatchedTransactions() {
  try {
    const res = await fetch('/api/admin/unmatched?channel=usdt', {
      headers: getHeaders()
    });
    if (!res.ok) return;
    const body = await res.json();
    const list = body.data || [];

    const tbody = document.getElementById('unmatchedTbody');
    if (!tbody) return;

    if (list.length === 0) {
      tbody.innerHTML = `<tr><td colspan="4" class="p-3 text-center text-muted">Tidak ada mutasi gantung saat ini. Semua transaksi cocok!</td></tr>`;
      return;
    }

    tbody.innerHTML = list.map(item => {
      const isResolved = !!item.resolved_order_id;
      return `
        <tr class="hover:bg-cardHover/50 transition">
          <td class="p-2 text-muted">${formatDate(item.tx_time)}</td>
          <td class="p-2">
            <a href="https://bscscan.com/tx/${item.tx_hash}" target="_blank" class="text-usdt hover:underline font-mono">
              ${item.tx_hash.slice(0, 8)}...${item.tx_hash.slice(-6)}
            </a>
          </td>
          <td class="p-2 font-bold text-white">${item.amount_str} USDT</td>
          <td class="p-2">
            ${isResolved
              ? `<span class="px-1.5 py-0.5 rounded text-[9px] font-bold bg-emerald-950 text-emerald-400">PASANG: ${item.resolved_order_id}</span>`
              : `<button onclick="openResolveModal('${item.id}')" class="bg-usdt hover:bg-usdtHover text-white px-2 py-0.5 rounded text-[10px] font-bold shadow">
                   Pasangkan
                 </button>`}
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    console.error('Failed to load unmatched transactions:', err);
  }
}

function openResolveModal(unmatchedId) {
  document.getElementById('resolveUnmatchedID').value = unmatchedId;
  document.getElementById('resolveUnmatchedDisplay').value = unmatchedId;
  document.getElementById('resolveOrderID').value = '';
  document.getElementById('resolveModal').classList.remove('hidden');
}

function closeResolveModal() {
  document.getElementById('resolveModal').classList.add('hidden');
}

async function handleResolveUnmatched(e) {
  e.preventDefault();
  const unmatchedId = document.getElementById('resolveUnmatchedID').value;
  const orderId = document.getElementById('resolveOrderID').value.trim();

  try {
    const res = await fetch('/api/admin/unmatched/resolve', {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({
        unmatched_id: unmatchedId,
        order_id: orderId
      })
    });

    const data = await res.json();
    if (res.ok && data.success) {
      showToast('Mutasi berhasil dipasangkan ke order dan webhook dikirim!', 'success');
      closeResolveModal();
      loadUnmatchedTransactions();
      loadChannelOrders('usdt', false);
    } else {
      showToast(data.error || 'Gagal memasangkan mutasi', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  }
}

// =========================================================================
// 10. Merchant Apps & Store Selector
// =========================================================================
async function loadApps() {
  try {
    const res = await fetch('/api/admin/apps', { headers: getHeaders() });
    if (!res.ok) return;
    const body = await res.json();
    state.apps = body.data || [];

    const badge = document.getElementById('badgeAppsCount');
    if (badge) badge.textContent = state.apps.length;

    // Populate Header Store Selector
    const storeSelector = document.getElementById('headerStoreSelector');
    if (storeSelector && state.apps.length > 0) {
      storeSelector.innerHTML = state.apps.map(a => `
        <option value="${a.id}" ${state.selectedAppId === a.id ? 'selected' : ''}>${escapeHtml(a.name)}</option>
      `).join('');

      if (!state.selectedAppId && state.apps[0]) {
        handleSelectStore(state.apps[0].id);
      }
    }

    const container = document.getElementById('appsListContainer');
    if (!container) return;

    if (state.apps.length === 0) {
      container.innerHTML = `
        <div class="col-span-2 p-8 text-center bg-card border border-borderDark rounded-2xl text-muted">
          Belum ada toko terdaftar. Klik "Tambah Toko Baru" untuk membuat toko dan pasangan API Key Anda.
        </div>
      `;
      return;
    }

    container.innerHTML = state.apps.map(app => `
      <div class="bg-card border border-borderDark rounded-2xl p-5 shadow space-y-4">
        <div class="flex justify-between items-start">
          <div>
            <h4 class="font-bold text-white text-base">${escapeHtml(app.name)}</h4>
            <span class="text-[11px] font-mono text-muted">ID: ${app.id}</span>
          </div>
          <button onclick="handleDeleteApp('${app.id}')" class="text-xs text-red-400 hover:text-red-300 px-2 py-1 rounded bg-red-950/40 border border-red-900/60 transition">
            Hapus Toko
          </button>
        </div>

        <div class="space-y-2 text-xs">
          <!-- Live API Key -->
          <div>
            <div class="flex justify-between text-muted mb-1">
              <span class="text-success font-semibold">Live API Key (Produksi)</span>
              <button onclick="copyToClipboard('${app.live_api_key}')" class="text-muted hover:text-white">Salin</button>
            </div>
            <div class="bg-darker border border-borderDark rounded-xl p-2 font-mono text-slate-300 break-all select-all">
              ${app.live_api_key}
            </div>
          </div>

          <!-- Sandbox API Key -->
          <div>
            <div class="flex justify-between text-muted mb-1">
              <span class="text-yellow-400 font-semibold">Sandbox API Key (Uji Coba)</span>
              <button onclick="copyToClipboard('${app.sandbox_api_key}')" class="text-muted hover:text-white">Salin</button>
            </div>
            <div class="bg-darker border border-borderDark rounded-xl p-2 font-mono text-slate-300 break-all select-all">
              ${app.sandbox_api_key}
            </div>
          </div>

          <!-- Webhook Secret -->
          <div>
            <div class="flex justify-between text-muted mb-1">
              <span class="text-blue-400 font-semibold">Webhook Secret (HMAC-SHA256)</span>
              <button onclick="copyToClipboard('${app.webhook_secret}')" class="text-muted hover:text-white">Salin</button>
            </div>
            <div class="bg-darker border border-borderDark rounded-xl p-2 font-mono text-slate-300 break-all select-all">
              ${app.webhook_secret || 'Belum diatur'}
            </div>
          </div>

          <!-- Webhook Destination URL -->
          <div>
            <span class="text-muted font-semibold">Webhook Destination URL:</span>
            <div class="bg-darker border border-borderDark rounded-xl p-2 font-mono text-slate-300 truncate">
              ${escapeHtml(app.webhook_url || 'Belum diatur')}
            </div>
          </div>
        </div>
      </div>
    `).join('');
  } catch (err) {
    console.error('Failed to load apps:', err);
  }
}

function handleSelectStore(appId) {
  state.selectedAppId = appId;
  const app = state.apps.find(a => a.id === appId);
  if (app) {
    state.activeLiveKey = app.live_api_key;
    state.activeSandboxKey = app.sandbox_api_key;
    state.activeWebhookSecret = app.webhook_secret;

    // Refresh credentials bar and doc snippets on the active view
    if (state.activeTab !== 'overview' && state.activeTab !== 'apps' && state.activeTab !== 'security') {
      const parts = state.activeTab.split('-');
      if (parts.length === 2) {
        updateChannelCredentials(state.activeTab, parts[1] === 'sandbox');
        renderDocSnippet(state.activeTab, parts[0], parts[1] === 'sandbox');
      }
    }
    showToast(`Toko aktif: ${app.name}`, 'info');
  }
}

function openCreateAppModal() {
  document.getElementById('newAppName').value = '';
  document.getElementById('newAppWebhook').value = '';
  document.getElementById('createAppModal').classList.remove('hidden');
}

function closeCreateAppModal() {
  document.getElementById('createAppModal').classList.add('hidden');
}

async function handleCreateApp(e) {
  e.preventDefault();
  const submitBtn = e.target.querySelector('button[type="submit"]');
  const name = document.getElementById('newAppName').value.trim();
  const wh = document.getElementById('newAppWebhook').value.trim();

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Membuat Toko...</span>';
  }

  try {
    const res = await fetch('/api/admin/apps', {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ name: name, webhook_url: wh })
    });

    const data = await res.json();
    if (res.ok && data.success) {
      showToast('Toko baru dan pasangan API Key berhasil dibuat!', 'success');
      closeCreateAppModal();
      loadApps();
    } else {
      showToast(data.error || 'Gagal membuat toko', 'error');
    }
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>Buat Toko & Kunci API</span>';
    }
  }
}

async function handleDeleteApp(id) {
  if (!confirm('Yakin ingin menghapus toko ini? API Key live dan sandbox-nya akan langsung dinonaktifkan.')) {
    return;
  }

  try {
    const res = await fetch(`/api/admin/apps/${id}`, {
      method: 'DELETE',
      headers: getHeaders()
    });

    if (res.ok) {
      showToast('Toko berhasil dihapus.', 'info');
      loadApps();
    }
  } catch (err) {
    showToast('Gagal menghapus toko: ' + err.message, 'error');
  }
}

// =========================================================================
// 11. UI Helpers, Badges & Toast Notifications
// =========================================================================
function getStatusBadge(status) {
  if (status === 'PAID') {
    return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-emerald-950 text-emerald-400 border border-emerald-800">PAID</span>`;
  }
  if (status === 'PENDING') {
    return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-yellow-950 text-yellow-400 border border-yellow-800">PENDING</span>`;
  }
  if (status === 'EXPIRED') {
    return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-slate-800 text-slate-400 border border-slate-700">EXPIRED</span>`;
  }
  return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-red-950 text-red-400 border border-red-800">${escapeHtml(status)}</span>`;
}

function getChannelBadge(channel) {
  if (channel === 'qris') {
    return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-red-950/60 text-shopee border border-shopee/40">QRIS</span>`;
  }
  if (channel === 'binance') {
    return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-amber-950/60 text-gold border border-gold/40">BINANCE</span>`;
  }
  if (channel === 'usdt') {
    return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-emerald-950/60 text-usdt border border-usdt/40">USDT BEP20</span>`;
  }
  return `<span class="px-2 py-0.5 rounded font-mono font-bold text-[10px] bg-borderDark text-muted">${escapeHtml(channel)}</span>`;
}

function showToast(message, type = 'info') {
  const container = document.getElementById('toastContainer');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;

  const icon = type === 'success' ? '✅' : type === 'error' ? '❌' : type === 'warning' ? '⚠️' : 'ℹ️';
  toast.innerHTML = `<span>${icon}</span><span>${escapeHtml(message)}</span>`;

  container.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

function copyToClipboard(text) {
  if (!text) return;
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(() => {
      showToast('Disalin ke clipboard!', 'success');
    }).catch(() => fallbackCopy(text));
  } else {
    fallbackCopy(text);
  }
}

function fallbackCopy(text) {
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.top = '0';
    ta.style.left = '0';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const successful = document.execCommand('copy');
    document.body.removeChild(ta);
    if (successful) {
      showToast('Disalin ke clipboard!', 'success');
    } else {
      showToast('Gagal menyalin.', 'error');
    }
  } catch (err) {
    showToast('Gagal menyalin: ' + err.message, 'error');
  }
}

function formatIDR(amount) {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0
  }).format(amount);
}

function formatDate(ms) {
  if (!ms) return '-';
  const d = new Date(ms);
  return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ' ' +
         d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short' });
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/[&<>'"]/g, tag => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;'
  }[tag] || tag));
}
