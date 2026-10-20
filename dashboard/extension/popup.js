// PaymentG Shopee Token Sync - Popup Script

document.addEventListener("DOMContentLoaded", () => {
  const statusDot = document.getElementById("status-dot");
  const statusText = document.getElementById("status-text");
  const tokenBox = document.getElementById("token-box");
  const cfgUrl = document.getElementById("cfg-url");
  const cfgKey = document.getElementById("cfg-key");
  const cfgAuto = document.getElementById("cfg-autosync");
  const btnSync = document.getElementById("btn-sync");
  const alertBox = document.getElementById("alert-box");

  let detectedToken = null;

  function showAlert(msg, isSuccess = true) {
    alertBox.className = `alert ${isSuccess ? "success" : "error"}`;
    alertBox.innerText = msg;
    alertBox.style.display = "block";
    setTimeout(() => { alertBox.style.display = "none"; }, 5000);
  }

  // Muat status dari background worker
  chrome.runtime.sendMessage({ action: "GET_STATUS" }, (res) => {
    if (!res) return;
    const cfg = res.config || {};
    cfgUrl.value = cfg.vpsUrl || "https://paymentg.syukurapi.online";
    cfgKey.value = cfg.adminKey || "pg_ZZio-WwakXZLozuUvjphxYAh_L9MnnazjPbhakBwKDE";
    cfgAuto.checked = cfg.autoSync !== false;

    if (res.detectedToken && res.detectedToken.startsWith("B:")) {
      detectedToken = res.detectedToken;
      statusDot.className = "dot green";
      statusText.innerText = "Sesi Shopee Terdeteksi";
      tokenBox.innerText = `${detectedToken.substring(0, 36)}...`;
      tokenBox.title = detectedToken;
    } else {
      statusDot.className = "dot red";
      statusText.innerText = "Belum Terdeteksi";
      tokenBox.innerText = "Buka partner.shopee.co.id & login";
    }
  });

  // Simpan pengaturan otomatis saat input diubah
  function saveConfig() {
    chrome.storage.local.set({
      vpsUrl: cfgUrl.value.trim().replace(/\/$/, ""),
      adminKey: cfgKey.value.trim(),
      autoSync: cfgAuto.checked
    });
  }

  cfgUrl.addEventListener("input", saveConfig);
  cfgKey.addEventListener("input", saveConfig);
  cfgAuto.addEventListener("change", saveConfig);

  // Tombol Sync Sekarang
  btnSync.addEventListener("click", () => {
    saveConfig();
    btnSync.disabled = true;
    btnSync.innerHTML = "<span>⏳</span> Memvalidasi & Mengirim...";

    chrome.runtime.sendMessage({
      action: "SYNC_NOW",
      token: detectedToken,
      config: {
        vpsUrl: cfgUrl.value.trim().replace(/\/$/, ""),
        adminKey: cfgKey.value.trim()
      }
    }, (res) => {
      btnSync.disabled = false;
      btnSync.innerHTML = "<span>⚡</span> Sinkronkan Token Sekarang";

      if (res && res.success) {
        showAlert("✅ " + (res.data || "Berhasil disinkronkan ke VPS!"), true);
      } else {
        showAlert("❌ " + ((res && res.error) || "Gagal menghubungi VPS"), false);
      }
    });
  });
});
