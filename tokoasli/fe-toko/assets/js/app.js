// Logic Beranda Katalog Produk TokoAsli (Direct Page Navigation ke /product/?id=...)
let allProducts = [];
let selectedCategory = "ALL";
let searchQuery = "";

document.addEventListener("DOMContentLoaded", () => {
    initHistoryButton();
    loadProducts();

    // Event listener untuk update jumlah riwayat
    window.addEventListener("tokoasli_orders_updated", updateHistoryBadge);
    updateHistoryBadge();

    // Tutup drawer riwayat dengan tombol Escape
    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape") {
            closeHistoryDrawer();
        }
    });
});

// 1. Inisialisasi Tombol & Backdrop Riwayat
function initHistoryButton() {
    const btnHistory = document.getElementById("btn-history");
    const drawer = document.getElementById("history-drawer");
    const backdrop = document.getElementById("drawer-backdrop");
    const closeDrawer = document.getElementById("close-history-drawer");
    const clearHistory = document.getElementById("btn-clear-history");

    if (btnHistory) {
        btnHistory.addEventListener("click", () => {
            renderHistoryList();
            drawer.classList.remove("translate-x-full");
            if (backdrop) backdrop.classList.remove("hidden");
        });
    }

    if (closeDrawer) {
        closeDrawer.addEventListener("click", closeHistoryDrawer);
    }

    if (clearHistory) {
        clearHistory.addEventListener("click", () => {
            if (confirm("Hapus seluruh riwayat pesanan dari browser ini?")) {
                Storage.clear();
                renderHistoryList();
            }
        });
    }
}

function closeHistoryDrawer() {
    const drawer = document.getElementById("history-drawer");
    const backdrop = document.getElementById("drawer-backdrop");
    if (drawer) drawer.classList.add("translate-x-full");
    if (backdrop) backdrop.classList.add("hidden");
}

// 2. Update Badge Jumlah Riwayat
function updateHistoryBadge() {
    const badge = document.getElementById("history-badge");
    const orders = Storage.getOrders();
    if (badge) {
        badge.textContent = orders.length;
        badge.style.display = orders.length > 0 ? "inline-flex" : "none";
    }
}

// 3. Render Riwayat di Drawer
function renderHistoryList() {
    const container = document.getElementById("history-items-container");
    const orders = Storage.getOrders();

    if (!container) return;

    if (orders.length === 0) {
        container.innerHTML = `
            <div class="text-center py-12 text-slate-400">
                <span class="text-4xl block mb-2">📭</span>
                <p class="text-sm font-semibold text-slate-300">Belum ada riwayat pesanan</p>
                <p class="text-xs text-slate-500 mt-1">Pesanan Anda di browser ini akan tercatat otomatis di sini.</p>
            </div>
        `;
        return;
    }

    container.innerHTML = orders.map(order => {
        let badgeColor = "bg-amber-500/10 text-amber-400 border-amber-500/20";
        let statusText = "Menunggu Bayar";

        if (order.status === "PAID") {
            badgeColor = "bg-emerald-500/10 text-emerald-400 border-emerald-500/20";
            statusText = "Lunas (Selesai)";
        } else if (order.status === "EXPIRED") {
            badgeColor = "bg-slate-500/10 text-slate-400 border-slate-500/20";
            statusText = "Kedaluwarsa";
        }

        const dateStr = order.created_at ? new Date(order.created_at).toLocaleDateString("id-ID", {
            day: "numeric", month: "short", hour: "2-digit", minute: "2-digit"
        }) : "-";

        return `
            <div class="p-4 rounded-2xl bg-slate-800/60 border border-slate-700/60 flex flex-col gap-2 hover:border-slate-600 transition">
                <div class="flex items-center justify-between text-xs">
                    <span class="font-mono text-slate-400 font-semibold">#${escapeHtml(order.id)}</span>
                    <span class="px-2.5 py-0.5 rounded-full border text-[11px] font-medium ${badgeColor}">${statusText}</span>
                </div>
                <div class="font-bold text-slate-100 text-sm line-clamp-1">${escapeHtml(order.product_name)}</div>
                <div class="flex items-center justify-between mt-1 text-xs">
                    <span class="text-slate-400">${dateStr}</span>
                    <span class="font-bold text-emerald-400">${Storage.formatRupiah(order.total_amount)}</span>
                </div>
                <a href="/order/?id=${encodeURIComponent(order.id)}&token=${encodeURIComponent(order.token)}" 
                   class="mt-2 text-center py-2 px-3 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 hover:text-blue-300 border border-blue-500/30 rounded-xl text-xs font-semibold transition">
                   ${order.status === 'PAID' ? '🎁 Buka Produk' : '💳 Bayar Sekarang'}
                </a>
            </div>
        `;
    }).join("");
}

// 4. Memuat Produk dari Backend
async function loadProducts() {
    const grid = document.getElementById("products-grid");
    const loading = document.getElementById("products-loading");

    try {
        const res = await fetch(`${API_BASE}/api/products`);
        const json = await res.json();

        if (loading) loading.style.display = "none";

        if (json.success && Array.isArray(json.data)) {
            allProducts = json.data;
            renderCategories();
            renderProductGrid();
        } else {
            grid.innerHTML = `<div class="col-span-full text-center py-12 text-slate-400">Gagal memuat produk.</div>`;
        }
    } catch (err) {
        console.error("Fetch error:", err);
        if (loading) loading.style.display = "none";
        grid.innerHTML = `<div class="col-span-full text-center py-12 text-red-400">Tidak dapat terhubung ke server toko.</div>`;
    }
}

// 5. Render Kategori Pills
function renderCategories() {
    const container = document.getElementById("category-filters");
    if (!container) return;

    const categories = ["ALL", ...new Set(allProducts.map(p => p.category).filter(Boolean))];

    container.innerHTML = categories.map(cat => {
        const isSelected = selectedCategory === cat;
        const activeClass = isSelected
            ? "bg-blue-600 text-white font-semibold shadow-lg shadow-blue-500/20 border-blue-500"
            : "bg-slate-800/80 text-slate-300 hover:bg-slate-700/80 border-slate-700/60";

        const label = cat === "ALL" ? "Semua Produk" : cat;
        return `
            <button onclick="filterCategory('${escapeHtml(cat)}')" 
                    class="px-4 py-2 rounded-xl text-xs sm:text-sm border transition whitespace-nowrap ${activeClass} cursor-pointer">
                ${escapeHtml(label)}
            </button>
        `;
    }).join("");
}

function handleSearch(val) {
    searchQuery = (val || "").trim().toLowerCase();

    const clearBtn = document.getElementById("btn-clear-search");
    if (clearBtn) {
        if (searchQuery) {
            clearBtn.classList.remove("hidden");
        } else {
            clearBtn.classList.add("hidden");
        }
    }

    renderProductGrid();
}

function triggerSearch() {
    const input = document.getElementById("product-search");
    if (input) {
        handleSearch(input.value);
    }
}

function clearSearch() {
    const input = document.getElementById("product-search");
    if (input) {
        input.value = "";
        handleSearch("");
        input.focus();
    }
}

function filterCategory(cat) {
    selectedCategory = cat;
    renderCategories();
    renderProductGrid();
}

// 6. Render Grid Produk (Setiap Kartu Direct ke Halaman /product/?id=...)
function renderProductGrid() {
    const grid = document.getElementById("products-grid");
    if (!grid) return;

    let filtered = selectedCategory === "ALL"
        ? allProducts
        : allProducts.filter(p => p.category === selectedCategory);

    if (searchQuery) {
        const inCurrentCat = filtered.filter(p => 
            (p.name && p.name.toLowerCase().includes(searchQuery)) ||
            (p.description && p.description.toLowerCase().includes(searchQuery)) ||
            (p.category && p.category.toLowerCase().includes(searchQuery))
        );

        if (inCurrentCat.length === 0 && selectedCategory !== "ALL") {
            const inAll = allProducts.filter(p => 
                (p.name && p.name.toLowerCase().includes(searchQuery)) ||
                (p.description && p.description.toLowerCase().includes(searchQuery)) ||
                (p.category && p.category.toLowerCase().includes(searchQuery))
            );
            if (inAll.length > 0) {
                selectedCategory = "ALL";
                renderCategories();
                filtered = allProducts;
            }
        }

        filtered = filtered.filter(p => 
            (p.name && p.name.toLowerCase().includes(searchQuery)) ||
            (p.description && p.description.toLowerCase().includes(searchQuery)) ||
            (p.category && p.category.toLowerCase().includes(searchQuery))
        );
    }

    if (filtered.length === 0) {
        grid.innerHTML = `
            <div class="col-span-full text-center py-16 text-slate-400">
                <span class="text-5xl block mb-3">🔍</span>
                <p class="font-bold text-slate-200 text-base">Tidak ada produk yang cocok ${searchQuery ? `dengan "${escapeHtml(searchQuery)}"` : ''}</p>
                <p class="text-xs text-slate-400 mt-1 mb-4">Coba gunakan kata kunci lain atau bersihkan pencarian.</p>
                ${searchQuery ? `
                    <button onclick="clearSearch()" class="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-blue-400 text-xs font-bold transition border border-slate-700 cursor-pointer">
                        ✕ Reset Pencarian
                    </button>
                ` : ''}
            </div>
        `;
        return;
    }

    grid.innerHTML = filtered.map(p => {
        const isOutOfStock = p.stock_count <= 0;
        const stockBadge = isOutOfStock
            ? `<span class="px-2.5 py-1 rounded-md bg-red-500/10 text-red-400 border border-red-500/20 text-xs font-semibold">Habis</span>`
            : (p.delivery_type === 'fixed' 
                ? `<span class="px-2.5 py-1 rounded-md bg-blue-500/10 text-blue-400 border border-blue-500/20 text-xs font-semibold">Tersedia</span>`
                : `<span class="px-2.5 py-1 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-semibold">Stok: ${p.stock_count}</span>`);

        const imgHtml = p.image_url && p.image_url.trim()
            ? `<img src="${escapeHtml(typeof formatImageUrl === "function" ? formatImageUrl(p.image_url) : p.image_url)}" alt="${escapeHtml(p.name)}" class="w-full h-48 object-cover rounded-t-3xl group-hover:scale-105 transition duration-300">`
            : `<div class="w-full h-48 bg-gradient-to-br from-blue-950/80 to-indigo-950/80 flex items-center justify-center rounded-t-3xl text-5xl group-hover:scale-105 transition duration-300 text-blue-400">⚡</div>`;

        return `
            <a href="/product/?id=${encodeURIComponent(p.id)}" 
               class="glass-card rounded-3xl flex flex-col justify-between overflow-hidden cursor-pointer group hover:border-blue-500/40 hover:shadow-2xl hover:shadow-blue-500/10 transition block">
                <div>
                    <div class="relative overflow-hidden">
                        ${imgHtml}
                        <div class="absolute top-3 left-3">
                            <span class="px-3 py-1 rounded-xl bg-slate-900/90 backdrop-blur-md border border-slate-700/80 text-slate-200 text-xs font-semibold">
                                ${escapeHtml(p.category)}
                            </span>
                        </div>
                        <div class="absolute top-3 right-3">
                            ${stockBadge}
                        </div>
                    </div>
                    <div class="p-6">
                        <h3 class="font-bold text-slate-100 text-lg leading-snug line-clamp-1 group-hover:text-blue-400 transition">${escapeHtml(p.name)}</h3>
                        <p class="text-slate-400 text-xs mt-2 line-clamp-2 leading-relaxed">${escapeHtml(p.description || "Pengiriman otomatis instan setelah pembayaran QRIS terkonfirmasi.")}</p>
                    </div>
                </div>
                <div class="p-6 pt-0">
                    <div class="pt-4 border-t border-slate-800 flex items-center justify-between">
                        <div>
                            <span class="text-[10px] text-slate-400 uppercase font-semibold block">Harga</span>
                            <span class="text-xl font-black text-emerald-400">${Storage.formatRupiah(p.price)}</span>
                        </div>
                        <div class="px-4 py-2.5 rounded-xl bg-blue-600/10 group-hover:bg-blue-600 text-blue-400 group-hover:text-white border border-blue-500/20 group-hover:border-blue-500 font-bold text-xs transition flex items-center gap-1">
                            <span>Lihat Detail →</span>
                        </div>
                    </div>
                </div>
            </a>
        `;
    }).join("");
}

function escapeHtml(text) {
    if (!text && text !== 0) return "";
    return String(text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

// 7. Modal Lacak Pesanan
function openTrackModal() {
    const modal = document.getElementById("track-modal-backdrop");
    const err = document.getElementById("track-error");
    if (err) err.classList.add("hidden");
    if (modal) modal.classList.remove("hidden");
    const idInput = document.getElementById("track-order-id");
    if (idInput) setTimeout(() => idInput.focus(), 50);
}

function closeTrackModal() {
    const modal = document.getElementById("track-modal-backdrop");
    if (modal) modal.classList.add("hidden");
}

async function submitTrackOrder(e) {
    e.preventDefault();
    const orderID = (document.getElementById("track-order-id").value || "").trim();
    const email = (document.getElementById("track-customer-email").value || "").trim();
    const errEl = document.getElementById("track-error");
    const btn = document.getElementById("btn-submit-track");

    if (!orderID || !email) {
        if (errEl) {
            errEl.textContent = "Invoice ID dan Email wajib diisi.";
            errEl.classList.remove("hidden");
        }
        return;
    }

    btn.disabled = true;
    btn.innerHTML = `<span class="inline-block animate-spin mr-2">⟳</span> Mencari pesanan...`;
    if (errEl) errEl.classList.add("hidden");

    try {
        const res = await fetch(`${API_BASE}/api/orders/track`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ order_id: orderID, customer_email: email })
        });
        const json = await res.json();

        if (json.success && json.data) {
            // Simpan juga ke local storage pembeli jika belum ada
            Storage.saveOrder({
                id: json.data.order_id,
                token: json.data.secret_token,
                product_name: json.data.product_name,
                total_amount: json.data.total_amount,
                status: json.data.status,
                created_at: json.data.created_at
            });

            // Redirect langsung ke halaman klaim pesanan
            window.location.href = `/order/?id=${encodeURIComponent(json.data.order_id)}&token=${encodeURIComponent(json.data.secret_token)}`;
        } else {
            if (errEl) {
                errEl.textContent = json.error || "Pesanan tidak ditemukan atau email tidak cocok.";
                errEl.classList.remove("hidden");
            }
        }
    } catch (err) {
        if (errEl) {
            errEl.textContent = "Gagal menghubungi server. Periksa koneksi internet Anda.";
            errEl.classList.remove("hidden");
        }
    } finally {
        btn.disabled = false;
        btn.textContent = "Cari & Buka Pesanan";
    }
}
