// Storage Helper untuk mengelola riwayat pesanan pembeli di LocalStorage browser
const STORAGE_KEY = "tokoasli_orders";

const Storage = {
    // Mengambil semua riwayat pesanan (paling baru di atas)
    getOrders() {
        try {
            const raw = localStorage.getItem(STORAGE_KEY);
            if (!raw) return [];
            const list = JSON.parse(raw);
            return Array.isArray(list) ? list : [];
        } catch (e) {
            console.error("Gagal membaca riwayat dari localStorage:", e);
            return [];
        }
    },

    // Menyimpan atau mengupdate data pesanan
    saveOrder(order) {
        if (!order || !order.id) return;
        const list = this.getOrders();
        const existingIdx = list.findIndex(item => item.id === order.id);

        const orderData = {
            id: order.id,
            token: order.token || order.secret_token || "",
            product_name: order.product_name || "Produk Digital",
            total_amount: order.total_amount || 0,
            status: order.status || "PENDING",
            created_at: order.created_at || new Date().toISOString()
        };

        if (existingIdx >= 0) {
            // Update data yang ada
            list[existingIdx] = { ...list[existingIdx], ...orderData };
        } else {
            // Tambahkan di urutan teratas
            list.unshift(orderData);
        }

        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
            // Trigger custom event agar komponen UI lain langsung update
            window.dispatchEvent(new Event("tokoasli_orders_updated"));
        } catch (e) {
            console.error("Gagal menyimpan ke localStorage:", e);
        }
    },

    // Update status pesanan
    updateStatus(orderId, status) {
        const list = this.getOrders();
        const item = list.find(o => o.id === orderId);
        if (item) {
            item.status = status;
            localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
            window.dispatchEvent(new Event("tokoasli_orders_updated"));
        }
    },

    // Menghapus seluruh riwayat
    clear() {
        localStorage.removeItem(STORAGE_KEY);
        window.dispatchEvent(new Event("tokoasli_orders_updated"));
    },

    // Helper format mata uang Rupiah
    formatRupiah(amount) {
        return new Intl.NumberFormat("id-ID", {
            style: "currency",
            currency: "IDR",
            minimumFractionDigits: 0,
            maximumFractionDigits: 0
        }).format(amount);
    }
};

// Global HTML Escaping to prevent XSS and ReferenceErrors
function escapeHtml(text) {
    if (!text && text !== 0) return "";
    return String(text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}
