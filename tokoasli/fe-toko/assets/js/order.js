// Logic Halaman Order: Tampilan QRIS, Hitung Mundur, Real-time SSE Stream, dan Kotak Klaim Produk
let orderId = "";
let secretToken = "";
let sseSource = null;
let pollTimer = null;
let countdownTimer = null;

document.addEventListener("DOMContentLoaded", () => {
    const params = new URLSearchParams(window.location.search);
    orderId = params.get("id");
    secretToken = params.get("token") || "";

    if (!orderId) {
        showErrorView("ID Pesanan tidak ditemukan pada URL.");
        return;
    }

    // Fallback: jika token tidak ada di parameter URL, pulihkan dari riwayat browser
    if (!secretToken && typeof Storage !== "undefined") {
        const saved = Storage.getOrders().find(o => o.id === orderId);
        if (saved && saved.token) {
            secretToken = saved.token;
        }
    }

    loadOrderDetail();
});

// 1. Memuat Status Pesanan dari Backend
async function loadOrderDetail() {
    try {
        const url = `${API_BASE}/api/orders/${encodeURIComponent(orderId)}?token=${encodeURIComponent(secretToken)}`;
        const res = await fetch(url);
        const json = await res.json();

        if (!res.ok || !json.success) {
            throw new Error(json.error || "Pesanan tidak ditemukan.");
        }

        const order = json.data;

        // Simpan / update ke localStorage
        Storage.saveOrder({
            id: order.id,
            token: secretToken,
            product_name: order.product_name,
            total_amount: order.total_amount,
            status: order.status
        });

        // Update WhatsApp CS context dengan Invoice ID
        if (typeof injectWhatsAppWidget === "function") {
            const el = document.getElementById("tokoasli-wa-float");
            if (el) el.remove();
            injectWhatsAppWidget(`Halo Admin TokoAsli, saya butuh bantuan untuk pesanan saya #${order.id} (${order.product_name || "Produk Digital"}).`);
        }

        if (order.status === "PAID") {
            renderPaidView(order);
        } else if (order.status === "EXPIRED") {
            renderExpiredView(order);
        } else if (order.status === "CANCELLED") {
            renderCancelledView(order.id);
        } else {
            renderPendingView(order);
            startRealtimeListener();
        }

    } catch (err) {
        console.error("Load order error:", err);
        showErrorView(err.message);
    }
}

// 2. Render Tampilan Menunggu Pembayaran (PENDING)
function renderPendingView(order) {
    ["loading-state", "paid-state", "expired-state", "error-state"].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add("hidden");
    });

    const pendingState = document.getElementById("pending-state");
    if (pendingState) pendingState.classList.remove("hidden");

    // Isi Informasi secara aman
    const invEl = document.getElementById("order-invoice-id");
    if (invEl) invEl.textContent = `#${order.id}`;

    const prodNameEl = document.getElementById("order-prod-name");
    if (prodNameEl) prodNameEl.textContent = order.product_name || "Produk Digital";

    const totalEl = document.getElementById("order-total-amount");
    if (totalEl) totalEl.textContent = Storage.formatRupiah(order.total_amount);

    const codeEl = document.getElementById("order-unique-code");
    if (codeEl) codeEl.textContent = order.unique_code ?? 0;

    // Tampilkan Gambar QRIS
    const qrImg = document.getElementById("order-qr-img");
    if (qrImg) {
        qrImg.onerror = () => {
            const fallbackData = order.qr_string || order.id;
            qrImg.onerror = null;
            qrImg.src = `https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=${encodeURIComponent(fallbackData)}`;
        };
        if (order.qr_url) {
            qrImg.src = (typeof formatImageUrl === "function" ? formatImageUrl(order.qr_url) : order.qr_url);
        } else {
            qrImg.src = `https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=${encodeURIComponent(order.qr_string || order.id)}`;
        }
    }

    // Mulai Countdown Timer
    startCountdown(order.expires_at);
}

// 3. Render Tampilan Pembayaran Berhasil (PAID)
function renderPaidView(order) {
    // Matikan listener & timer
    stopRealtimeListener();
    if (countdownTimer) clearInterval(countdownTimer);

    ["loading-state", "pending-state", "expired-state", "error-state"].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add("hidden");
    });

    const paidState = document.getElementById("paid-state");
    if (paidState) paidState.classList.remove("hidden");

    const invEl = document.getElementById("paid-invoice-id");
    if (invEl) invEl.textContent = `#${order.id}`;

    const prodNameEl = document.getElementById("paid-prod-name");
    if (prodNameEl) prodNameEl.textContent = order.product_name || "Produk Digital";

    const totalEl = document.getElementById("paid-total-amount");
    if (totalEl) totalEl.textContent = Storage.formatRupiah(order.total_amount);

    const deliveryBox = document.getElementById("paid-delivery-content");
    if (deliveryBox) {
        deliveryBox.value = order.delivery_content || "Data produk berhasil dikirim ke email Anda.";
    }

    // Sediakan tombol buka langsung jika konten berupa link website / cloud drive
    const trimmedContent = (order.delivery_content || "").trim();
    let linkBtn = document.getElementById("btn-open-delivery-link");
    if (trimmedContent.startsWith("http://") || trimmedContent.startsWith("https://")) {
        if (!linkBtn) {
            linkBtn = document.createElement("a");
            linkBtn.id = "btn-open-delivery-link";
            linkBtn.target = "_blank";
            linkBtn.rel = "noopener noreferrer";
            linkBtn.className = "w-full mt-3 py-3 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-bold text-sm transition shadow-lg shadow-blue-600/25 flex items-center justify-center gap-2 text-center";
            if (deliveryBox && deliveryBox.parentElement) {
                deliveryBox.parentElement.appendChild(linkBtn);
            }
        }
        linkBtn.href = trimmedContent;
        linkBtn.innerHTML = `<span>🚀</span> Buka Link Akses / Unduhan`;
        linkBtn.classList.remove("hidden");
    } else if (linkBtn) {
        linkBtn.classList.add("hidden");
    }
}

// 4. Render Tampilan Kedaluwarsa (EXPIRED)
function renderExpiredView(order) {
    stopRealtimeListener();
    if (countdownTimer) clearInterval(countdownTimer);

    ["loading-state", "pending-state", "paid-state", "error-state"].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add("hidden");
    });

    const expiredState = document.getElementById("expired-state");
    if (expiredState) expiredState.classList.remove("hidden");

    const invEl = document.getElementById("expired-invoice-id");
    if (invEl) invEl.textContent = `#${order.id}`;
}

// 5. Tampilan Error
function showErrorView(msg) {
    ["loading-state", "pending-state", "paid-state", "expired-state"].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add("hidden");
    });

    const errState = document.getElementById("error-state");
    if (errState) errState.classList.remove("hidden");

    const errMsg = document.getElementById("error-message");
    if (errMsg) errMsg.textContent = msg;
}

// 6. Realtime Server-Sent Events (SSE) Listener
function startRealtimeListener() {
    stopRealtimeListener();

    const sseUrl = `${API_BASE}/api/orders/${encodeURIComponent(orderId)}/stream?token=${encodeURIComponent(secretToken)}`;
    console.log("[SSE] Membuka koneksi realtime:", sseUrl);

    sseSource = new EventSource(sseUrl);

    sseSource.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            console.log("[SSE] Menerima update status:", data);

            if (data.status === "PAID") {
                Storage.updateStatus(orderId, "PAID");
                loadOrderDetail(); // Reload detail untuk render tampilan lunas
            } else if (data.status === "EXPIRED") {
                Storage.updateStatus(orderId, "EXPIRED");
                renderExpiredView({ id: orderId });
            } else if (data.status === "CANCELLED") {
                Storage.updateStatus(orderId, "CANCELLED");
                renderCancelledView(orderId);
            }
        } catch (e) {
            console.error("[SSE] Parse error:", e);
        }
    };

    sseSource.onerror = (err) => {
        console.warn("[SSE] Koneksi terputus, fallback ke polling:", err);
        sseSource.close();
        // Fallback polling setiap 3 detik jika SSE bermasalah di browser pembeli
        startFallbackPolling();
    };
}

function stopRealtimeListener() {
    if (sseSource) {
        sseSource.close();
        sseSource = null;
    }
    if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
    }
}

function startFallbackPolling() {
    if (pollTimer) return;
    pollTimer = setInterval(async () => {
        try {
            const res = await fetch(`${API_BASE}/api/orders/${encodeURIComponent(orderId)}?token=${encodeURIComponent(secretToken)}`);
            const json = await res.json();
            if (json.success && json.data) {
                if (json.data.status === "PAID") {
                    Storage.updateStatus(orderId, "PAID");
                    renderPaidView(json.data);
                } else if (json.data.status === "EXPIRED") {
                    Storage.updateStatus(orderId, "EXPIRED");
                    renderExpiredView({ id: orderId });
                } else if (json.data.status === "CANCELLED") {
                    Storage.updateStatus(orderId, "CANCELLED");
                    renderCancelledView(orderId);
                }
            }
        } catch (e) {}
    }, 3000);
}

// 7. Hitung Mundur Waktu Bayar (15 Menit)
function startCountdown(expiresAtStr) {
    if (countdownTimer) clearInterval(countdownTimer);

    const expiresAt = new Date(expiresAtStr).getTime();

    function update() {
        const now = new Date().getTime();
        const diff = expiresAt - now;

        if (diff <= 0) {
            clearInterval(countdownTimer);
            const timerEl = document.getElementById("countdown-timer");
            if (timerEl) timerEl.textContent = "00:00";
            Storage.updateStatus(orderId, "EXPIRED");
            renderExpiredView({ id: orderId });
            return;
        }

        const mins = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
        const secs = Math.floor((diff % (1000 * 60)) / 1000);

        const pad = (n) => n < 10 ? `0${n}` : n;
        const timerEl = document.getElementById("countdown-timer");
        if (timerEl) timerEl.textContent = `${pad(mins)}:${pad(secs)}`;
    }

    update();
    countdownTimer = setInterval(update, 1000);
}

// 8. Clipboard Helpers
function copyTotalAmount() {
    const totalEl = document.getElementById("order-total-amount");
    if (!totalEl) return;
    const rawNumber = totalEl.textContent.replace(/[^0-9]/g, "");
    if (rawNumber && navigator.clipboard) {
        navigator.clipboard.writeText(rawNumber).then(() => {
            showToast("Nominal disalin: " + rawNumber);
        });
    }
}

function copyDeliveryContent() {
    const box = document.getElementById("paid-delivery-content");
    if (!box) return;
    box.select();
    if (navigator.clipboard) {
        navigator.clipboard.writeText(box.value).then(() => {
            showToast("✓ Data produk berhasil disalin!");
        });
    }
}

function showToast(msg) {
    const toast = document.getElementById("toast");
    toast.textContent = msg;
    toast.classList.remove("opacity-0", "translate-y-4");
    setTimeout(() => {
        toast.classList.add("opacity-0", "translate-y-4");
    }, 2500);
}

// Download QRIS Image to Device Gallery
async function downloadQRIS() {
    const qrImg = document.getElementById("order-qr-img");
    if (!qrImg || !qrImg.src) {
        showToast("Gambar QRIS belum siap");
        return;
    }

    try {
        const response = await fetch(qrImg.src);
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.style.display = "none";
        a.href = url;
        a.download = `QRIS_${orderId || "PEMBAYARAN"}.png`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
        showToast("✓ QRIS tersimpan ke Galeri/Downloads!");
    } catch (err) {
        // Fallback: open image in new tab if direct download blocked by CORS
        window.open(qrImg.src, "_blank");
        showToast("Membuka gambar QRIS...");
    }
}

// 9. Render Tampilan Dibatalkan (CANCELLED)
function renderCancelledView(orderIdToRender) {
    stopRealtimeListener();
    if (countdownTimer) clearInterval(countdownTimer);

    ["loading-state", "pending-state", "paid-state", "expired-state", "error-state"].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.add("hidden");
    });

    const cancelledState = document.getElementById("cancelled-state");
    if (cancelledState) cancelledState.classList.remove("hidden");

    const invEl = document.getElementById("cancelled-invoice-id");
    if (invEl) invEl.textContent = `#${orderIdToRender || orderId}`;
}

// 10. Cek Status Pembayaran Manual (Sudah Transfer?)
async function checkPaymentStatusManual() {
    const btn = document.getElementById("btn-manual-check");
    if (!btn) return;

    btn.disabled = true;
    const originalText = btn.innerHTML;
    btn.innerHTML = `<span class="inline-block animate-spin mr-1">⟳</span> Menghubungi Gateway Pembayaran...`;

    try {
        const res = await fetch(`${API_BASE}/api/orders/${encodeURIComponent(orderId)}/check`, {
            method: "POST"
        });
        const json = await res.json();

        if (json.success && json.data) {
            if (json.data.status === "PAID") {
                showToast("✓ Pembayaran Lunas Terverifikasi!");
                Storage.updateStatus(orderId, "PAID");
                setTimeout(() => loadOrderDetail(), 500);
                return;
            } else if (json.data.status === "CANCELLED") {
                renderCancelledView(orderId);
                return;
            } else if (json.data.status === "EXPIRED") {
                renderExpiredView({ id: orderId });
                return;
            } else {
                showToast(json.data.message || "Mutasi belum terdeteksi. Silakan coba 1 menit lagi.");
            }
        } else {
            showToast("Belum ada mutasi masuk. Tunggu 1-2 menit setelah transfer.");
        }
    } catch (err) {
        console.error("Check status error:", err);
        showToast("Gagal menghubungi server. Periksa koneksi Anda.");
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

// 11. Batalkan Pesanan Pembeli (Lepas Stok Segera)
async function cancelCurrentOrder() {
    if (!confirm("Apakah Anda yakin ingin membatalkan pesanan ini? Stok produk akan dilepas kembali ke toko.")) {
        return;
    }

    const btn = document.getElementById("btn-cancel-order");
    if (btn) {
        btn.disabled = true;
        btn.textContent = "Membatalkan...";
    }

    try {
        const res = await fetch(`${API_BASE}/api/orders/${encodeURIComponent(orderId)}/cancel?token=${encodeURIComponent(secretToken)}`, {
            method: "POST"
        });
        const json = await res.json();

        if (json.success) {
            Storage.updateStatus(orderId, "CANCELLED");
            showToast("✓ Pesanan berhasil dibatalkan.");
            renderCancelledView(orderId);
        } else {
            alert(json.error || "Gagal membatalkan pesanan.");
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = `<span>✕</span> Batalkan Pesanan`;
            }
        }
    } catch (err) {
        alert("Gagal menghubungi server.");
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = `<span>✕</span> Batalkan Pesanan`;
        }
    }
}

// 12. Salin Link Akses Halaman Ini
function copyPageAccessLink() {
    const url = window.location.href;
    if (navigator.clipboard) {
        navigator.clipboard.writeText(url).then(() => {
            showToast("✓ Link akses klaim pesanan disalin ke clipboard!");
        }).catch(() => {
            prompt("Salin link akses halaman Anda berikut:", url);
        });
    } else {
        prompt("Salin link akses halaman Anda berikut:", url);
    }
}
