@echo off
setlocal EnableDelayedExpansion

echo =========================================================
echo    PaymentG - Setup ShopeePay Auto-Sync Scheduler
echo =========================================================
echo.

set "SCRIPT_DIR=%~dp0"
set "VBS_PATH=%SCRIPT_DIR%scripts\run_silent_sync.vbs"
set "TASK_NAME=PaymentG_Shopee_AutoSync"

if not exist "%VBS_PATH%" (
    echo [ERROR] File tidak ditemukan: %VBS_PATH%
    pause
    exit /b 1
)

echo [*] Mendaftarkan Task Scheduler Windows...
echo     Nama Task : %TASK_NAME%
echo     Target    : %VBS_PATH%
echo     Frekuensi : Setiap 4 jam + saat login Windows
echo.

:: Hapus task lama jika sudah ada
schtasks /delete /tn "%TASK_NAME%" /f >nul 2>&1

:: Daftarkan task baru yang berjalan setiap 4 jam
schtasks /create /tn "%TASK_NAME%" /tr "wscript.exe \"%VBS_PATH%\"" /sc minute /mo 240 /f
if %ERRORLEVEL% equ 0 (
    echo [+] SUKSES! Task Scheduler berhasil didaftarkan.
    echo [*] Mencoba menjalankan task untuk pertama kali...
    schtasks /run /tn "%TASK_NAME%"
    echo.
    echo =========================================================
    echo  Token ShopeePay sekarang akan diperbarui otomatis
    echo  setiap 4 jam di latar belakang (tanpa jendela pop-up).
    echo =========================================================
) else (
    echo [-] GAGAL mendaftarkan task. Pastikan dijalankan sebagai Administrator jika diminta.
)

echo.
pause
