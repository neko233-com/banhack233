param([string]$Version = 'latest')
$ErrorActionPreference = 'Stop'
$repo = 'neko233-com/banhack233'
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run PowerShell as Administrator to install host protection.' }
if ($Version -eq 'latest') {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
    if ($release.draft -or $release.prerelease) { throw 'Expected a published stable release' }
    $Version = $release.tag_name
}
$Version = 'v' + ($Version -replace '^v', '')
if ($Version -cnotmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') { throw 'A stable release tag is required' }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }
$asset = "banhack233-windows-$arch.exe"
$base = "https://github.com/$repo/releases/download/$Version"
$installDir = Join-Path $env:ProgramFiles 'banhack233'
$configDir = Join-Path $env:ProgramData 'banhack233'
New-Item -ItemType Directory -Force -Path $installDir, $configDir | Out-Null
$target = Join-Path $installDir 'banhack233.exe'
$config = Join-Path $configDir 'config.json'
$stage = Join-Path $installDir ('.banhack233-install-' + [Guid]::NewGuid().ToString('N') + '.exe')
try {
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $stage -UseBasicParsing -TimeoutSec 600
    $sums = (Invoke-WebRequest -Uri "$base/SHA256SUMS.txt" -UseBasicParsing -TimeoutSec 60).Content
    $matchesForAsset = @($sums -split '\r?\n' | Where-Object { $_ -match ('^[0-9a-fA-F]{64}\s+\*?' + [regex]::Escape($asset) + '$') })
    if ($matchesForAsset.Count -ne 1) { throw 'Missing or duplicate checksum' }
    $expected = ($matchesForAsset[0] -split '\s+')[0]
    if ((Get-FileHash -LiteralPath $stage -Algorithm SHA256).Hash -ne $expected) { throw 'SHA256 mismatch; installation refused' }
    & $stage version
    if ($LASTEXITCODE -ne 0) { throw 'Downloaded binary does not run' }
    if (Test-Path -LiteralPath $config) {
        & $stage config-check -config $config
        if ($LASTEXITCODE -ne 0) { throw 'Existing configuration validation failed' }
    }
    $task = Get-ScheduledTask -TaskName 'banhack233' -ErrorAction SilentlyContinue
    $wasRunning = $task -and $task.State -eq 'Running'
    if ($wasRunning) { Stop-ScheduledTask -TaskName 'banhack233'; Start-Sleep -Seconds 2 }
    try {
        if (Test-Path -LiteralPath $target) { Copy-Item -LiteralPath $target -Destination ($target + '.installer-backup') -Force }
        Move-Item -LiteralPath $stage -Destination $target -Force
    } finally { if ($wasRunning) { Start-ScheduledTask -TaskName 'banhack233' } }
    if (-not (Test-Path -LiteralPath $config)) {
        & $target init-config -config $config
        if ($LASTEXITCODE -ne 0) { throw 'Configuration initialization failed' }
    }
    Write-Output "Installed $Version. Config: $config"
    Write-Output "Start: & '$target' install-autostart -config '$config'"
    Write-Output "Daily updates: & '$target' auto-update -enable -config '$config'"
} finally {
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Force }
}
