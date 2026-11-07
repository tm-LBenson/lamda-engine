param([switch]$Demo,[string]$WowPath,[string]$Account)
$ErrorActionPreference='Stop'
if($Demo){Write-Host 'Use /lamda > Modules > LamdaCD > Preview > Preview on frames in WoW.';exit 0}
$engine=Join-Path $PSScriptRoot 'lamda-engine.exe'
$data=Join-Path $PSScriptRoot 'data'
if(-not(Test-Path -LiteralPath $engine)){throw 'The MVP binary is missing. Rebuild the local package.'}
New-Item -ItemType Directory -Force -Path $data | Out-Null
$stateFile=Join-Path $data 'state.json'
if(Test-Path -LiteralPath $stateFile){
 try{
  $previous=Get-Content -LiteralPath $stateFile -Raw -Encoding UTF8 | ConvertFrom-Json
  $running=Get-Process -Id $previous.pid -ErrorAction SilentlyContinue
 }catch{$running=$null}
 if($running -and $running.Path -eq $engine){Write-Host 'This MVP session is already running.';exit 0}
}
function Quote-Path([string]$path){
 if($path -match '["\r\n]'){throw 'Invalid path.'}
 return '"'+$path+'"'
}
$arguments='--no-overlay --state-dir '+(Quote-Path $data)
if(-not $WowPath){
 $candidates=@('D:\World of Warcraft\_retail_','C:\Program Files (x86)\World of Warcraft\_retail_')
 $runningWow=Get-Process Wow,WowT -ErrorAction SilentlyContinue | Select-Object -First 1
 if($runningWow -and $runningWow.Path){$candidates=@((Split-Path $runningWow.Path -Parent))+$candidates}
 $found=@($candidates | Select-Object -Unique | Where-Object {Test-Path -LiteralPath (Join-Path $_ 'Wow.exe')})
 if($found.Count -eq 1){$WowPath=$found[0]}else{$WowPath=Read-Host 'WoW _retail_ folder'}
}
$WowPath=(Resolve-Path -LiteralPath $WowPath).Path
if(-not(Test-Path -LiteralPath (Join-Path $WowPath 'Wow.exe'))){throw 'Select the _retail_ folder containing Wow.exe.'}
if(-not $Account){
 $accounts=@(Get-ChildItem -LiteralPath (Join-Path $WowPath 'WTF\Account') -Directory | Where-Object {$_.Name -ne 'SavedVariables'})
 if($accounts.Count -eq 1){$Account=$accounts[0].Name}else{$Account=Read-Host 'Account folder under WTF\Account'}
}
if(-not $Account -or $Account -match '[/\\"\r\n]' -or $Account -in @('.','..')){throw 'Invalid account folder.'}
$accountPath=Join-Path $WowPath ('WTF\Account\'+$Account)
if(-not(Test-Path -LiteralPath $accountPath -PathType Container)){throw 'Account folder not found.'}
$config=Join-Path $accountPath 'SavedVariables\lamdaUI.lua'
$arguments+=' --retail '+(Quote-Path $WowPath)+' --config '+(Quote-Path $config)
$process=Start-Process -FilePath $engine -ArgumentList $arguments -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru
Start-Sleep -Milliseconds 700
if($process.HasExited){throw 'Engine did not stay running. Check data\engine.log; another Lamda Engine may already be running.'}
Write-Host 'Engine started. In /lamda > Modules > LamdaCD > Engine, enable estimates on frames and the live strip, then Apply & Reload. Use /combatlog outdoors and cast a tracked defensive after startup. Estimates follow your unit frames.'
