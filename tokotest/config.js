/**
 * TokoTest — Konfigurasi Lingkungan (ENV Statis)
 * 
 * Anda bisa mengubah URL & API Key langsung di file ini, 
 * ATAU gunakan tombol icon [⚙️ Pengaturan API] di pojok kanan atas web toko
 * untuk mengubahnya langsung dari browser (tersimpan di localStorage).
 */

window.DEFAULT_STORE_CONFIG = {
  // Mode Default: Sandbox Gateway PaymentG
  // Jika sudah siap pakai uang asli, ganti ke: "http://localhost:3200/api" atau URL VPS Anda
  API_URL: "http://localhost:3200/api/sandbox",

  // API Key Toko:
  // Dapatkan API Key di: http://localhost:3200/sandbox/#tab-apps
  // Masukkan API Key Anda di bawah ini:
  API_KEY: "ak_sbx_ganti_dengan_key_toko_anda",

  // Nama Brand Toko
  STORE_NAME: "VoucherZone Digital",

  // Mata Uang
  CURRENCY: "IDR"
};

// Ambil konfigurasi aktif dari localStorage jika pernah disimpan lewat UI web
window.STORE_CONFIG = (function() {
  const savedUrl = localStorage.getItem("tokotest_api_url");
  const savedKey = localStorage.getItem("tokotest_api_key");
  const savedName = localStorage.getItem("tokotest_store_name");

  return {
    API_URL: savedUrl || window.DEFAULT_STORE_CONFIG.API_URL,
    API_KEY: savedKey || window.DEFAULT_STORE_CONFIG.API_KEY,
    STORE_NAME: savedName || window.DEFAULT_STORE_CONFIG.STORE_NAME,
    CURRENCY: window.DEFAULT_STORE_CONFIG.CURRENCY
  };
})();
