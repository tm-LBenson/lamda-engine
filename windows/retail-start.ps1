param([string]$WowPath='D:\World of Warcraft\_retail_',[string]$Account)
$ErrorActionPreference='Stop'
$engine=Join-Path $PSScriptRoot 'lamda-engine.exe'
$data=Join-Path $PSScriptRoot 'data'
if(-not(Test-Path -LiteralPath $engine)){throw 'LamdaUI Retail engine is missing.'}
$WowPath=(Resolve-Path -LiteralPath $WowPath).Path
if(-not(Test-Path -LiteralPath (Join-Path $WowPath 'Wow.exe'))){throw 'Select the Retail folder containing Wow.exe.'}
New-Item -ItemType Directory -Force -Path $data | Out-Null
# Match this bundle only; never adopt or stop another client's companion.
$running=Get-Process lamda-engine -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq $engine}
if($running){Write-Host 'LamdaUI Retail engine is already running.';exit 0}
function Quote-Path([string]$path){
 if($path -match '["\r\n]' -or $path.EndsWith('\')){throw 'Invalid path.'}
 return '"'+$path+'"'
}
if($Account){
 if($Account -match '[/\\"\r\n]' -or $Account -in @('.','..')){throw 'Invalid account folder.'}
 $config=Join-Path $WowPath ('WTF\Account\'+$Account+'\SavedVariables\lamdaUI.lua')
}else{
 $configs=@(Get-ChildItem -LiteralPath (Join-Path $WowPath 'WTF\Account') -Directory |
  ForEach-Object {Join-Path $_.FullName 'SavedVariables\lamdaUI.lua'} |
  Where-Object {Test-Path -LiteralPath $_ -PathType Leaf})
 if($configs.Count -ne 1){throw 'Log into Retail with LamdaUI and reload once. If several accounts have settings, run Start.ps1 -Account with the intended account folder.'}
 $config=$configs[0]
}
if(-not(Test-Path -LiteralPath $config -PathType Leaf)){throw 'Retail LamdaUI settings are missing. Log in and reload once.'}
$arguments='--client retail --wow '+(Quote-Path $WowPath)+' --config '+(Quote-Path $config)+' --state-dir '+(Quote-Path $data)
$process=Start-Process -FilePath $engine -ArgumentList $arguments -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru
Start-Sleep -Milliseconds 900
if($process.HasExited){throw 'Retail engine did not stay running. See data\engine.log.'}
Write-Host 'LamdaUI Retail engine started. Settings: /lamda. Heal binds: /lf.'
