// Config base URL untuk Store API
// Mendukung Cloudflare Pages, Localhost, dan VPS Custom Domain
const API_BASE = (function() {
    // 1. Prioritas tertinggi: Window Override
    if (window.API_BASE_OVERRIDE) {
        return window.API_BASE_OVERRIDE.replace(/\/+$/, "");
    }

    // 2. Cek tag meta: <meta name="api-base" content="https://api.domainanda.com">
    const metaTag = document.querySelector('meta[name="api-base"]');
    if (metaTag && metaTag.content && !metaTag.content.startsWith("http://localhost")) {
        return metaTag.content.replace(/\/+$/, "");
    }

    // 3. Localhost development environment
    const host = window.location.hostname;
    if (host === "localhost" || host === "127.0.0.1") {
        if (window.location.port === "8080") {
            return window.location.origin;
        }
        // Jika frontend dibuka lewat file server terpisah (e.g. port 3000 / 5500), arahkan ke Backend Go di 8080
        return "http://localhost:8080";
    }

    // 4. Cek custom override dari localStorage (memudahkan testing di browser)
    const stored = localStorage.getItem("tokoasli_api_base");
    if (stored) {
        return stored.replace(/\/+$/, "");
    }

    // 5. Default fallback ke origin saat ini (jika frontend & backend di-serve oleh reverse proxy yang sama di VPS)
    return window.location.origin;
})();

let STORE_CONFIG = {
    whatsapp_number: "",
    store_url: window.location.origin
};

// Auto-fetch konfigurasi toko (seperti nomor WhatsApp admin untuk widget CS)
async function loadStoreConfig() {
    try {
        const res = await fetch(`${API_BASE}/api/config`);
        if (res.ok) {
            const json = await res.json();
            if (json.success && json.data) {
                STORE_CONFIG = Object.assign(STORE_CONFIG, json.data);
                injectWhatsAppWidget();
            }
        }
    } catch (e) {
        // Non-blocking failover
    }
}

// Injeksi tombol floating WhatsApp CS yang elegan
function injectWhatsAppWidget(contextMessage) {
    if (!STORE_CONFIG.whatsapp_number) return;
    if (document.getElementById("tokoasli-wa-float")) return;

    let phone = STORE_CONFIG.whatsapp_number.replace(/[^0-9]/g, "");
    if (phone.startsWith("0")) {
        phone = "62" + phone.slice(1);
    }

    let defaultMsg = "Halo Admin TokoAsli, saya butuh bantuan terkait pesanan saya.";
    if (contextMessage) {
        defaultMsg = contextMessage;
    }

    const waUrl = `https://wa.me/${phone}?text=${encodeURIComponent(defaultMsg)}`;

    const btn = document.createElement("a");
    btn.id = "tokoasli-wa-float";
    btn.href = waUrl;
    btn.target = "_blank";
    btn.rel = "noopener noreferrer";
    btn.setAttribute("title", "Bantuan Customer Service via WhatsApp");
    btn.innerHTML = `
        <svg viewBox="0 0 32 32" width="28" height="28" fill="currentColor">
            <path d="M16.002 2C8.28 2 2 8.28 2 16.002c0 2.658.742 5.143 2.031 7.27L2 30l6.937-2.008A13.93 13.93 0 0 0 16.002 30C23.72 30 30 23.72 30 16.002 30 8.28 23.72 2 16.002 2zm7.625 19.895c-.328.922-1.636 1.766-2.656 1.984-.719.149-1.656.273-4.812-1.039-3.797-1.578-6.234-5.437-6.422-5.695-.187-.258-1.531-2.039-1.531-3.891 0-1.852.969-2.766 1.312-3.14.344-.375.75-.469 1-.469.25 0 .5.008.719.016.234.008.547-.086.859.656.328.781 1.109 2.703 1.203 2.906.094.203.156.445.023.719-.133.273-.203.445-.406.688-.203.242-.43.539-.617.727-.211.211-.43.438-.187.859.242.422 1.078 1.781 2.32 2.883 1.602 1.422 2.945 1.867 3.367 2.07.422.203.672.18.922-.109.25-.289 1.078-1.258 1.367-1.695.289-.438.578-.367.969-.219.391.148 2.477 1.164 2.906 1.375.43.211.719.313.82.484.102.172.102.992-.226 1.914z"/>
        </svg>
        <span class="wa-label">Bantuan CS</span>
    `;

    const style = document.createElement("style");
    style.textContent = `
        #tokoasli-wa-float {
            position: fixed;
            bottom: 24px;
            right: 24px;
            background-color: #25D366;
            color: #ffffff;
            border-radius: 50px;
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 10px 18px 10px 14px;
            box-shadow: 0 4px 14px rgba(37, 211, 102, 0.4);
            text-decoration: none;
            font-family: inherit;
            font-weight: 600;
            font-size: 14px;
            z-index: 9999;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        #tokoasli-wa-float:hover {
            transform: translateY(-3px);
            box-shadow: 0 6px 20px rgba(37, 211, 102, 0.6);
            color: #ffffff;
        }
        @media (max-width: 640px) {
            #tokoasli-wa-float .wa-label {
                display: none;
            }
            #tokoasli-wa-float {
                padding: 12px;
                border-radius: 50%;
                bottom: 18px;
                right: 18px;
            }
        }
    `;

    document.head.appendChild(style);
    document.body.appendChild(btn);
}

// Inisialisasi konfigurasi saat DOM siap
if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", loadStoreConfig);
} else {
    loadStoreConfig();
}

console.log("[TokoAsli v2.0] API Base URL:", API_BASE);

// Normalisasi URL gambar (mendukung path relatif /uploads saat frontend di hosting terpisah)
function formatImageUrl(url) {
    if (!url) return "";
    if (url.startsWith("http://") || url.startsWith("https://") || url.startsWith("data:")) return url;
    return `${API_BASE}${url.startsWith("/") ? "" : "/"}${url}`;
}
