# Portable MariaDB (MySQL-compatible) for local dev on Windows: no Docker, no admin install.
# Prod uses the Akamai Managed MySQL database; this mirrors it locally.
#   .\scripts\dev-db.ps1          # download (first time), init, start on :3306
#   .\scripts\dev-db.ps1 stop
#   .\scripts\dev-db.ps1 sql      # open a SQL shell
# Credentials match .env.example: trailmix / trailmix, database trailmix.
param([string]$cmd = 'start')

# 'Continue' so native tools printing to stderr (e.g. "can't connect yet") don't abort the script;
# exit codes are checked explicitly where it matters.
$ErrorActionPreference = 'Continue'
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

function Test-Up { cmd /c "`"$bin\mariadb-admin.exe`" -uroot -p$rootPw --port=3306 --connect-timeout=2 ping >nul 2>&1"; return ($LASTEXITCODE -eq 0) }

if (-not (Test-Up)) {
  Start-Process -FilePath "$bin\mariadbd.exe" -ArgumentList "--datadir=`"$data`"", "--port=3306", "--log-error=`"$log`"" -WindowStyle Hidden
  for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    if (Test-Up) { break }
  }
  if (-not (Test-Up)) { Write-Host "MariaDB did not start; see $log"; exit 1 }
}

$setup = "CREATE DATABASE IF NOT EXISTS trailmix CHARACTER SET utf8mb4; CREATE USER IF NOT EXISTS 'trailmix'@'%' IDENTIFIED BY 'trailmix'; CREATE USER IF NOT EXISTS 'trailmix'@'localhost' IDENTIFIED BY 'trailmix'; GRANT ALL ON trailmix.* TO 'trailmix'@'%'; GRANT ALL ON trailmix.* TO 'trailmix'@'localhost'; FLUSH PRIVILEGES;"
cmd /c "`"$bin\mariadb.exe`" -uroot -p$rootPw --port=3306 -e `"$setup`" >nul 2>&1"
if ($LASTEXITCODE -ne 0) {
  # The Windows zip's install-db does not always set the root password. Fix it
  # once via a temporary skip-grant-tables server, then restart normally.
  Write-Host "Root password not set by install-db; resetting it..."
  Stop-Process -Name mariadbd -Force -ErrorAction SilentlyContinue; Start-Sleep -Seconds 3
  $tmp = Start-Process -FilePath "$bin\mariadbd.exe" -ArgumentList "--datadir=`"$data`"", "--port=3306", "--skip-grant-tables", "--log-error=`"$log`"" -WindowStyle Hidden -PassThru
  Start-Sleep -Seconds 5
  cmd /c "`"$bin\mariadb.exe`" --port=3306 -e `"FLUSH PRIVILEGES; ALTER USER 'root'@'localhost' IDENTIFIED BY '$rootPw'; $setup`" >nul 2>&1"
  Stop-Process -Id $tmp.Id -Force; Start-Sleep -Seconds 3
  Start-Process -FilePath "$bin\mariadbd.exe" -ArgumentList "--datadir=`"$data`"", "--port=3306", "--log-error=`"$log`"" -WindowStyle Hidden
  for ($i = 0; $i -lt 30; $i++) { Start-Sleep -Seconds 1; if (Test-Up) { break } }
}
cmd /c "`"$bin\mariadb.exe`" -utrailmix -ptrailmix --port=3306 trailmix -e `"SELECT 1`" >nul 2>&1"
if ($LASTEXITCODE -ne 0) { Write-Host "setup failed; see $log"; exit 1 }
Write-Host "MariaDB ready: mysql://trailmix:trailmix@localhost:3306/trailmix"
