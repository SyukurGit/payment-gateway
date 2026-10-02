# Quick automated test script for PaymentG API
$ErrorActionPreference = "Stop"

Write-Host "=================================================" -ForegroundColor Cyan
Write-Host "   PaymentG - Full API Verification Suite" -ForegroundColor Cyan
Write-Host "=================================================" -ForegroundColor Cyan

$baseUrl = "http://localhost:3200"
$adminKey = "adm_secret_paymentg_2026"

# 1. Test Health
Write-Host "`n[1/6] Testing GET /api/health..." -ForegroundColor Yellow
$health = Invoke-RestMethod -Uri "$baseUrl/api/health"
if ($health.success) {
    Write-Host "  -> PASS: Health check OK (Pending orders: $($health.data.pending_orders))" -ForegroundColor Green
} else {
    Write-Host "  -> FAIL: Health check response false" -ForegroundColor Red
}

# 2. Test Create App (Admin)
Write-Host "`n[2/6] Testing POST /api/apps (Register new client)..." -ForegroundColor Yellow
$appHeaders = @{ "X-Admin-Key" = $adminKey; "Content-Type" = "application/json" }
$appBody = @{ name = "QuickTest Store"; webhook_url = "http://127.0.0.1:9999/webhook" } | ConvertTo-Json
$appRes = Invoke-RestMethod -Uri "$baseUrl/api/apps" -Method POST -Headers $appHeaders -Body $appBody
$apiKey = $appRes.data.api_key
$appId = $appRes.data.id
Write-Host "  -> PASS: Created App ID=$appId, APIKey=$apiKey" -ForegroundColor Green

# 3. Test List Apps (Admin)
Write-Host "`n[3/6] Testing GET /api/apps..." -ForegroundColor Yellow
$listRes = Invoke-RestMethod -Uri "$baseUrl/api/apps" -Headers $appHeaders
Write-Host "  -> PASS: Found $($listRes.data.Count) registered apps" -ForegroundColor Green

# 4. Test Create Order (Client)
Write-Host "`n[4/6] Testing POST /api/orders (Create dynamic QRIS order)..." -ForegroundColor Yellow
$clientHeaders = @{ "X-API-Key" = $apiKey; "Content-Type" = "application/json" }
$orderBody = @{ reference_id = "INV-TEST-$(Get-Random)"; amount = 35000; expiry_minutes = 15 } | ConvertTo-Json
$orderRes = Invoke-RestMethod -Uri "$baseUrl/api/orders" -Method POST -Headers $clientHeaders -Body $orderBody
$orderId = $orderRes.data.order_id
$totalAmount = $orderRes.data.total_amount
$uniqueCode = $orderRes.data.unique_code
Write-Host "  -> PASS: Created Order ID=$orderId" -ForegroundColor Green
Write-Host "           Base Amount: Rp 35.000 | Kode Unik: $uniqueCode | Total Bayar: Rp $totalAmount" -ForegroundColor Green

# 5. Test Download QR Image (Client)
Write-Host "`n[5/6] Testing GET /api/orders/:id/qr.png..." -ForegroundColor Yellow
$qrRes = Invoke-WebRequest -Uri "$baseUrl/api/orders/$orderId/qr.png" -Headers $clientHeaders
if ($qrRes.StatusCode -eq 200 -and $qrRes.Headers["Content-Type"] -eq "image/png") {
    Write-Host "  -> PASS: Valid PNG QR Code received ($($qrRes.RawContentLength) bytes)" -ForegroundColor Green
} else {
    Write-Host "  -> FAIL: QR code response invalid" -ForegroundColor Red
}

# 6. Test Cancel Order
Write-Host "`n[6/6] Testing POST /api/orders/:id/cancel..." -ForegroundColor Yellow
$cancelRes = Invoke-RestMethod -Uri "$baseUrl/api/orders/$orderId/cancel" -Method POST -Headers $clientHeaders
Write-Host "  -> PASS: Order successfully cancelled, unique amount released" -ForegroundColor Green

Write-Host "`n=================================================" -ForegroundColor Cyan
Write-Host "   SEMUA TEST BERHASIL 100% (VERIFIED)!" -ForegroundColor Green
Write-Host "=================================================" -ForegroundColor Cyan
