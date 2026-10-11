// Logic Dashboard Admin: Autentikasi Password, CRUD Produk, Textarea Bulk Stok & Monitor Order
let adminToken = sessionStorage.getItem("tokoasli_admin_token") || "";
let editingProductId = null;
let stockProductId = null;
let currentOrderStatus = "ALL";
let adminProductsList = [];
let adminOrdersList = [];
let currentViewingOrderId = null;
let currentOrdersPage = 1;
let ordersPageLimit = 20;
let ordersTotalPages = 1;
let currentStockDetailProductId = null;
let currentStockDetailProductName = "";

document.addEventListener("DOMContentLoaded", () => {
    if (adminToken) {
        showDashboard();
    } else {
        showLogin();
    }
});

// 1. Tampilan Login vs Dashboard
function showLogin() {
    document.getElementById("login-card").classList.remove("hidden");
    document.getElementById("admin-dashboard").classList.add("hidden");
    document.getElementById("admin-password").value = "";
    document.getElementById("login-error").classList.add("hidden");
}

function showDashboard() {
    document.getElementById("login-card").classList.add("hidden");
    document.getElementById("admin-dashboard").classList.remove("hidden");

    loadStats();
    loadAdminProducts();
    loadAdminOrders();
}

// 2. Form Login Admin
async function handleAdminLogin(e) {
    e.preventDefault();
    const passInput = document.getElementById("admin-password");
    const errorEl = document.getElementById("login-error");
    const btn = document.getElementById("btn-login");

    const password = passInput.value.trim();
    if (!password) return;

    btn.disabled = true;
    btn.textContent = "Memverifikasi...";
    errorEl.classList.add("hidden");

    try {
        const res = await fetch(`${API_BASE}/api/admin/login`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ password })
        });
        const json = await res.json();

        if (!res.ok || !json.success) {
            throw new Error(json.error || "Password salah");
        }

        adminToken = json.token;
        sessionStorage.setItem("tokoasli_admin_token", adminToken);
        showDashboard();

    } catch (err) {
        errorEl.textContent = err.message;
        errorEl.classList.remove("hidden");
    } finally {
        btn.disabled = false;
        btn.textContent = "Masuk ke Dashboard →";
    }
}

function handleLogout() {
    adminToken = "";
    sessionStorage.removeItem("tokoasli_admin_token");
    showLogin();
}

function getAuthHeaders() {
    return {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`
    };
}

// 3. Memuat Statistik Bisnis
async function loadStats() {
    try {
        const res = await fetch(`${API_BASE}/api/admin/stats`, { headers: getAuthHeaders() });
        const json = await res.json();
        if (json.success && json.data) {
            const d = json.data;
            document.getElementById("stat-revenue").textContent = Storage.formatRupiah(d.total_revenue || 0);
            document.getElementById("stat-paid-orders").textContent = d.total_paid_orders || 0;
            document.getElementById("stat-pending-orders").textContent = d.total_pending_orders || 0;
            document.getElementById("stat-products").textContent = d.total_products || 0;
        }
    } catch (err) {
        console.error("Stats error:", err);
    }
}

// 4. Manajemen Produk (List, Create, Update, Delete)
async function loadAdminProducts() {
    const tableBody = document.getElementById("products-table-body");
    try {
        const res = await fetch(`${API_BASE}/api/admin/products`, { headers: getAuthHeaders() });
        const json = await res.json();

        if (!json.success || !Array.isArray(json.data)) {
            tableBody.innerHTML = `<tr><td colspan="6" class="text-center py-6 text-slate-400">Gagal memuat produk.</td></tr>`;
            return;
        }

        adminProductsList = json.data;
        const products = json.data;
        if (products.length === 0) {
            tableBody.innerHTML = `<tr><td colspan="6" class="text-center py-8 text-slate-400">Belum ada produk. Tambahkan produk pertama Anda!</td></tr>`;
            return;
        }

        tableBody.innerHTML = products.map(p => {
            const stockLabel = p.delivery_type === 'fixed'
                ? `<span class="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 text-xs font-semibold">Unlimited (Link)</span>`
                : (p.stock_count > 0 
                    ? `<span class="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-xs font-semibold">${p.stock_count} item</span>`
                    : `<span class="px-2 py-0.5 rounded-full bg-red-500/10 text-red-400 text-xs font-semibold">Habis</span>`);

            return `
                <tr class="border-b border-slate-800 hover:bg-slate-800/30 transition text-sm">
                    <td class="py-3 px-4">
                        <div class="flex items-center gap-3">
                            ${p.image_url ? `<img src="${escapeHtml(typeof formatImageUrl === "function" ? formatImageUrl(p.image_url) : p.image_url)}" class="w-10 h-10 object-cover rounded-lg border border-slate-700">` : `<div class="w-10 h-10 rounded-lg bg-slate-800 flex items-center justify-center text-lg">⚡</div>`}
                            <div>
                                <div class="font-bold text-slate-100">${escapeHtml(p.name)}</div>
                                <div class="text-[11px] text-slate-400">${escapeHtml(p.category)} &bull; ${escapeHtml(p.id)}</div>
                            </div>
                        </div>
                    </td>
                    <td class="py-3 px-4 font-bold text-emerald-400">${Storage.formatRupiah(p.price)}</td>
                    <td class="py-3 px-4">${stockLabel}</td>
                    <td class="py-3 px-4">
                        <button onclick="toggleProductStatus('${escapeHtml(p.id)}')" 
                                class="px-2.5 py-1 rounded-full text-xs font-bold transition ${p.is_active ? 'bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 border border-emerald-500/20' : 'bg-slate-700/60 text-slate-400 hover:bg-slate-700 border border-slate-600'}" 
                                title="Klik untuk ubah status aktif/non-aktif">
                            ${p.is_active ? '● Aktif' : '○ Non-aktif'}
                        </button>
                    </td>
                    <td class="py-3 px-4 text-right flex items-center justify-end gap-1.5">
                        ${p.delivery_type === 'pool' ? `
                            <button onclick="openStockDetailModal('${escapeHtml(p.id)}', '${escapeHtml(p.name)}')"
                                    class="px-2.5 py-1 rounded-lg bg-indigo-600/20 text-indigo-400 hover:bg-indigo-600/30 text-xs font-semibold border border-indigo-500/30 transition cursor-pointer" title="Lihat Rincian Stok">
                                🔍 Stok
                            </button>
                            <button onclick="openStockModal('${escapeHtml(p.id)}', '${escapeHtml(p.name)}')"
                                    class="px-2.5 py-1 rounded-lg bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/30 text-xs font-semibold border border-emerald-500/30 transition cursor-pointer" title="Isi Stok">
                                + Stok
                            </button>
                        ` : ''}
                        <button onclick="openEditProductModal('${escapeHtml(p.id)}')"
                                class="px-2.5 py-1 rounded-lg bg-blue-600/20 text-blue-400 hover:bg-blue-600/30 text-xs font-semibold border border-blue-500/30 transition cursor-pointer">
                            Edit
                        </button>
                        <button onclick="deleteProduct('${escapeHtml(p.id)}')"
                                class="px-2.5 py-1 rounded-lg bg-red-600/20 text-red-400 hover:bg-red-600/30 text-xs font-semibold border border-red-500/30 transition cursor-pointer">
                            Hapus
                        </button>
                    </td>
                </tr>
            `;
        }).join("");

    } catch (err) {
        console.error("Products error:", err);
    }
}

// 5. Modal Tambah / Edit Produk
function openAddProductModal() {
    editingProductId = null;
    document.getElementById("prod-modal-title").textContent = "Tambah Produk Baru";
    document.getElementById("prod-name").value = "";
    document.getElementById("prod-category").value = "Streaming";
    document.getElementById("prod-price").value = "";
    document.getElementById("prod-delivery-type").value = "pool";
    document.getElementById("prod-fixed-content").value = "";
    document.getElementById("prod-desc").value = "";
    document.getElementById("prod-image").value = "";
    document.getElementById("prod-is-active").checked = true;

    clearProductImage();
    toggleFixedContentField();
    document.getElementById("product-modal").classList.remove("hidden");
    document.getElementById("product-modal").classList.add("flex");
}

function openEditProductModal(id) {
    editingProductId = id;
    document.getElementById("prod-modal-title").textContent = "Edit Produk";

    // Cari dari cache adminProductsList terlebih dahulu
    const p = adminProductsList.find(x => x.id === id);
    if (p) {
        document.getElementById("prod-name").value = p.name;
        document.getElementById("prod-category").value = p.category;
        document.getElementById("prod-price").value = p.price;
        document.getElementById("prod-delivery-type").value = p.delivery_type;
        document.getElementById("prod-fixed-content").value = p.fixed_content || "";
        document.getElementById("prod-desc").value = p.description || "";
        document.getElementById("prod-image").value = p.image_url || "";
        document.getElementById("prod-is-active").checked = p.is_active;

        updateImagePreview(p.image_url || "");
        toggleFixedContentField();
        document.getElementById("product-modal").classList.remove("hidden");
        document.getElementById("product-modal").classList.add("flex");
    } else {
        alert("Produk tidak ditemukan di daftar.");
    }
}

function closeProductModal() {
    document.getElementById("product-modal").classList.add("hidden");
    document.getElementById("product-modal").classList.remove("flex");
    editingProductId = null;
}

function toggleFixedContentField() {
    const type = document.getElementById("prod-delivery-type").value;
    const box = document.getElementById("box-fixed-content");
    if (type === "fixed") {
        box.classList.remove("hidden");
    } else {
        box.classList.add("hidden");
    }
}

// Upload file foto dari komputer
async function handleImageFileUpload(event) {
    const file = event.target.files[0];
    if (!file) return;

    const btnText = document.getElementById("btn-upload-text");
    btnText.textContent = "Mengupload...";

    const formData = new FormData();
    formData.append("image", file);

    try {
        const res = await fetch(`${API_BASE}/api/admin/upload`, {
            method: "POST",
            headers: {
                "Authorization": `Bearer ${adminToken}`
            },
            body: formData
        });

        const json = await res.json();
        if (!res.ok || !json.success) {
            throw new Error(json.error || "Gagal mengupload gambar");
        }

        document.getElementById("prod-image").value = json.url;
        updateImagePreview(json.url);
        btnText.textContent = "Ganti Foto";
    } catch (err) {
        alert("Upload error: " + err.message);
        btnText.textContent = "Pilih Foto dari Komputer";
    }
}

function updateImagePreview(url) {
    const container = document.getElementById("image-preview-container");
    const preview = document.getElementById("image-preview");
    const previewUrl = document.getElementById("image-preview-url");

    if (url && url.trim()) {
        preview.src = (typeof formatImageUrl === "function" ? formatImageUrl(url) : url);
        previewUrl.textContent = url;
        container.classList.remove("hidden");
        container.classList.add("flex");
    } else {
        container.classList.add("hidden");
        container.classList.remove("flex");
        preview.src = "";
    }
}

function clearProductImage() {
    document.getElementById("prod-image").value = "";
    const fileInput = document.getElementById("prod-file-input");
    if (fileInput) fileInput.value = "";
    const btnText = document.getElementById("btn-upload-text");
    if (btnText) btnText.textContent = "Pilih Foto dari Komputer";
    updateImagePreview("");
}

async function saveProduct(e) {
    e.preventDefault();
    const payload = {
        name: document.getElementById("prod-name").value.trim(),
        category: document.getElementById("prod-category").value.trim(),
        price: parseInt(document.getElementById("prod-price").value) || 0,
        delivery_type: document.getElementById("prod-delivery-type").value,
        fixed_content: document.getElementById("prod-fixed-content").value.trim(),
        description: document.getElementById("prod-desc").value.trim(),
        image_url: document.getElementById("prod-image").value.trim(),
        is_active: document.getElementById("prod-is-active").checked
    };

    const url = editingProductId 
        ? `${API_BASE}/api/admin/products/${editingProductId}`
        : `${API_BASE}/api/admin/products`;

    const method = editingProductId ? "PUT" : "POST";

    try {
        const res = await fetch(url, {
            method: method,
            headers: getAuthHeaders(),
            body: JSON.stringify(payload)
        });
        const json = await res.json();

        if (!res.ok || !json.success) {
            throw new Error(json.error || "Gagal menyimpan produk");
        }

        closeProductModal();
        loadAdminProducts();
        loadStats();
    } catch (err) {
        alert("Error: " + err.message);
    }
}

async function deleteProduct(id) {
    if (!confirm("Hapus produk ini beserta seluruh riwayat stoknya?")) return;
    try {
        const res = await fetch(`${API_BASE}/api/admin/products/${id}`, {
            method: "DELETE",
            headers: getAuthHeaders()
        });
        const json = await res.json();
        if (json.success) {
            loadAdminProducts();
            loadStats();
        } else {
            alert(json.error);
        }
    } catch (err) {
        alert("Gagal menghapus: " + err.message);
    }
}

async function toggleProductStatus(productId) {
    try {
        const res = await fetch(`${API_BASE}/api/admin/products/${productId}/toggle`, {
            method: "PATCH",
            headers: getAuthHeaders()
        });
        const json = await res.json();
        if (!res.ok || !json.success) {
            throw new Error(json.error || "Gagal mengubah status produk");
        }
        loadAdminProducts();
        loadStats();
    } catch (err) {
        alert("Gagal: " + err.message);
    }
}

// 6. Textarea Bulk Upload Stok
function openStockModal(productId, productName) {
    stockProductId = productId;
    document.getElementById("stock-prod-name").textContent = productName;
    document.getElementById("stock-lines").value = "";
    document.getElementById("stock-counter").textContent = "0";

    document.getElementById("stock-modal").classList.remove("hidden");
    document.getElementById("stock-modal").classList.add("flex");
}

function closeStockModal() {
    document.getElementById("stock-modal").classList.add("hidden");
    document.getElementById("stock-modal").classList.remove("flex");
    stockProductId = null;
}

function updateStockCount() {
    const text = document.getElementById("stock-lines").value;
    const lines = text.split("\n").map(l => l.trim()).filter(Boolean);
    document.getElementById("stock-counter").textContent = lines.length;
}

async function submitStockUpload() {
    if (!stockProductId) return;

    const text = document.getElementById("stock-lines").value;
    const lines = text.split("\n").map(l => l.trim()).filter(Boolean);

    if (lines.length === 0) {
        alert("Silakan masukkan setidaknya 1 baris akun/voucher.");
        return;
    }

    try {
        const res = await fetch(`${API_BASE}/api/admin/products/${stockProductId}/stocks`, {
            method: "POST",
            headers: getAuthHeaders(),
            body: JSON.stringify({ lines })
        });
        const json = await res.json();

        if (json.success) {
            alert(`Berhasil menambahkan ${json.added} stok!`);
            closeStockModal();
            loadAdminProducts();
        } else {
            alert("Error: " + json.error);
        }
    } catch (err) {
        alert("Gagal upload stok: " + err.message);
    }
}

// 7. Monitoring Daftar Pesanan
async function loadAdminOrders() {
    const tableBody = document.getElementById("orders-table-body");
    try {
        const res = await fetch(`${API_BASE}/api/admin/orders?status=${encodeURIComponent(currentOrderStatus)}&page=${currentOrdersPage}&limit=${ordersPageLimit}`, {
            headers: getAuthHeaders()
        });
        const json = await res.json();

        if (!json.success || !Array.isArray(json.data)) {
            tableBody.innerHTML = `<tr><td colspan="6" class="text-center py-6 text-slate-400">Gagal memuat pesanan.</td></tr>`;
            return;
        }

        adminOrdersList = json.data;
        const orders = json.data;

        // Update pagination controls
        if (json.pagination) {
            ordersTotalPages = json.pagination.total_pages || 1;
            const totalCount = json.pagination.total || 0;
            const curPageEl = document.getElementById("orders-current-page");
            const totalPagesEl = document.getElementById("orders-total-pages");
            const totalCountEl = document.getElementById("orders-total-count");
            const prevBtn = document.getElementById("btn-orders-prev");
            const nextBtn = document.getElementById("btn-orders-next");

            if (curPageEl) curPageEl.textContent = currentOrdersPage;
            if (totalPagesEl) totalPagesEl.textContent = ordersTotalPages;
            if (totalCountEl) totalCountEl.textContent = totalCount;
            if (prevBtn) prevBtn.disabled = currentOrdersPage <= 1;
            if (nextBtn) nextBtn.disabled = currentOrdersPage >= ordersTotalPages;
        }

        if (orders.length === 0) {
            tableBody.innerHTML = `<tr><td colspan="6" class="text-center py-8 text-slate-400">Belum ada pesanan dengan status ini.</td></tr>`;
            return;
        }

        tableBody.innerHTML = orders.map(o => {
            let statusBadge = `<span class="px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 text-xs font-semibold">PENDING</span>`;
            if (o.status === "PAID") {
                statusBadge = `<span class="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-semibold">PAID</span>`;
            } else if (o.status === "EXPIRED") {
                statusBadge = `<span class="px-2 py-0.5 rounded-full bg-slate-500/10 text-slate-400 border border-slate-500/20 text-xs font-semibold">EXPIRED</span>`;
            } else if (o.status === "CANCELLED") {
                statusBadge = `<span class="px-2 py-0.5 rounded-full bg-red-500/10 text-red-400 border border-red-500/20 text-xs font-semibold">CANCELLED</span>`;
            }

            const dateStr = o.created_at ? new Date(o.created_at).toLocaleString("id-ID", {
                day: "numeric", month: "short", hour: "2-digit", minute: "2-digit"
            }) : "-";

            return `
                <tr class="border-b border-slate-800 hover:bg-slate-800/30 transition text-sm">
                    <td class="py-3 px-4 font-mono font-bold text-slate-300">#${escapeHtml(o.id)}</td>
                    <td class="py-3 px-4">
                        <div class="font-semibold text-slate-100">${escapeHtml(o.product_name)}</div>
                        <div class="text-xs text-slate-400">${escapeHtml(o.customer_email)}${o.customer_phone ? ` &bull; ${escapeHtml(o.customer_phone)}` : ''}</div>
                    </td>
                    <td class="py-3 px-4 font-bold text-emerald-400">${Storage.formatRupiah(o.total_amount)}</td>
                    <td class="py-3 px-4">${statusBadge}</td>
                    <td class="py-3 px-4 text-xs text-slate-400">${dateStr}</td>
                    <td class="py-3 px-4 text-right">
                        <div class="flex items-center justify-end gap-1.5">
                            <button onclick="openOrderDetailModal('${escapeHtml(o.id)}')"
                                    class="px-2.5 py-1 rounded-lg bg-slate-700/60 text-slate-300 hover:bg-slate-700 hover:text-white text-xs font-semibold border border-slate-600 transition cursor-pointer"
                                    title="Lihat Detail Transaksi">
                                👁️ Detail
                            </button>
                            ${o.status === "PAID" ? `
                                <button onclick="resendOrderEmail('${escapeHtml(o.id)}')"
                                        class="px-2.5 py-1 rounded-lg bg-blue-600/20 text-blue-400 hover:bg-blue-600/30 text-xs font-semibold border border-blue-500/30 transition cursor-pointer"
                                        title="Kirim Ulang Email">
                                    ✉️
                                </button>
                            ` : ''}
                        </div>
                    </td>
                </tr>
            `;
        }).join("");

    } catch (err) {
        console.error("Orders error:", err);
    }
}

function changeOrdersPage(delta) {
    const target = currentOrdersPage + delta;
    if (target < 1 || target > ordersTotalPages) return;
    currentOrdersPage = target;
    loadAdminOrders();
}

function filterOrderStatus(status) {
    currentOrderStatus = status;
    currentOrdersPage = 1;
    document.querySelectorAll(".btn-order-filter").forEach(b => {
        b.classList.remove("bg-blue-600", "text-white");
        b.classList.add("bg-slate-800", "text-slate-400");
    });
    if (window.event && window.event.target) {
        window.event.target.classList.add("bg-blue-600", "text-white");
        window.event.target.classList.remove("bg-slate-800", "text-slate-400");
    }

    loadAdminOrders();
}

// 8. Ekspor Daftar Pesanan ke CSV
async function exportOrdersCSV() {
    try {
        const res = await fetch(`${API_BASE}/api/admin/orders/export?status=${encodeURIComponent(currentOrderStatus)}`, {
            headers: getAuthHeaders()
        });
        if (!res.ok) throw new Error("Gagal mengunduh CSV");
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.style.display = "none";
        a.href = url;
        a.download = `orders_${currentOrderStatus.toLowerCase()}_${new Date().toISOString().slice(0, 10)}.csv`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
    } catch (err) {
        alert("Gagal ekspor CSV: " + err.message);
    }
}

function openOrderDetailModal(orderId) {
    const o = adminOrdersList.find(x => x.id === orderId);
    if (!o) {
        alert("Pesanan tidak ditemukan");
        return;
    }
    currentViewingOrderId = orderId;
    document.getElementById("admin-detail-invoice").textContent = `#${o.id}`;
    document.getElementById("admin-detail-prod").textContent = o.product_name || "-";
    document.getElementById("admin-detail-amount").textContent = Storage.formatRupiah(o.total_amount);
    document.getElementById("admin-detail-email").textContent = o.customer_email || "-";

    let statusBadge = `<span class="px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 text-xs font-semibold">PENDING</span>`;
    if (o.status === "PAID") {
        statusBadge = `<span class="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-semibold">PAID (Lunas)</span>`;
    } else if (o.status === "EXPIRED") {
        statusBadge = `<span class="px-2 py-0.5 rounded-full bg-slate-500/10 text-slate-400 border border-slate-500/20 text-xs font-semibold">EXPIRED</span>`;
    } else if (o.status === "CANCELLED") {
        statusBadge = `<span class="px-2 py-0.5 rounded-full bg-red-500/10 text-red-400 border border-red-500/20 text-xs font-semibold">CANCELLED (Dibatalkan)</span>`;
    }
    document.getElementById("admin-detail-status").innerHTML = statusBadge;

    const createdStr = o.created_at ? new Date(o.created_at).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "medium" }) : "-";
    const paidStr = o.paid_at ? new Date(o.paid_at).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "medium" }) : "-";
    document.getElementById("admin-detail-created").textContent = createdStr;
    document.getElementById("admin-detail-paid").textContent = paidStr;
    document.getElementById("admin-detail-tx").textContent = o.paymentg_order_id || "-";

    const deliveryBox = document.getElementById("admin-detail-delivery");
    if (o.delivery_content && o.delivery_content.trim()) {
        deliveryBox.value = o.delivery_content;
    } else if (o.status === "PAID") {
        deliveryBox.value = "(Tidak ada konten tersimpan atau produk instan sudah terkirim)";
    } else {
        deliveryBox.value = "(Belum lunas - akun/voucher belum dirilis)";
    }

    const resendBtn = document.getElementById("btn-admin-resend-modal");
    if (o.status === "PAID") {
        resendBtn.classList.remove("hidden");
    } else {
        resendBtn.classList.add("hidden");
    }

    const markPaidBtn = document.getElementById("btn-admin-mark-paid");
    if (markPaidBtn) {
        if (o.status === "PENDING") {
            markPaidBtn.classList.remove("hidden");
        } else {
            markPaidBtn.classList.add("hidden");
        }
    }

    const modal = document.getElementById("order-detail-modal");
    modal.classList.remove("hidden");
    modal.classList.add("flex");
}

function closeOrderDetailModal() {
    const modal = document.getElementById("order-detail-modal");
    if (modal) {
        modal.classList.add("hidden");
        modal.classList.remove("flex");
    }
    currentViewingOrderId = null;
}

// 9. Konfirmasi Pelunasan Manual oleh Admin
async function markCurrentOrderPaid() {
    if (!currentViewingOrderId) return;
    if (!confirm(`Konfirmasi pelunasan manual untuk pesanan #${currentViewingOrderId}? Stok akun akan langsung dilepas dan email tanda terima akan dikirim ke pembeli.`)) {
        return;
    }

    const btn = document.getElementById("btn-admin-mark-paid");
    if (btn) {
        btn.disabled = true;
        btn.textContent = "Memproses...";
    }

    try {
        const res = await fetch(`${API_BASE}/api/admin/orders/${encodeURIComponent(currentViewingOrderId)}/mark-paid`, {
            method: "POST",
            headers: getAuthHeaders()
        });
        const json = await res.json();
        if (!res.ok || !json.success) {
            throw new Error(json.error || "Gagal konfirmasi lunas manual");
        }

        alert("✓ Pesanan berhasil ditandai Lunas!");
        closeOrderDetailModal();
        loadStats();
        loadAdminOrders();
        loadAdminProducts();
    } catch (err) {
        alert("Error: " + err.message);
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = "⚡ Konfirmasi Lunas Manual";
        }
    }
}

function copyAdminDeliveryContent() {
    const deliveryBox = document.getElementById("admin-detail-delivery");
    if (!deliveryBox || !deliveryBox.value || deliveryBox.value.startsWith("(")) {
        alert("Tidak ada konten yang dapat disalin");
        return;
    }
    navigator.clipboard.writeText(deliveryBox.value).then(() => {
        alert("Berhasil disalin ke clipboard!");
    }).catch(() => {
        deliveryBox.select();
        document.execCommand("copy");
        alert("Berhasil disalin!");
    });
}

function resendCurrentOrderEmail() {
    if (currentViewingOrderId) {
        resendOrderEmail(currentViewingOrderId);
    }
}

async function resendOrderEmail(orderId) {
    if (!confirm(`Kirim ulang email tanda terima untuk order #${orderId}?`)) return;
    try {
        const res = await fetch(`${API_BASE}/api/admin/orders/${orderId}/resend-email`, {
            method: "POST",
            headers: getAuthHeaders()
        });
        const json = await res.json();
        alert(json.message || json.error);
    } catch (err) {
        alert("Gagal: " + err.message);
    }
}

// 10. Modal Rincian Pool Stok Produk
async function openStockDetailModal(productId, productName) {
    currentStockDetailProductId = productId;
    currentStockDetailProductName = productName;
    document.getElementById("stock-detail-prod-name").textContent = productName;

    document.getElementById("stock-detail-modal").classList.remove("hidden");
    document.getElementById("stock-detail-modal").classList.add("flex");

    await loadProductStockDetails(productId);
}

function closeStockDetailModal() {
    const modal = document.getElementById("stock-detail-modal");
    if (modal) {
        modal.classList.add("hidden");
        modal.classList.remove("flex");
    }
    currentStockDetailProductId = null;
}

function openStockModalFromDetail() {
    const pId = currentStockDetailProductId;
    const pName = currentStockDetailProductName;
    closeStockDetailModal();
    if (pId) {
        openStockModal(pId, pName);
    }
}

async function loadProductStockDetails(productId) {
    const tbody = document.getElementById("stock-items-table-body");
    tbody.innerHTML = `<tr><td colspan="4" class="p-4 text-center text-slate-400">Memuat rincian stok...</td></tr>`;

    try {
        const res = await fetch(`${API_BASE}/api/admin/products/${encodeURIComponent(productId)}/stocks`, {
            headers: getAuthHeaders()
        });
        const json = await res.json();

        if (!json.success || !json.data) {
            tbody.innerHTML = `<tr><td colspan="4" class="p-4 text-center text-slate-400">Gagal memuat rincian stok.</td></tr>`;
            return;
        }

        const data = json.data;
        const summary = data;
        const items = data.items || [];

        document.getElementById("stock-summary-available").textContent = summary.available_count || 0;
        document.getElementById("stock-summary-held").textContent = summary.held_count || 0;
        document.getElementById("stock-summary-sold").textContent = summary.sold_count || 0;
        document.getElementById("stock-summary-total").textContent = summary.total_count || 0;

        if (items.length === 0) {
            tbody.innerHTML = `<tr><td colspan="4" class="p-6 text-center text-slate-400">Belum ada item stok di pool ini.</td></tr>`;
            return;
        }

        tbody.innerHTML = items.map(item => {
            let statusBadge = `<span class="px-2 py-0.5 rounded text-[11px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">Tersedia</span>`;
            let canDelete = true;

            if (item.status === "held") {
                statusBadge = `<span class="px-2 py-0.5 rounded text-[11px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20">Hold (${escapeHtml(item.held_by_order_id || '')})</span>`;
                canDelete = false;
            } else if (item.status === "sold") {
                statusBadge = `<span class="px-2 py-0.5 rounded text-[11px] font-bold bg-blue-500/10 text-blue-400 border border-blue-500/20">Terjual (${escapeHtml(item.held_by_order_id || '')})</span>`;
                canDelete = false;
            }

            return `
                <tr class="hover:bg-slate-800/40">
                    <td class="p-3 text-slate-400 text-[11px]">#${item.id}</td>
                    <td class="p-3 text-slate-200 max-w-xs truncate" title="${escapeHtml(item.content)}">${escapeHtml(item.content)}</td>
                    <td class="p-3">${statusBadge}</td>
                    <td class="p-3 text-right">
                        ${canDelete ? `
                            <button onclick="deleteStockItem(${item.id})" class="px-2 py-1 rounded bg-red-600/20 text-red-400 hover:bg-red-600/30 text-[11px] font-semibold border border-red-500/30 transition cursor-pointer">
                                Hapus
                            </button>
                        ` : '<span class="text-slate-600 text-[11px]">-</span>'}
                    </td>
                </tr>
            `;
        }).join("");

    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="4" class="p-4 text-center text-red-400">Error: ${escapeHtml(err.message)}</td></tr>`;
    }
}

async function deleteStockItem(stockId) {
    if (!confirm("Hapus baris stok ini dari pool?")) return;
    try {
        const res = await fetch(`${API_BASE}/api/admin/stocks/${stockId}`, {
            method: "DELETE",
            headers: getAuthHeaders()
        });
        const json = await res.json();
        if (json.success) {
            if (currentStockDetailProductId) {
                loadProductStockDetails(currentStockDetailProductId);
            }
            loadAdminProducts();
            loadStats();
        } else {
            alert(json.error || "Gagal menghapus item stok");
        }
    } catch (err) {
        alert("Gagal menghapus: " + err.message);
    }
}

// Global shortcut Escape to close any modal
document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
        closeProductModal();
        closeStockModal();
        closeOrderDetailModal();
        closeStockDetailModal();
    }
});

// Click outside modal panel to close
["product-modal", "stock-modal", "order-detail-modal", "stock-detail-modal"].forEach(modalId => {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.addEventListener("click", (e) => {
            if (e.target === modal) {
                if (modalId === "product-modal") closeProductModal();
                if (modalId === "stock-modal") closeStockModal();
                if (modalId === "order-detail-modal") closeOrderDetailModal();
                if (modalId === "stock-detail-modal") closeStockDetailModal();
            }
        });
    }
});

function escapeHtml(text) {
    if (!text && text !== 0) return "";
    return String(text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}
