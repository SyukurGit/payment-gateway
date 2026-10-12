# =========================================================
#   PaymentG - Setup ShopeePay Auto-Sync Scheduler (PowerShell)
# =========================================================

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$VbsPath = Join-Path $ScriptDir "scripts\run_silent_sync.vbs"
$TaskName = "PaymentG_Shopee_AutoSync"

if (-not (Test-Path $VbsPath)) {
    Write-Host "[-] File tidak ditemukan: $VbsPath" -ForegroundColor Red
    exit 1
}

Write-Host "[*] Menyiapkan Task Scheduler: $TaskName" -ForegroundColor Cyan
Write-Host "    Target: $VbsPath" -ForegroundColor Gray
Write-Host "    Interval: Setiap 4 Jam" -ForegroundColor Gray

# Hapus task lama jika ada
schtasks /delete /tn $TaskName /f 2>$null | Out-Null

# Daftarkan task baru setiap 240 menit (4 jam)
$res = schtasks /create /tn $TaskName /tr "wscript.exe `"$VbsPath`"" /sc minute /mo 240 /f

if ($LASTEXITCODE -eq 0) {
    Write-Host "[+] BERHASIL: Task Scheduler berhasil didaftarkan!" -ForegroundColor Green
    Write-Host "[*] Menguji jalankan task..." -ForegroundColor Yellow
    schtasks /run /tn $TaskName | Out-Null
    Write-Host "[+] Task diuji dan sedang berjalan di latar belakang." -ForegroundColor Green
    Write-Host "`nToken ShopeePay akan otomatis disinkronkan ke VPS setiap 4 jam tanpa pop-up." -ForegroundColor Cyan
} else {
    Write-Host "[-] Gagal mendaftarkan task. Coba jalankan PowerShell as Administrator." -ForegroundColor Red
}
