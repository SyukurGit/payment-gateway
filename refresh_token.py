import os
import sys
from pathlib import Path
import httpx
from playwright.sync_api import sync_playwright

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

def refresh_token(headless=True):
    print(f"[*] Menjalankan Playwright Chromium (Headless: {headless})...")
    os.makedirs(PROFILE_DIR, exist_ok=True)

    with sync_playwright() as p:
        # Args optimasi VPS: hemat RAM + bypass deteksi bot
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

        # Samarkan navigator.webdriver
        page.add_init_script("""
            Object.defineProperty(navigator, 'webdriver', {
                get: () => undefined
            });
        """)

        print(f"[*] Mengakses {SHOPEE_URL}...")
        try:
            page.goto(SHOPEE_URL, wait_until="domcontentloaded", timeout=45000)
        except Exception as e:
            print(f"[-] Warning page load: {e}")

        token = None
        try:
            # Tunggu window.injectData disuntikkan oleh frontend Shopee
            page.wait_for_function(
                "() => window.injectData && window.injectData.User && window.injectData.User.token",
                timeout=12000
            )
            token = page.evaluate("() => window.injectData.User.token")
        except Exception:
            if not headless:
                print("\n[!] Sesi belum aktif. Silakan login & masukkan OTP di jendela browser...")
                try:
                    page.wait_for_function(
                        "() => window.injectData && window.injectData.User && window.injectData.User.token",
                        timeout=180000
                    )
                    token = page.evaluate("() => window.injectData.User.token")
                except Exception as e:
                    print(f"[-] Timeout menunggu login: {e}")
            else:
                print("[-] Token tidak ditemukan secara headless (profil mungkin belum login atau session habis).")

        # Langsung tutup browser agar RAM VPS kembali bebas 0 MB
        context.close()

        if token:
            print(f"[+] Token berhasil didapatkan: {token[:32]}...")
            try:
                res = httpx.put(
                    PAYMENTG_API,
                    headers={"X-Admin-Key": ADMIN_KEY, "Content-Type": "application/json"},
                    json={"token": token},
                    timeout=10
                )
                if res.status_code == 200:
                    print("[+] Token berhasil diperbarui ke PaymentG database!")
                    return True
                else:
                    print(f"[-] Gagal update ke API: {res.status_code} {res.text}")
            except Exception as e:
                print(f"[-] Gagal menghubungi PaymentG API: {e}")
        else:
            print("[-] Gagal memperbarui token.")
        return False

if __name__ == "__main__":
    # Gunakan --setup saat pertama kali login jika ingin tampilan GUI
    is_setup = "--setup" in sys.argv
    refresh_token(headless=not is_setup)
