// ==UserScript==
// @name         PaymentG - ShopeePay Auto Sync Token ke VPS (v2.0)
// @namespace    https://paymentg.syukurapi.online/
// @version      2.0.0
// @description  Otomatis mendeteksi token ShopeePay dari partner.shopee.co.id (localStorage, Cookies, API Network) dan mengirimkannya ke PaymentG VPS
// @author       PaymentG
// @match        https://partner.shopee.co.id/*
// @match        https://partner.business.accounts.shopee.co.id/*
// @grant        GM_xmlhttpRequest
// @grant        unsafeWindow
// @run-at       document-start
// ==/UserScript==

(function () {
    'use strict';

    // Konfigurasi endpoint & admin key (diinjeksi otomatis oleh backend PaymentG atau fallback default)
    const VPS_ENDPOINT = "%%VPS_ENDPOINT%%".includes("%%") ? "https://paymentg.syukurapi.online/api/config/token" : "%%VPS_ENDPOINT%%";
    const VPS_ADMIN_KEY = "%%VPS_ADMIN_KEY%%".includes("%%") ? "pg_ZZio-WwakXZLozuUvjphxYAh_L9MnnazjPbhakBwKDE" : "%%VPS_ADMIN_KEY%%";

    let lastSyncedToken = sessionStorage.getItem("paymentg_synced_token") || "";

    function showToast(message, isError = false) {
        try {
            const toastId = "paymentg-sync-toast";
            let toast = document.getElementById(toastId);
            if (!toast && document.body) {
                toast = document.createElement("div");
                toast.id = toastId;
                toast.style.position = "fixed";
                toast.style.bottom = "24px";
                toast.style.right = "24px";
                toast.style.padding = "12px 20px";
                toast.style.borderRadius = "8px";
                toast.style.color = "#ffffff";
                toast.style.fontSize = "13px";
                toast.style.fontWeight = "600";
                toast.style.fontFamily = "system-ui, -apple-system, sans-serif";
                toast.style.boxShadow = "0 8px 24px rgba(0,0,0,0.3)";
                toast.style.zIndex = "9999999";
                toast.style.transition = "all 0.3s ease";
                document.body.appendChild(toast);
            }

            if (toast) {
                toast.style.backgroundColor = isError ? "#e53e3e" : "#059669";
                toast.innerText = message;
                toast.style.display = "block";
                toast.style.opacity = "1";

                setTimeout(() => {
                    if (toast) {
                        toast.style.opacity = "0";
                        setTimeout(() => { toast.style.display = "none"; }, 300);
                    }
                }, 5000);
            }
        } catch (e) {}
    }

    function syncToken(token) {
        if (!token || token === lastSyncedToken || !token.startsWith("B:")) return;

        console.log("[PaymentG] Token ShopeePay terdeteksi (" + token.substring(0, 25) + "...), mengirim ke VPS...");

        GM_xmlhttpRequest({
            method: "PUT",
            url: VPS_ENDPOINT,
            headers: {
                "Content-Type": "application/json",
                "X-Admin-Key": VPS_ADMIN_KEY
            },
            data: JSON.stringify({ token: token }),
            onload: function (res) {
                try {
                    const data = JSON.parse(res.responseText);
                    if (res.status === 200 && data && data.success) {
                        lastSyncedToken = token;
                        sessionStorage.setItem("paymentg_synced_token", token);
                        console.log("[PaymentG] ✅ Berhasil sinkronisasi token ke VPS!");
                        showToast("✅ PaymentG: Token ShopeePay berhasil divalidasi & aktif di VPS!", false);
                    } else {
                        console.error("[PaymentG] Gagal sync VPS:", data);
                        showToast("⚠️ PaymentG: " + (data.error || "Gagal sinkronisasi"), true);
                    }
                } catch (e) {
                    console.error("[PaymentG] Response parse error:", e);
                }
            },
            onerror: function (err) {
                console.error("[PaymentG] Gagal menghubungi VPS:", err);
                showToast("⚠️ PaymentG: Tidak dapat menghubungi server VPS", true);
            }
        });
    }

    // Helper: Ekstrak JWT token
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

    // Scanner: Deteksi token dari semua sumber yang mungkin
    function detectToken() {
        // 1. Scan localStorage
        try {
            for (let i = 0; i < localStorage.length; i++) {
                const key = localStorage.key(i);
                const val = localStorage.getItem(key);
                if (val && val.includes('"token":"B:')) {
                    const match = val.match(/"token"\s*:\s*"(B:[^"]+)"/);
                    if (match && match[1]) {
                        syncToken(match[1]);
                        return;
                    }
                }
            }
        } catch (e) {}

        // 2. Scan sessionStorage
        try {
            for (let i = 0; i < sessionStorage.length; i++) {
                const key = sessionStorage.key(i);
                const val = sessionStorage.getItem(key);
                if (val && val.includes('"token":"B:')) {
                    const match = val.match(/"token"\s*:\s*"(B:[^"]+)"/);
                    if (match && match[1]) {
                        syncToken(match[1]);
                        return;
                    }
                }
            }
        } catch (e) {}

        // 3. Scan document.cookie
        try {
            const cookies = document.cookie.split(";");
            for (let i = 0; i < cookies.length; i++) {
                const cookie = cookies[i].trim();
                if (cookie.includes("token")) {
                    const parts = cookie.split("=");
                    if (parts.length > 1) {
                        const val = parts.slice(1).join("=");
                        if (val.startsWith("B:")) {
                            syncToken(val);
                            return;
                        }
                        const jwtTok = extractTokenFromJwt(val);
                        if (jwtTok) {
                            syncToken(jwtTok);
                            return;
                        }
                    }
                }
            }
        } catch (e) {}

        // 4. Legacy window.injectData fallback
        try {
            const win = typeof unsafeWindow !== "undefined" ? unsafeWindow : window;
            const d = win.injectData || win['injectData'];
            if (d && d.User && d.User.token && String(d.User.token).startsWith("B:")) {
                syncToken(String(d.User.token));
                return;
            }
        } catch (e) {}
    }

    // 5. Network Request Hook (Menangkap payload get-transaction-list)
    try {
        const win = typeof unsafeWindow !== "undefined" ? unsafeWindow : window;
        const originalFetch = win.fetch;
        if (originalFetch) {
            win.fetch = async function (...args) {
                try {
                    const url = typeof args[0] === "string" ? args[0] : (args[0] && args[0].url ? args[0].url : "");
                    if (url.includes("partner-web") || url.includes("merchant")) {
                        const init = args[1];
                        if (init && init.body && typeof init.body === "string" && init.body.includes('"token":"B:')) {
                            const match = init.body.match(/"token"\s*:\s*"(B:[^"]+)"/);
                            if (match && match[1]) syncToken(match[1]);
                        }
                    }
                } catch (e) {}
                return originalFetch.apply(this, args);
            };
        }
    } catch (e) {}

    // Jalankan pemeriksaan berkala
    window.addEventListener("load", () => {
        detectToken();
        setInterval(detectToken, 3000);
    });

    setTimeout(detectToken, 1000);
    setInterval(detectToken, 4000);

    console.log("[PaymentG] ShopeePay Auto Sync Userscript v2.0 aktif.");
})();
