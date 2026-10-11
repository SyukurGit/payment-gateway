import os
import sys
import re
import json
import base64
import time
from pathlib import Path
import httpx
from playwright.sync_api import sync_playwright

# Force UTF-8 output on Windows console
if sys.platform == "win32":
    os.environ.setdefault("PYTHONIOENCODING", "utf-8")
    try:
        sys.stdout.reconfigure(encoding="utf-8")
        sys.stderr.reconfigure(encoding="utf-8")
    except Exception:
        pass

# 1. Load config from .env
env_path = Path(__file__).parent / ".env"
config = {}
if env_path.exists():
    for line in env_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            config[k.strip()] = v.strip()

PORT = config.get("PORT", "3200")
ADMIN_KEY = config.get("ADMIN_KEY", "adm_secret_paymentg_2026")
PAYMENTG_API = f"http://localhost:{PORT}/api/config/token"
PROFILE_DIR = os.path.join(os.path.dirname(__file__), "data", "browser_profile")
SHOPEE_URL = "https://partner.shopee.co.id/shopeepay-portal/transactions"

# Konfigurasi VPS untuk auto-sync
VPS_URL = config.get("VPS_URL", "https://paymentg.syukurapi.online")
VPS_ADMIN_KEY = config.get("VPS_ADMIN_KEY", "")


def extract_token_from_jwt(jwt_str: str):
    """Mendekode payload JWT cookie Shopee untuk mendapatkan B:token."""
    try:
        parts = jwt_str.split(".")
        if len(parts) >= 2:
            payload = parts[1]
            payload += "=" * ((4 - len(payload) % 4) % 4)
            data = json.loads(base64.urlsafe_b64decode(payload))
            if "token" in data and str(data["token"]).startswith("B:"):
                return str(data["token"]).strip()
    except Exception:
        pass
    return None


def extract_token_from_browser(context, page):
    """Ekstrak token Shopee baik dari Cookie modern, injectData, maupun localStorage."""
    # 1. Cek Cookie JWT modern (Shopee Partner v2024+)
    try:
        for c in context.cookies():
            val = c.get("value", "")
            if val.startswith("B:"):
                return val
            if "token" in c.get("name", "").lower():
                jwt_tok = extract_token_from_jwt(val)
                if jwt_tok:
                    return jwt_tok
    except Exception:
        pass

    # 2. Cek window.injectData (Shopee Partner legacy)
    try:
        tok = page.evaluate("() => (window.injectData && window.injectData.User && window.injectData.User.token) || null")
        if tok and str(tok).startswith("B:"):
            return str(tok).strip()
    except Exception:
        pass

    # 3. Cek localStorage
    try:
        ls_tokens = page.evaluate("""() => {
            const keys = Object.keys(localStorage);
            for (const k of keys) {
                const val = localStorage.getItem(k);
                if (val && val.includes('"token":"B:')) {
                    try {
                        const parsed = JSON.parse(val);
                        if (parsed.token && parsed.token.startsWith('B:')) return parsed.token;
                    } catch(e) {}
                }
            }
            return null;
        }""")
        if ls_tokens and str(ls_tokens).startswith("B:"):
            return str(ls_tokens).strip()
    except Exception:
        pass

    return None


def update_env_token(new_token):
    """Update SHOPEE_TOKEN di .env file lokal."""
    if not env_path.exists():
        print("[-] .env file tidak ditemukan, skip simpan ke .env")
        return False

    content = env_path.read_text(encoding="utf-8")

    # Ganti baris SHOPEE_TOKEN=... yang sudah ada
    if re.search(r"^SHOPEE_TOKEN=.*$", content, re.MULTILINE):
        content = re.sub(
            r"^SHOPEE_TOKEN=.*$",
            f"SHOPEE_TOKEN={new_token}",
            content,
            flags=re.MULTILINE,
        )
    else:
        # Tambahkan jika belum ada
        content = content.rstrip("\n") + f"\nSHOPEE_TOKEN={new_token}\n"

    env_path.write_text(content, encoding="utf-8")
    print("[+] File .env lokal berhasil diperbarui dengan token baru!")
    return True


def sync_to_vps(token):
    """Kirim token ke VPS via HTTPS API."""
    if not VPS_ADMIN_KEY:
        print("[!] VPS_ADMIN_KEY kosong di .env, skip sync ke VPS")
        return False

    print(f"[*] Sync ke VPS: {VPS_URL}...")
    try:
        res = httpx.put(
            f"{VPS_URL}/api/config/token",
            headers={"X-Admin-Key": VPS_ADMIN_KEY, "Content-Type": "application/json"},
            json={"token": token},
            timeout=15,
        )
        if res.status_code == 200:
            print("[+] OK! Token berhasil disinkronkan ke VPS!")
            # Verifikasi health
            try:
                health = httpx.get(f"{VPS_URL}/api/health", timeout=10)
                if health.status_code == 200:
                    data = health.json()
                    if data.get("success") and data.get("data", {}).get("token_valid"):
                        print("[+] OK! VPS health: token_valid = true (Siap terima transaksi)")
                    else:
                        detail = data.get("data", {}).get("token_error", "N/A")
                        print(f"[!] WARN: VPS health: token_valid = false ({detail})")
            except Exception:
                pass
            return True
        else:
            print(f"[-] GAGAL sync VPS: HTTP {res.status_code} {res.text}")
            return False
    except Exception as e:
        print(f"[-] GAGAL sync VPS: {e}")
        return False


def refresh_token(headless=True):
    print(f"[*] Menjalankan Playwright Chromium (Headless: {headless})...")
    os.makedirs(PROFILE_DIR, exist_ok=True)

    with sync_playwright() as p:
        args = [
            "--disable-blink-features=AutomationControlled",
            "--disable-infobars",
            "--no-sandbox",
            "--disable-setuid-sandbox",
            "--disable-dev-shm-usage",
            "--disable-gpu",
            "--no-first-run",
            "--no-default-browser-check"
        ]

        context = p.chromium.launch_persistent_context(
            user_data_dir=PROFILE_DIR,
            headless=headless,
            args=args,
            viewport={"width": 1280, "height": 720},
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        )

        page = context.pages[0] if context.pages else context.new_page()

        # Samarkan webdriver
        page.add_init_script("Object.defineProperty(navigator, 'webdriver', {get: () => undefined});")

        print(f"[*] Mengakses {SHOPEE_URL}...")
        try:
            page.goto(SHOPEE_URL, wait_until="domcontentloaded", timeout=45000)
        except Exception as e:
            print(f"[-] Warning page load: {e}")

        token = None
        # Tunggu halaman dan cek token
        max_attempts = 18 if not headless else 4
        for attempt in range(max_attempts):
            time.sleep(3 if not headless else 2)
            token = extract_token_from_browser(context, page)
            if token:
                break
            if attempt == 1 and not headless:
                print("\n[!] Menunggu login... Silakan masukkan OTP di jendela browser jika diminta.")

        context.close()

        if token:
            print(f"[+] Token berhasil didapatkan: {token[:32]}...")

            # 1. Simpan token baru ke .env lokal
            update_env_token(token)

            # 2. Update database lokal via localhost API
            try:
                res = httpx.put(
                    PAYMENTG_API,
                    headers={"X-Admin-Key": ADMIN_KEY, "Content-Type": "application/json"},
                    json={"token": token},
                    timeout=5
                )
                if res.status_code == 200:
                    print("[+] Token berhasil diperbarui ke PaymentG database lokal!")
            except Exception:
                pass

            # 3. Sync token ke VPS
            sync_to_vps(token)
            return True
        else:
            print("[-] Gagal mendapatkan token ShopeePay.")
            return False


if __name__ == "__main__":
    is_setup = "--setup" in sys.argv
    refresh_token(headless=not is_setup)
