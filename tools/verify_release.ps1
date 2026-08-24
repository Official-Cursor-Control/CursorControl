param(
  [Parameter(Mandatory=$true)][string]$RuntimeZip,
  [string]$Manifest = "release\manifest.json"
)
$ErrorActionPreference = "Stop"
$m = Get-Content $Manifest -Raw | ConvertFrom-Json
$hash = (Get-FileHash $RuntimeZip -Algorithm SHA256).Hash.ToLower()
$bytes = (Get-Item $RuntimeZip).Length
Write-Host "Manifest version : $($m.version)"
Write-Host "Runtime bytes    : $bytes"
Write-Host "Runtime SHA-256  : $hash"
if ($hash -ne $m.package_sha256.ToLower()) { throw "Runtime SHA-256 does not match manifest" }
if ($bytes -ne [int64]$m.package_bytes) { throw "Runtime size does not match manifest" }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [System.IO.Compression.ZipFile]::OpenRead((Resolve-Path $RuntimeZip))
try {
  $exe = $zip.Entries | Where-Object { $_.FullName -match '(^|/)CursorControl\.exe$' } | Select-Object -First 1
  if (-not $exe) { throw "CursorControl.exe not found inside Runtime ZIP" }
  $sha = [System.Security.Cryptography.SHA256]::Create()
  $stream = $exe.Open()
  try { $exeHash = ([BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-','').ToLower() }
  finally { $stream.Dispose(); $sha.Dispose() }
  Write-Host "Game EXE SHA-256 : $exeHash"
  if ($exeHash -ne $m.game_exe_sha256.ToLower()) { throw "Game EXE SHA-256 does not match manifest" }
} finally { $zip.Dispose() }
Write-Host "PASS: release package matches manifest" -ForegroundColor Green
