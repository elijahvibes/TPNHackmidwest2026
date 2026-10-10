# Portable PostgreSQL for local dev on Windows, no Docker and no admin install.
#   .\scripts\dev-db.ps1          # download (first time), init, start on :5432
#   .\scripts\dev-db.ps1 stop
#   .\scripts\dev-db.ps1 psql     # open a psql shell
# Credentials match .env.example: trailmix / trailmix, database trailmix.
param([string]$cmd = 'start')

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$root = Split-Path -Parent $PSScriptRoot
$pg   = Join-Path $root '.pg'
$bin  = Join-Path $pg 'pgsql\bin'
$data = Join-Path $pg 'data'
$log  = Join-Path $pg 'pg.log'
$ver  = '17.6-1'
$env:PGPASSWORD = 'trailmix'

if ($cmd -eq 'stop') {
  & "$bin\pg_ctl.exe" -D $data stop -m fast
  exit $LASTEXITCODE
}
if ($cmd -eq 'psql') {
  & "$bin\psql.exe" -h localhost -p 5432 -U trailmix -d trailmix
  exit $LASTEXITCODE
}

if (-not (Test-Path "$bin\pg_ctl.exe")) {
  New-Item -ItemType Directory -Force $pg | Out-Null
  $zip = Join-Path $pg 'pg.zip'
  Write-Host "Downloading portable PostgreSQL $ver (~350 MB, one time)..."
  Invoke-WebRequest "https://get.enterprisedb.com/postgresql/postgresql-$ver-windows-x64-binaries.zip" -OutFile $zip
  Write-Host "Extracting..."
  Expand-Archive $zip -DestinationPath $pg -Force
  Remove-Item $zip
}

if (-not (Test-Path "$data\PG_VERSION")) {
  Write-Host "Initializing data directory..."
  $pwfile = Join-Path $pg 'pw.txt'
  Set-Content $pwfile 'trailmix' -NoNewline
  & "$bin\initdb.exe" -D $data -U trailmix --pwfile="$pwfile" -E UTF8 -A scram-sha-256 --locale=C | Out-Null
  Remove-Item $pwfile
}

& "$bin\pg_isready.exe" -h localhost -p 5432 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) {
  & "$bin\pg_ctl.exe" -D $data -l $log -o "-p 5432" -w start | Out-Null
}

$exists = & "$bin\psql.exe" -h localhost -p 5432 -U trailmix -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='trailmix'"
if ($exists -ne '1') {
  & "$bin\createdb.exe" -h localhost -p 5432 -U trailmix trailmix
  Write-Host "Created database trailmix"
}
Write-Host "PostgreSQL ready: postgres://trailmix:trailmix@localhost:5432/trailmix?sslmode=disable"
