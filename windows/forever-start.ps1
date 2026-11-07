param([string]$WowPath,[string]$Account)
$ErrorActionPreference='Stop'
$engine=Join-Path $PSScriptRoot 'lamda-engine.exe'
$data=Join-Path $PSScriptRoot 'data'
if(-not(Test-Path -LiteralPath $engine)){throw 'LamdaUI Forever engine is missing.'}
if(-not $WowPath){
 $WowPath=if(Test-Path -LiteralPath 'D:\World of Warcraft\_forever_\Wow.exe'){'D:\World of Warcraft\_forever_'}else{'D:\World of Warcraft\_classic_beta_'}
}
$WowPath=(Resolve-Path -LiteralPath $WowPath).Path
$gameExecutable=if((Split-Path -Leaf $WowPath) -ieq '_forever_'){'Wow.exe'}else{'WowB.exe'}
if(-not(Test-Path -LiteralPath (Join-Path $WowPath $gameExecutable))){throw "Select the Forever folder containing $gameExecutable."}
New-Item -ItemType Directory -Force -Path $data | Out-Null
# Exact binary path: never adopt or stop the Retail engine.
$running=Get-Process lamda-engine -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq $engine}
if($running){Write-Host 'LamdaUI Forever engine is already running.';exit 0}
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
 if($configs.Count -ne 1){throw 'Log into Forever with LamdaUI and reload once. If several accounts have settings, run Start.ps1 -Account with the intended account folder.'}
 $config=$configs[0]
}
if(-not(Test-Path -LiteralPath $config -PathType Leaf)){throw 'Forever LamdaUI settings are missing. Log in and reload once.'}
$arguments='--client forever --wow '+(Quote-Path $WowPath)+' --config '+(Quote-Path $config)+' --state-dir '+(Quote-Path $data)
$process=Start-Process -FilePath $engine -ArgumentList $arguments -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru
Start-Sleep -Milliseconds 900
if($process.HasExited){throw 'Forever engine did not stay running. See data\engine.log.'}
Write-Host 'LamdaUI Forever engine started. Log into Forever; /reload after addon updates. Settings: /lamda. Heal binds: /lf.'
