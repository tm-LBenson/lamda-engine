# Download and review this script before running it. PowerShell 5.1 or later.
[CmdletBinding()]
param(
 [string]$Repository = 'https://github.com/tm-LBenson/lamda-engine.git',
 [string]$Ref = 'v0.7.0',
 [string]$WowPath,
 [string]$Account,
 [string]$InstallRoot = (Join-Path $env:LOCALAPPDATA 'LamdaUI'),
 [string]$SourcePath
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
function Run-Native([string]$File,[string[]]$Arguments) {
 & $File @Arguments
 if ($LASTEXITCODE -ne 0) { throw "$File failed (exit $LASTEXITCODE)." }
}
function Refresh-Path {
 $env:Path = [Environment]::GetEnvironmentVariable('Path','Machine')+';'+[Environment]::GetEnvironmentVariable('Path','User')
}
function Need-Tool([string]$Name,[string]$Package) {
 if (Get-Command $Name -ErrorAction SilentlyContinue) { return }
 if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { throw "Install Windows App Installer (winget), then run this script again. Missing: $Name" }
 Run-Native 'winget' @('install','--id',$Package,'--exact','--source','winget','--accept-source-agreements','--accept-package-agreements')
 Refresh-Path
 if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) { throw "$Name installed but not found. Open a new PowerShell and rerun." }
}
if ($env:OS -ne 'Windows_NT') { throw 'This installer is for Windows.' }
if (-not [Environment]::Is64BitOperatingSystem) { throw '64-bit Windows is required.' }
if (-not [Environment]::Is64BitProcess) { throw 'Run this script in 64-bit PowerShell.' }
if (-not $WowPath) {
 $candidates = @('D:\World of Warcraft\_retail_', 'C:\Program Files (x86)\World of Warcraft\_retail_')
 $found = @($candidates | Where-Object { Test-Path (Join-Path $_ 'Wow.exe') })
 if ($found.Count -eq 1) { $WowPath = $found[0] } else { $WowPath = Read-Host 'Path to WoW _retail_ folder' }
}
$WowPath = (Resolve-Path -LiteralPath $WowPath).Path
if (-not (Test-Path (Join-Path $WowPath 'Wow.exe'))) { throw 'Select the _retail_ folder containing Wow.exe.' }
if (Get-Process Wow,WowT -ErrorAction SilentlyContinue) { throw 'Close WoW before installing or updating addon files, then rerun.' }
Need-Tool 'git' 'Git.Git'
Need-Tool 'go' 'GoLang.Go'
$stage = Join-Path ([IO.Path]::GetTempPath()) ('LamdaUI-'+[Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $stage | Out-Null
try {
 $source = Join-Path $stage 'source'
 if ($SourcePath) {
  $source = (Resolve-Path -LiteralPath $SourcePath).Path
 } else {
  if ($Ref.StartsWith('-') -or $Ref -match '[\r\n]') { throw 'Invalid release ref.' }
  Run-Native 'git' @('clone','--depth','1','--branch',$Ref,'--',$Repository,$source)
 }
 $build = Join-Path $stage 'build'
 New-Item -ItemType Directory -Path $build | Out-Null
 Push-Location $source
 try {
  $previousToolchain = $env:GOTOOLCHAIN
  $env:GOTOOLCHAIN = 'local'
  Run-Native 'go' @('test','./...')
  Run-Native 'go' @('build','-trimpath','-o',(Join-Path $build 'lamda-engine.exe'),'./cmd/lamda-engine')
  Run-Native (Join-Path $build 'lamda-engine.exe') @('--version')
 } finally { $env:GOTOOLCHAIN = $previousToolchain; Pop-Location }
 foreach ($name in @('overlay.ps1','launch.ps1')) { Copy-Item -LiteralPath (Join-Path $source "windows\$name") -Destination $build }
 Copy-Item -LiteralPath (Join-Path $source 'assets\lui.ico') -Destination $build
 $addonSource = Join-Path $source 'addon\lamdaUI'
 if (-not (Test-Path (Join-Path $addonSource 'lamdaUI.toc'))) { throw 'Addon missing from source.' }
 $accounts = @(Get-ChildItem -LiteralPath (Join-Path $WowPath 'WTF\Account') -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -ne 'SavedVariables' })
 if (-not $Account -and $accounts.Count -eq 1) { $Account = $accounts[0].Name }
 if (-not $Account) { $Account = Read-Host 'WoW account folder name under WTF\Account' }
 if ($Account -match '[/\\]' -or $Account -in @('.','..') -or -not $Account) { throw 'Invalid account folder name.' }
 $accountPath = Join-Path $WowPath "WTF\Account\$Account"
 if (-not (Test-Path -LiteralPath $accountPath -PathType Container)) { throw 'Account folder does not exist. Log into WoW once, then close it and rerun.' }
 New-Item -ItemType Directory -Force -Path $InstallRoot | Out-Null
 $backup = Join-Path $InstallRoot ('backups\'+(Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
 New-Item -ItemType Directory -Force -Path $backup | Out-Null
 $current = Join-Path $InstallRoot 'current'
 $addon = Join-Path $WowPath 'Interface\AddOns\lamdaUI'
 $configDir = Join-Path $env:APPDATA 'LamdaUI'
 New-Item -ItemType Directory -Force -Path $configDir | Out-Null
 $configPath = Join-Path $configDir 'install.json'
 $oldConfig = Test-Path -LiteralPath $configPath
 $desktop = [Environment]::GetFolderPath('Desktop')
 $shortcutPath = Join-Path $desktop 'lambaUI.lnk'
 $oldShortcut = Test-Path -LiteralPath $shortcutPath
 if ($oldShortcut) { Copy-Item -LiteralPath $shortcutPath -Destination (Join-Path $backup 'lambaUI.lnk') }
 if ($oldConfig) { Copy-Item -LiteralPath $configPath -Destination (Join-Path $backup 'install.json') }
 # Stop only our installed executable, after the replacement has built successfully.
 Get-Process 'lamda-engine' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq (Join-Path $current 'lamda-engine.exe') } | Stop-Process
 $oldEngine = Test-Path $current
 $oldAddon = Test-Path $addon
 $engineMoved = $false; $addonMoved = $false; $engineWritten = $false; $addonWritten = $false; $configWritten = $false; $shortcutWritten = $false
 try {
  if ($oldEngine) { Move-Item -LiteralPath $current -Destination (Join-Path $backup 'engine'); $engineMoved = $true }
  if ($oldAddon) { Move-Item -LiteralPath $addon -Destination (Join-Path $backup 'lamdaUI'); $addonMoved = $true }
  $engineWritten = $true
  Copy-Item -LiteralPath $build -Destination $current -Recurse
  $addonWritten = $true
  Copy-Item -LiteralPath $addonSource -Destination $addon -Recurse
  $configWritten = $true
  @{retail=$WowPath;config=(Join-Path $accountPath 'SavedVariables\lamdaUI.lua')} | ConvertTo-Json | Set-Content -LiteralPath $configPath -Encoding UTF8
  $shell = New-Object -ComObject WScript.Shell
  $shortcut = $shell.CreateShortcut($shortcutPath)
  $shortcut.TargetPath = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
  $shortcut.Arguments = '-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "'+(Join-Path $current 'launch.ps1')+'"'
  $shortcut.WorkingDirectory = $current
  $shortcut.IconLocation = (Join-Path $current 'lui.ico')+',0'
  $shortcutWritten = $true
  $shortcut.Save()
 } catch {
  if ($engineWritten) { Remove-Item -LiteralPath $current -Recurse -Force -ErrorAction SilentlyContinue }
  if ($addonWritten) { Remove-Item -LiteralPath $addon -Recurse -Force -ErrorAction SilentlyContinue }
  if ($engineMoved) { Move-Item -LiteralPath (Join-Path $backup 'engine') -Destination $current }
  if ($addonMoved) { Move-Item -LiteralPath (Join-Path $backup 'lamdaUI') -Destination $addon }
  if ($configWritten) {
   if ($oldConfig) { Copy-Item -LiteralPath (Join-Path $backup 'install.json') -Destination $configPath -Force }
   else { Remove-Item -LiteralPath $configPath -Force -ErrorAction SilentlyContinue }
  }
  if ($shortcutWritten) {
   if ($oldShortcut) { Copy-Item -LiteralPath (Join-Path $backup 'lambaUI.lnk') -Destination $shortcutPath -Force }
   else { Remove-Item -LiteralPath $shortcutPath -Force -ErrorAction SilentlyContinue }
  }
  throw
 }
 Write-Host 'Installed. Launch lambaUI from your desktop. In WoW: /lui'
} finally {
 Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
}
