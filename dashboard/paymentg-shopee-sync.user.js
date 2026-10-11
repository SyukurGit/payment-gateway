// ==UserScript==
// @name         PaymentG - ShopeePay Auto Sync Token ke VPS
// @namespace    https://paymentg.syukurapi.online/
// @version      1.0.0
// @description  Otomatis mengambil token ShopeePay dari partner.shopee.co.id dan mengirimkannya ke PaymentG VPS
// @author       PaymentG
// @match        https://partner.shopee.co.id/*
// @match        https://partner.business.accounts.shopee.co.id/*
// @grant        GM_xmlhttpRequest
// @grant        GM_notification
// @run-at       document-idle
// ==/UserScript==

(function () {
    'use strict';

    const VPS_ENDPOINT = "https://paymentg.syukurapi.online/api/config/token";
    const VPS_ADMIN_KEY = "pg_ZZio-WwakXZLozuUvjphxYAh_L9MnnazjPbhakBwKDE";
    const LOCAL_ENDPOINT = "http://localhost:3200/api/config/token";
    const LOCAL_ADMIN_KEY = "adm_secret_paymentg_2026";

    let lastSyncedToken = "";

    function showToast(message, isError = false) {
        const toastId = "paymentg-sync-toast";
        let toast = document.getElementById(toastId);
        if (!toast) {
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
            toast.style.boxShadow = "0 8px 24px rgba(0,0,0,0.2)";
            toast.style.zIndex = "999999";
            toast.style.transition = "all 0.3s ease";
            document.body.appendChild(toast);
        }

        toast.style.backgroundColor = isError ? "#e53e3e" : "#2b6cb0";
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

    function syncToken(token) {
        if (!token || token === lastSyncedToken) return;

        console.log("[PaymentG] Token terdeteksi (" + token.substring(0, 20) + "...), sinkronisasi ke VPS...");

        // 1. Kirim ke VPS
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
                    if (data && data.success) {
                        lastSyncedToken = token;
                        sessionStorage.setItem("paymentg_synced_token", token);
                        console.log("[PaymentG] ✅ Berhasil sinkronisasi token ke VPS!");
                        showToast("✅ PaymentG: Token ShopeePay berhasil disinkronkan ke VPS!", false);
                    } else {
                        console.error("[PaymentG] Gagal sync VPS:", data);
                        showToast("⚠️ PaymentG: Gagal sinkronkan ke VPS (" + (data.error || "error") + ")", true);
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

        // 2. Kirim juga ke server lokal (jika ada)
        try {
            GM_xmlhttpRequest({
                method: "PUT",
                url: LOCAL_ENDPOINT,
                headers: {
                    "Content-Type": "application/json",
                    "X-Admin-Key": LOCAL_ADMIN_KEY
                },
                data: JSON.stringify({ token: token }),
                onload: function () {
                    console.log("[PaymentG] Lokal backend ikut diperbarui.");
                }
            });
        } catch (e) { }
    }

    function checkToken() {
        const d = window.injectData || window['injectData'];
        if (d && d.User && d.User.token) {
            syncToken(d.User.token);
        }
    }

    // Jalankan pemeriksaan saat script dimuat
    setTimeout(checkToken, 1500);

    // Periksa berkala tiap 5 detik (Shopee SPA)
    setInterval(checkToken, 5000);

    console.log("[PaymentG] ShopeePay Auto Sync Userscript aktif.");
})();
