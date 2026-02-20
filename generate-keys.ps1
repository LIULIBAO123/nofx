# Generate RSA keys using .NET
$keysDir = "keys"
if (-not (Test-Path $keysDir)) {
    New-Item -ItemType Directory -Path $keysDir | Out-Null
}

Write-Host "Generating RSA key pair (2048 bits)..." -ForegroundColor Green

# Generate RSA key pair
$rsa = [System.Security.Cryptography.RSA]::Create(2048)

# Export private key in PKCS#1 format
$privateKeyPem = "-----BEGIN RSA PRIVATE KEY-----`n"
$privateKeyBytes = $rsa.ExportRSAPrivateKey()
$privateKeyBase64 = [Convert]::ToBase64String($privateKeyBytes)
# Split into 64-character lines
for ($i = 0; $i -lt $privateKeyBase64.Length; $i += 64) {
    $length = [Math]::Min(64, $privateKeyBase64.Length - $i)
    $privateKeyPem += $privateKeyBase64.Substring($i, $length) + "`n"
}
$privateKeyPem += "-----END RSA PRIVATE KEY-----`n"

# Export public key in PKCS#1 format
$publicKeyPem = "-----BEGIN PUBLIC KEY-----`n"
$publicKeyBytes = $rsa.ExportSubjectPublicKeyInfo()
$publicKeyBase64 = [Convert]::ToBase64String($publicKeyBytes)
# Split into 64-character lines
for ($i = 0; $i -lt $publicKeyBase64.Length; $i += 64) {
    $length = [Math]::Min(64, $publicKeyBase64.Length - $i)
    $publicKeyPem += $publicKeyBase64.Substring($i, $length) + "`n"
}
$publicKeyPem += "-----END PUBLIC KEY-----`n"

# Save to files
$privateKeyPem | Out-File -FilePath "$keysDir/private.pem" -Encoding ASCII -NoNewline
$publicKeyPem | Out-File -FilePath "$keysDir/public.pem" -Encoding ASCII -NoNewline

Write-Host "✓ RSA keys generated successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Private key: $(Resolve-Path $keysDir/private.pem)" -ForegroundColor Cyan
Write-Host "Public key:  $(Resolve-Path $keysDir/public.pem)" -ForegroundColor Cyan
Write-Host ""
Write-Host "⚠️  Important: Keep your private key secure!" -ForegroundColor Yellow

