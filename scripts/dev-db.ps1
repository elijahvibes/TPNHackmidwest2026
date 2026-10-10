# Portable MariaDB (MySQL-compatible) for local dev on Windows: no Docker, no admin install.
# Prod uses the Akamai Managed MySQL database; this mirrors it locally.
#   .\scripts\dev-db.ps1          # download (first time), init, start on :3306
#   .\scripts\dev-db.ps1 stop
#   .\scripts\dev-db.ps1 sql      # open a SQL shell
# Credentials match .env.example: trailmix / trailmix, database trailmix.
param([string]$cmd = 'start')

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$root = Split-Path -Parent $PSScriptRoot
$home_ = Join-Path $root '.mariadb'
$ver  = '11.4.8'
$dist = Join-Path $home_ "mariadb-$ver-winx64"
$bin  = Join-Path $dist 'bin'
$data = Join-Path $home_ 'data'
$log  = Join-Path $home_ 'mariadb.log'
$rootPw = 'trailmix'

if ($cmd -eq 'stop') {
  & "$bin\mariadb-admin.exe" -uroot -p$rootPw --port=3306 shutdown
  exit $LASTEXITCODE
}
if ($cmd -eq 'sql') {
  & "$bin\mariadb.exe" -utrailmix -ptrailmix --port=3306 trailmix
  exit $LASTEXITCODE
}

if (-not (Test-Path "$bin\mariadbd.exe")) {
  New-Item -ItemType Directory -Force $home_ | Out-Null
  $zip = Join-Path $home_ 'mariadb.zip'
  Write-Host "Downloading portable MariaDB $ver (one time)..."
  Invoke-WebRequest "https://archive.mariadb.org/mariadb-$ver/winx64-packages/mariadb-$ver-winx64.zip" -OutFile $zip
  Write-Host "Extracting..."
  Expand-Archive $zip -DestinationPath $home_ -Force
  Remove-Item $zip
}

if (-not (Test-Path "$data\mysql")) {
  Write-Host "Initializing data directory..."
  & "$bin\mariadb-install-db.exe" --datadir="$data" --password=$rootPw --port=3306 | Out-Null
}

& "$bin\mariadb-admin.exe" -uroot -p$rootPw --port=3306 --connect-timeout=2 ping 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) {
  Start-Process -FilePath "$bin\mariadbd.exe" -ArgumentList "--datadir=`"$data`"", "--port=3306", "--log-error=`"$log`"" -WindowStyle Hidden
  for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    & "$bin\mariadb-admin.exe" -uroot -p$rootPw --port=3306 --connect-timeout=2 ping 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) { break }
  }
}

& "$bin\mariadb.exe" -uroot -p$rootPw --port=3306 -e "CREATE DATABASE IF NOT EXISTS trailmix CHARACTER SET utf8mb4; CREATE USER IF NOT EXISTS 'trailmix'@'%' IDENTIFIED BY 'trailmix'; CREATE USER IF NOT EXISTS 'trailmix'@'localhost' IDENTIFIED BY 'trailmix'; GRANT ALL ON trailmix.* TO 'trailmix'@'%'; GRANT ALL ON trailmix.* TO 'trailmix'@'localhost'; FLUSH PRIVILEGES;"
Write-Host "MariaDB ready: mysql://trailmix:trailmix@localhost:3306/trailmix"
