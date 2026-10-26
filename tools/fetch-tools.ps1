# Downloads pinned dev tools into .tools/ (git-ignored) and verifies their SHA-256.
#   powershell -ExecutionPolicy Bypass -File tools/fetch-tools.ps1
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$tools = Join-Path $root '.tools'

$pins = @(
  @{ Name = 'sqlc';  Version = '1.31.1'
     Url = 'https://github.com/sqlc-dev/sqlc/releases/download/v1.31.1/sqlc_1.31.1_windows_amd64.zip'
     Sha256 = '352711fa7dcb05dcdfefca0ad71b2c9a74fd090f8d7fc609419de4cbc725429f' },
  @{ Name = 'caddy'; Version = '2.11.7'
     Url = 'https://github.com/caddyserver/caddy/releases/download/v2.11.7/caddy_2.11.7_windows_amd64.zip'
     Sha256 = '0a1edc0b799512051c57071ce0e798d3f2cf67dc98171366f1d2b326072e3b06' }
)

New-Item -ItemType Directory -Force (Join-Path $tools '_dl') | Out-Null
foreach ($p in $pins) {
  $zip = Join-Path $tools ('_dl\' + [IO.Path]::GetFileName($p.Url))
  if (-not (Test-Path $zip)) { Invoke-WebRequest -Uri $p.Url -OutFile $zip }
  $hash = (Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower()
  if ($hash -ne $p.Sha256) { throw "$($p.Name): checksum mismatch ($hash)" }
  $dest = Join-Path $tools $p.Name
  if (Test-Path $dest) { Remove-Item -Recurse -Force $dest }
  Expand-Archive -Path $zip -DestinationPath $dest
  Write-Host "$($p.Name) $($p.Version) ready in $dest"
}
