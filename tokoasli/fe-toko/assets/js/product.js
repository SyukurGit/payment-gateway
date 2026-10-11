// Logic Halaman Detail Produk (PDP) TokoAsli
let currentProduct = null;

document.addEventListener("DOMContentLoaded", () => {
    const params = new URLSearchParams(window.location.search);
    const productId = params.get("id");

    if (!productId) {
        showError("Parameter ID produk tidak ditemukan di URL.");
        return;
    }

    loadProductDetail(productId);
});

async function loadProductDetail(id) {
    const loadingEl = document.getElementById("product-loading");
    const errorEl = document.getElementById("product-error");
    const contentEl = document.getElementById("product-content");

    try {
        const res = await fetch(`${API_BASE}/api/products/${encodeURIComponent(id)}`);
        const json = await res.json();

        if (!res.ok || !json.success || !json.data) {
            throw new Error(json.error || "Produk tidak ditemukan atau sudah non-aktif.");
        }

        currentProduct = json.data;
        renderProductData(currentProduct);

        loadingEl.classList.add("hidden");
        errorEl.classList.add("hidden");
        contentEl.classList.remove("hidden");

        // Update page title
        document.title = `${currentProduct.name} - TokoAsli`;

        // Update WhatsApp widget kontekstual
        if (typeof injectWhatsAppWidget === "function") {
            const el = document.getElementById("tokoasli-wa-float");
            if (el) el.remove();
            injectWhatsAppWidget(`Halo Admin TokoAsli, saya ingin bertanya tentang produk: ${currentProduct.name} (Harga: ${Storage.formatRupiah(currentProduct.price)}).`);
        }

    } catch (err) {
        console.error("Load product error:", err);
        loadingEl.classList.add("hidden");
        contentEl.classList.add("hidden");
        showError(err.message);
    }
}

function renderProductData(p) {
    // Breadcrumb
    document.getElementById("breadcrumb-category").textContent = p.category || "Umum";
    document.getElementById("breadcrumb-title").textContent = p.name;

    // Foto / Gambar Produk
    const imgEl = document.getElementById("prod-img");
    const placeholderEl = document.getElementById("prod-img-placeholder");

    if (p.image_url && p.image_url.trim()) {
        imgEl.src = (typeof formatImageUrl === "function" ? formatImageUrl(p.image_url) : p.image_url);
        imgEl.classList.remove("hidden");
        placeholderEl.classList.add("hidden");
    } else {
        imgEl.classList.add("hidden");
        placeholderEl.classList.remove("hidden");
    }

    // Nama & Kategori
    document.getElementById("prod-name").textContent = p.name;
    document.getElementById("prod-category-badge").textContent = p.category || "Umum";
    document.getElementById("prod-price").textContent = Storage.formatRupiah(p.price);

    // Deskripsi
    const descEl = document.getElementById("prod-description");
    if (p.description && p.description.trim()) {
        descEl.textContent = p.description;
    } else {
        descEl.textContent = "Produk digital resmi dengan aktivasi instan. Akun atau lisensi akan langsung dikirimkan ke email Anda begitu transaksi QRIS diverifikasi.";
    }

    // Badge Stok & Status Tombol
    const stockBadge = document.getElementById("prod-stock-badge");
    const btnSubmit = document.getElementById("btn-submit-order");
    const isOutOfStock = p.stock_count <= 0;

    if (isOutOfStock) {
        stockBadge.className = "px-3 py-1 rounded-lg bg-red-500/10 text-red-400 border border-red-500/20 text-xs font-semibold";
        stockBadge.textContent = "Stok Habis";

        btnSubmit.disabled = true;
        btnSubmit.className = "w-full py-4 rounded-2xl bg-slate-800 text-slate-500 font-bold text-sm cursor-not-allowed border border-slate-700 flex items-center justify-center gap-2 mt-2";
        btnSubmit.innerHTML = `<span>Stok Sedang Habis</span>`;
    } else {
        stockBadge.className = "px-3 py-1 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-semibold";
        stockBadge.textContent = p.delivery_type === "fixed" ? "● Unlimited (Link)" : `● Sisa Stok: ${p.stock_count} Akun`;

        btnSubmit.disabled = false;
        btnSubmit.className = "w-full py-4 rounded-2xl bg-blue-600 hover:bg-blue-500 text-white font-black text-sm transition shadow-xl shadow-blue-500/25 active:scale-95 flex items-center justify-center gap-2 cursor-pointer mt-2";
        btnSubmit.innerHTML = `<span>💳 Beli Sekarang via QRIS →</span>`;
    }

    // Box Ringkasan di Kolom Kanan
    document.getElementById("box-prod-name").textContent = p.name;
    document.getElementById("box-prod-category").textContent = p.category || "Umum";
    document.getElementById("box-prod-price").textContent = Storage.formatRupiah(p.price);
}

function showError(msg) {
    const errorEl = document.getElementById("product-error");
    const msgEl = document.getElementById("error-message");
    if (msgEl) msgEl.textContent = msg;
    if (errorEl) errorEl.classList.remove("hidden");
}

async function handleDirectCheckout(e) {
    e.preventDefault();
    if (!currentProduct || currentProduct.stock_count <= 0) return;

    const emailInput = document.getElementById("checkout-email");
    const errorEl = document.getElementById("checkout-error");
    const btnSubmit = document.getElementById("btn-submit-order");

    const email = emailInput.value.trim();
    if (!email || !email.includes("@") || !email.includes(".")) {
        errorEl.textContent = "Silakan masukkan alamat email yang valid.";
        errorEl.classList.remove("hidden");
        return;
    }

    const phoneInput = document.getElementById("checkout-phone");
    const phone = phoneInput ? phoneInput.value.trim() : "";

    // Loading State
    btnSubmit.disabled = true;
    btnSubmit.innerHTML = `<span class="inline-block animate-spin mr-2">⏳</span> Menyiapkan QRIS Dinamis...`;
    errorEl.classList.add("hidden");

    try {
        const res = await fetch(`${API_BASE}/api/orders`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                product_id: currentProduct.id,
                customer_email: email,
                customer_phone: phone
            })
        });

        const json = await res.json();
        if (!res.ok || !json.success) {
            throw new Error(json.error || "Gagal membuat pesanan.");
        }

        const data = json.data;

        // Simpan ke riwayat localStorage browser
        Storage.saveOrder({
            id: data.order_id,
            token: data.secret_token,
            product_name: data.product_name,
            total_amount: data.total_amount,
            status: "PENDING"
        });

        // Langsung redirect ke halaman pembayaran
        window.location.href = `/order/?id=${encodeURIComponent(data.order_id)}&token=${encodeURIComponent(data.secret_token)}`;

    } catch (err) {
        console.error("Checkout error:", err);
        errorEl.textContent = err.message || "Gagal memproses pesanan. Silakan coba kembali.";
        errorEl.classList.remove("hidden");
        btnSubmit.disabled = false;
        btnSubmit.innerHTML = `<span>💳 Beli Sekarang via QRIS →</span>`;
    }
}
