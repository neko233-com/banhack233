$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
Push-Location $repoRoot
try {
    foreach ($tool in @('go', 'node', 'actionlint', 'shellcheck')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "Required tool missing: $tool" }
    }
    actionlint
    if ($LASTEXITCODE -ne 0) { throw 'actionlint failed' }
    $shellFiles = @(Get-ChildItem -LiteralPath scripts -Filter '*.sh' | ForEach-Object FullName)
    shellcheck @shellFiles build-all.sh
    if ($LASTEXITCODE -ne 0) { throw 'ShellCheck failed' }
    node --test .github/scripts/release-guard.test.cjs
    if ($LASTEXITCODE -ne 0) { throw 'Release guard tests failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
    Push-Location tools/docs
    try {
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw 'Documentation vet failed' }
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw 'Documentation tests failed' }
        go run . -version dev -commit local
        if ($LASTEXITCODE -ne 0) { throw 'Documentation build failed' }
    } finally { Pop-Location }
    git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Diff check failed' }
    Write-Output 'All release checks passed.'
} finally { Pop-Location }
