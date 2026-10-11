#!/usr/bin/env python3
"""
sync_token.py — Sinkronisasi token ShopeePay dari lokal ke VPS.

Cara pakai:
  python sync_token.py                    # Baca token dari .env lokal, kirim ke VPS
  python sync_token.py "B:xxxxx..."       # Kirim token baru (otomatis update .env lokal + VPS)

Prasyarat:
  pip install httpx
"""
import sys
import os
import re
from pathlib import Path

# Force UTF-8 output on Windows console
if sys.platform == "win32":
    os.environ.setdefault("PYTHONIOENCODING", "utf-8")
    try:
        sys.stdout.reconfigure(encoding="utf-8")
        sys.stderr.reconfigure(encoding="utf-8")
    except Exception:
        pass

try:
    import httpx
except ImportError:
    print("[-] Module httpx belum terinstall. Jalankan: pip install httpx")
    sys.exit(1)

# === KONFIGURASI ===
VPS_URL = "https://paymentg.syukurapi.online"
LOCAL_ENV = Path(__file__).parent / ".env"


def load_env():
    """Baca .env lokal dan return dict."""
    config = {}
    if LOCAL_ENV.exists():
        for line in LOCAL_ENV.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                config[k.strip()] = v.strip()
    return config


def update_env_token(new_token: str):
    """Update SHOPEE_TOKEN di .env file lokal."""
    if not LOCAL_ENV.exists():
        return False
    content = LOCAL_ENV.read_text(encoding="utf-8")
    if re.search(r"^SHOPEE_TOKEN=.*$", content, re.MULTILINE):
        content = re.sub(
            r"^SHOPEE_TOKEN=.*$",
            f"SHOPEE_TOKEN={new_token}",
            content,
            flags=re.MULTILINE,
        )
    else:
        content = content.rstrip("\n") + f"\nSHOPEE_TOKEN={new_token}\n"
    LOCAL_ENV.write_text(content, encoding="utf-8")
    print("[+] File .env lokal berhasil diperbarui dengan token baru!")
    return True


def sync_local_api(token: str, local_admin_key: str, port: str = "3200"):
    """Update token ke backend lokal jika sedang running."""
    try:
        res = httpx.put(
            f"http://localhost:{port}/api/config/token",
            headers={"X-Admin-Key": local_admin_key, "Content-Type": "application/json"},
            json={"token": token},
            timeout=3,
        )
        if res.status_code == 200:
            print("[+] OK! Server lokal (localhost:3200) ikut diperbarui!")
    except Exception:
        # Server lokal tidak sedang berjalan, abaikan
        pass


def sync_token(token: str, vps_admin_key: str, local_admin_key: str = "", port: str = "3200"):
    """Kirim token ke VPS via PUT /api/config/token."""
    print(f"[*] Target VPS  : {VPS_URL}")
    print(f"[*] Token       : {token[:32]}...")

    try:
        res = httpx.put(
            f"{VPS_URL}/api/config/token",
            headers={"X-Admin-Key": vps_admin_key, "Content-Type": "application/json"},
            json={"token": token},
            timeout=15,
        )

        if res.status_code == 200:
            print("[+] OK! Token berhasil disinkronkan ke VPS!")
            
            # Coba update server lokal juga jika ada
            if local_admin_key:
                sync_local_api(token, local_admin_key, port)

            # Verifikasi: cek health endpoint VPS
            try:
                health = httpx.get(f"{VPS_URL}/api/health", timeout=10)
                if health.status_code == 200:
                    data = health.json()
                    if data.get("success") and data.get("data", {}).get("token_valid"):
                        print("[+] OK! Health check: token_valid = true (VPS siap menerima pembayaran!)")
                    else:
                        print("[!] WARN: Health check: token_valid = false (token ShopeePay kadaluarsa)")
                        detail = data.get("data", {}).get("token_error", "N/A")
                        print(f"    Detail error Shopee: {detail}")
                        print("\n[?] Panduan memperbarui token Shopee:")
                        print("    1. Jalankan Playwright GUI:  python refresh_token.py --setup")
                        print("       ATAU")
                        print("    2. Buka https://partner.shopee.co.id/ di browser, login,")
                        print("       buka F12 Console, ketik: window.injectData.User.token")
                        print('       Lalu jalankan: python sync_token.py "B:TOKEN_BARU..."')
            except Exception:
                print("[!] Tidak bisa cek endpoint /api/health (token tetap tersimpan di database VPS)")
            return True
        else:
            print(f"[-] GAGAL: HTTP {res.status_code}")
            print(f"    Response: {res.text}")
            return False

    except httpx.ConnectError:
        print(f"[-] GAGAL: Tidak bisa terhubung ke {VPS_URL}")
        print("    Pastikan VPS aktif dan domain bisa diakses.")
        return False
    except httpx.TimeoutException:
        print("[-] GAGAL: Timeout menghubungi VPS (>15 detik)")
        return False
    except Exception as e:
        print(f"[-] GAGAL: {e}")
        return False


def main():
    config = load_env()
    vps_admin_key = config.get("VPS_ADMIN_KEY") or config.get("ADMIN_KEY", "")
    local_admin_key = config.get("ADMIN_KEY", "")
    port = config.get("PORT", "3200")

    # Jika user mengoper argumen token baru
    custom_token = False
    if len(sys.argv) > 1 and sys.argv[1] not in ("--help", "-h"):
        token = sys.argv[1].strip()
        custom_token = True
    else:
        token = config.get("SHOPEE_TOKEN", "")

    if "--help" in sys.argv or "-h" in sys.argv:
        print(__doc__)
        sys.exit(0)

    if not token:
        print("[-] Token kosong!")
        print("    Isi SHOPEE_TOKEN di .env atau pass sebagai argument:")
        print('    python sync_token.py "B:xxxxx..."')
        sys.exit(1)

    if not vps_admin_key:
        print("[-] VPS_ADMIN_KEY kosong di .env!")
        sys.exit(1)

    print("=" * 52)
    print("  PaymentG Token Sync: Lokal -> VPS")
    print("=" * 52)

    if custom_token:
        update_env_token(token)

    success = sync_token(token, vps_admin_key, local_admin_key, port)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
