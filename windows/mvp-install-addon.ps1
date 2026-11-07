param([string]$WowPath)
$ErrorActionPreference='Stop'
if(Get-Process Wow,WowT -ErrorAction SilentlyContinue){throw 'Close WoW before installing the updated addon. The engine can collect logs with your existing addon.'}
if(-not $WowPath){
 $found=@(@('D:\World of Warcraft\_retail_','C:\Program Files (x86)\World of Warcraft\_retail_') | Where-Object {Test-Path -LiteralPath (Join-Path $_ 'Wow.exe')})
 if($found.Count -eq 1){$WowPath=$found[0]}else{$WowPath=Read-Host 'WoW _retail_ folder'}
}
$WowPath=(Resolve-Path -LiteralPath $WowPath).Path
if(-not(Test-Path -LiteralPath (Join-Path $WowPath 'Wow.exe'))){throw 'Select the _retail_ folder containing Wow.exe.'}
$source=Join-Path $PSScriptRoot 'addon\lamdaUI'
if(-not(Test-Path -LiteralPath (Join-Path $source 'lamdaUI.toc'))){throw 'Bundled addon is missing.'}
$addons=Join-Path $WowPath 'Interface\AddOns'
$target=Join-Path $addons 'lamdaUI'
$id=[Guid]::NewGuid().ToString('N')
$stage=Join-Path $addons ('.lamdaUI-mvp-stage-'+$id)
$previous=Join-Path $addons ('.lamdaUI-mvp-previous-'+$id)
$backup=Join-Path $PSScriptRoot ('backups\'+(Get-Date -Format 'yyyyMMdd-HHmmss')+'-'+$id)
$moved=$false;$installed=$false
try{
 New-Item -ItemType Directory -Force -Path $addons | Out-Null
 Copy-Item -LiteralPath $source -Destination $stage -Recurse
 if(Test-Path -LiteralPath $target){
  New-Item -ItemType Directory -Force -Path $backup | Out-Null
  Copy-Item -LiteralPath $target -Destination (Join-Path $backup 'lamdaUI') -Recurse
  Move-Item -LiteralPath $target -Destination $previous;$moved=$true
 }
 Move-Item -LiteralPath $stage -Destination $target;$installed=$true
}catch{
 if($installed){Remove-Item -LiteralPath $target -Recurse -Force}
 if($moved){Move-Item -LiteralPath $previous -Destination $target}
 throw
}finally{
 if(Test-Path -LiteralPath $stage){Remove-Item -LiteralPath $stage -Recurse -Force}
}
if($moved){Remove-Item -LiteralPath $previous -Recurse -Force}
Write-Host 'Addon installed; SavedVariables preserved. Start WoW, open /lamda > Modules > LamdaCD > Preview to check your frames. Display settings apply without a reload.'
