// PaymentG Shopee Token Sync - Background Service Worker

const DEFAULT_CONFIG = {
  vpsUrl: "https://paymentg.syukurapi.online",
  adminKey: "pg_ZZio-WwakXZLozuUvjphxYAh_L9MnnazjPbhakBwKDE",
  autoSync: true,
  lastSyncedToken: "",
  lastSyncTime: ""
};

// Helper: Decode JWT payload
function extractTokenFromJwt(jwtStr) {
  try {
    const parts = jwtStr.split(".");
    if (parts.length >= 2) {
      let payload = parts[1];
      payload += "====".slice(0, (4 - payload.length % 4) % 4);
      const decoded = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
      const data = JSON.parse(decoded);
      if (data && data.token && String(data.token).startsWith("B:")) {
        return String(data.token).trim();
      }
    }
  } catch (e) {}
  return null;
}

// Cari token dari Cookies domain shopee.co.id
async function findTokenFromCookies() {
  try {
    const cookies = await chrome.cookies.getAll({ domain: "shopee.co.id" });
    for (const c of cookies) {
      const val = c.value || "";
      if (val.startsWith("B:")) {
        return val.trim();
      }
      if (c.name.toLowerCase().includes("token")) {
        const jwtToken = extractTokenFromJwt(val);
        if (jwtToken) return jwtToken;
      }
    }
  } catch (e) {
    console.error("[PaymentG] Gagal membaca cookies:", e);
  }
  return null;
}

// Kirim token ke VPS
async function syncTokenToVPS(token, customConfig = null) {
  if (!token) return { success: false, error: "Token kosong" };

  const stored = await chrome.storage.local.get(DEFAULT_CONFIG);
  const cfg = customConfig || stored;

  const endpoint = `${cfg.vpsUrl.replace(/\/$/, "")}/api/config/token`;

  try {
    const response = await fetch(endpoint, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "X-Admin-Key": cfg.adminKey
      },
      body: JSON.stringify({ token: token })
    });

    const data = await response.json();
    if (response.ok && data.success) {
      const nowStr = new Date().toLocaleTimeString();
      await chrome.storage.local.set({
        lastSyncedToken: token,
        lastSyncTime: nowStr
      });
      return { success: true, data: data.data || "Token berhasil diperbarui & divalidasi!" };
    } else {
      return { success: false, error: data.error || `HTTP ${response.status}` };
    }
  } catch (err) {
    return { success: false, error: `Gagal menghubungi VPS: ${err.message}` };
  }
}

// Listener pesan dari popup & content script
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === "GET_STATUS") {
    (async () => {
      const stored = await chrome.storage.local.get(DEFAULT_CONFIG);
      const cookieToken = await findTokenFromCookies();
      sendResponse({
        config: stored,
        detectedToken: cookieToken || stored.lastSyncedToken || ""
      });
    })();
    return true; // Asynchronous response
  }

  if (request.action === "SYNC_NOW") {
    (async () => {
      let token = request.token;
      if (!token) {
        token = await findTokenFromCookies();
      }
      if (!token) {
        sendResponse({ success: false, error: "Tidak ada token terdeteksi. Silakan login ke partner.shopee.co.id terlebih dahulu." });
        return;
      }
      const res = await syncTokenToVPS(token, request.config);
      sendResponse(res);
    })();
    return true;
  }

  if (request.action === "REPORT_TOKEN_FROM_PAGE") {
    (async () => {
      const stored = await chrome.storage.local.get(DEFAULT_CONFIG);
      if (stored.autoSync && request.token && request.token !== stored.lastSyncedToken) {
        console.log("[PaymentG] Auto-sync token terdeteksi dari Shopee Partner:", request.token.substring(0, 20));
        await syncTokenToVPS(request.token, stored);
      }
      sendResponse({ received: true });
    })();
    return true;
  }
});
