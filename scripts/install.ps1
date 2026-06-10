param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$Repo = "neko233-com/banhack233"
$Binary = "banhack233"

function Get-LatestVersion {
    $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return ($r.tag_name -replace '^[vV]', '')
}

if ($Version -eq "latest") {
    $Version = Get-LatestVersion
}
$Version = $Version -replace '^[vV]', ''

$arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $arch = "arm64" }

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

$asset = "$Binary-windows-$arch.exe"
$url = "https://github.com/$Repo/releases/download/v$Version/$asset"
if ($isAdmin) {
    $installDir = Join-Path $env:ProgramFiles "banhack233"
    $configDir = Join-Path $env:ProgramData "banhack233"
} else {
    $installDir = Join-Path $env:LOCALAPPDATA "banhack233"
    $configDir = Join-Path $env:LOCALAPPDATA "banhack233"
}
$dest = Join-Path $installDir "$Binary.exe"
$config = Join-Path $configDir "config.json"

New-Item -ItemType Directory -Force -Path $installDir | Out-Null
New-Item -ItemType Directory -Force -Path $configDir | Out-Null

Write-Host "download $url"
Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing

if (!(Test-Path $config)) {
    $cfgUrl = "https://raw.githubusercontent.com/$Repo/main/configs/config.json.example"
    Invoke-WebRequest -Uri $cfgUrl -OutFile $config -UseBasicParsing
}

Write-Host "installed: $dest"
Write-Host "config: $config"
Write-Host "enable: run PowerShell as Administrator, then: `"$dest`" install-autostart -config `"$config`""
