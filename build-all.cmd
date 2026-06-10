@echo off
setlocal enabledelayedexpansion
if "%VERSION%"=="" set VERSION=dev
for /f %%i in ('git rev-parse --short HEAD 2^>NUL') do set COMMIT=%%i
if "%COMMIT%"=="" set COMMIT=unknown
if "%DATE%"=="" set DATE=unknown
if not exist dist mkdir dist
set LDFLAGS=-s -w -X github.com/neko233-com/banhack233/internal/version.Version=%VERSION% -X github.com/neko233-com/banhack233/internal/version.Commit=%COMMIT% -X github.com/neko233-com/banhack233/internal/version.Date=%DATE%
for %%O in (linux windows) do (
  for %%A in (amd64 arm64) do (
    set EXT=
    if "%%O"=="windows" set EXT=.exe
    echo build %%O/%%A
    set GOOS=%%O
    set GOARCH=%%A
    set CGO_ENABLED=0
    go build -ldflags "%LDFLAGS%" -o "dist\banhack233-%%O-%%A!EXT!" .\cmd\banhack233
  )
)
