# Demo: POST /orders twice (idempotent), then GET (equivalent to: make demo)
$base = if ($env:GATEWAY_URL) { $env:GATEWAY_URL } else { "http://localhost:8080" }
$body = '{"user_id":"u123","amount_cents":1299,"currency":"USD","idempotency_key":"demo-123"}'

Write-Host "=== 1) POST /orders with idempotency_key=demo-123 ==="
$r1 = Invoke-RestMethod -Uri "$base/orders" -Method Post -Body $body -ContentType "application/json"
$r1 | ConvertTo-Json
$oid = $r1.order_id

Write-Host "`n=== 2) Repeat same POST (idempotent - same order_id) ==="
$r2 = Invoke-RestMethod -Uri "$base/orders" -Method Post -Body $body -ContentType "application/json"
$r2 | ConvertTo-Json
if ($r2.order_id -eq $oid) { Write-Host "OK: same order_id" } else { Write-Host "MISMATCH: expected $oid" }

Write-Host "`n=== 3) GET /orders/$oid ==="
$get = Invoke-RestMethod -Uri "$base/orders/$oid" -Method Get
$get | ConvertTo-Json
