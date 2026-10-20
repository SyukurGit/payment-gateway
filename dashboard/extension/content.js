// PaymentG Shopee Token Sync - Content Script (partner.shopee.co.id)

(function () {
  'use strict';

  let currentDetectedToken = null;

  // Ekstrak token dari localStorage / sessionStorage
  function scanStorage() {
    try {
      // 1. Scan localStorage
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        const val = localStorage.getItem(key);
        if (val && val.includes('"token":"B:')) {
          const match = val.match(/"token"\s*:\s*"(B:[^"]+)"/);
          if (match && match[1]) return match[1];
        }
      }
    } catch (e) {}

    try {
      // 2. Scan sessionStorage
      for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        const val = sessionStorage.getItem(key);
        if (val && val.includes('"token":"B:')) {
          const match = val.match(/"token"\s*:\s*"(B:[^"]+)"/);
          if (match && match[1]) return match[1];
        }
      }
    } catch (e) {}

    // 3. Scan document.cookie JWT
    try {
      const match = document.cookie.match(/(?:^|;\s*)(?:__shopee_partner_website_x_token_live|token)=([^;]+)/);
      if (match && match[1]) {
        const parts = match[1].split(".");
        if (parts.length >= 2) {
          let payload = parts[1];
          payload += "====".slice(0, (4 - payload.length % 4) % 4);
          const decoded = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
          const data = JSON.parse(decoded);
          if (data && data.token && String(data.token).startsWith("B:")) {
            return String(data.token).trim();
          }
        }
      }
    } catch (e) {}

    return null;
  }

  // Tampilkan badge di pojok layar
  function ensureBadge(token) {
    let badge = document.getElementById("paymentg-sync-badge");
    if (!badge) {
      badge = document.createElement("div");
      badge.id = "paymentg-sync-badge";
      badge.style.position = "fixed";
      badge.style.bottom = "16px";
      badge.style.right = "16px";
      badge.style.zIndex = "999999";
      badge.style.background = "#0f172a";
      badge.style.border = "1px solid #334155";
      badge.style.borderRadius = "8px";
      badge.style.padding = "10px 14px";
      badge.style.boxShadow = "0 8px 24px rgba(0,0,0,0.3)";
      badge.style.display = "flex";
      badge.style.alignItems = "center";
      badge.style.gap = "10px";
      badge.style.fontFamily = "system-ui, -apple-system, sans-serif";
      badge.style.fontSize = "12px";
      badge.style.color = "#f8fafc";
      badge.style.cursor = "default";
      document.body.appendChild(badge);
    }

    if (token) {
      badge.innerHTML = `
        <span style="display:inline-block; width:8px; height:8px; border-radius:50%; background:#22c55e;"></span>
        <span><b>PaymentG:</b> Sesi Shopee Terdeteksi</span>
        <button id="paymentg-quick-sync-btn" style="background:#3b82f6; border:none; color:white; font-size:11px; font-weight:600; padding:4px 8px; border-radius:4px; cursor:pointer;">⚡ Sync VPS</button>
      `;

      const btn = document.getElementById("paymentg-quick-sync-btn");
      if (btn) {
        btn.onclick = () => {
          btn.innerText = "Mengirim...";
          btn.disabled = true;
          chrome.runtime.sendMessage({ action: "SYNC_NOW", token: token }, (res) => {
            if (res && res.success) {
              btn.innerText = "✅ Berhasil!";
              btn.style.background = "#10b981";
            } else {
              btn.innerText = "❌ Gagal";
              btn.style.background = "#ef4444";
              alert("Gagal sync ke VPS: " + (res ? res.error : "Unknown error"));
            }
            setTimeout(() => {
              btn.innerText = "⚡ Sync VPS";
              btn.style.background = "#3b82f6";
              btn.disabled = false;
            }, 3000);
          });
        };
      }
    }
  }

  function check() {
    const token = scanStorage();
    if (token && token !== currentDetectedToken) {
      currentDetectedToken = token;
      console.log("[PaymentG] Token ditemukan di Shopee Partner!");
      chrome.runtime.sendMessage({ action: "REPORT_TOKEN_FROM_PAGE", token: token });
      ensureBadge(token);
    }
  }

  setTimeout(check, 1000);
  setInterval(check, 4000);
})();
