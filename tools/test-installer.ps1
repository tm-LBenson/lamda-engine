# Runs on Linux PowerShell: mock Windows tooling; exercise actual staging and rollback.
$ErrorActionPreference='Stop'
$root=Join-Path ([IO.Path]::GetTempPath()) ('lui-installer-test-'+[guid]::NewGuid().ToString('N'))
$source=Split-Path $PSScriptRoot -Parent
$previous=@{}
foreach($key in @('OS','APPDATA','LOCALAPPDATA','SystemRoot')){$previous[$key]=[Environment]::GetEnvironmentVariable($key)}
try {
 $env:OS='Windows_NT';$env:APPDATA=Join-Path $root 'appdata';$env:LOCALAPPDATA=Join-Path $root 'local';$env:SystemRoot=Join-Path $root 'Windows'
 $wow=Join-Path $root 'wow';$install=Join-Path $root 'install'
 New-Item -ItemType Directory -Force -Path "$wow/Interface/AddOns/lamdaUI","$wow/WTF/Account/TEST/SavedVariables","$install/current" | Out-Null
 Set-Content "$wow/Wow.exe" ''
 Set-Content "$install/current/old.txt" 'previous engine'
 Set-Content "$wow/Interface/AddOns/lamdaUI/old.txt" 'previous addon'
 # Route the real shortcut file operations into this test's temporary directory.
 New-Item -ItemType Directory -Force -Path "$root/desktop" | Out-Null
 function Join-Path {
  param($Path,$ChildPath)
  if($Path -eq [Environment]::GetFolderPath('Desktop') -and $ChildPath -eq 'lambaUI.lnk'){return "$root/desktop/lambaUI.lnk"}
  Microsoft.PowerShell.Management\Join-Path @PSBoundParameters
 }
 function Get-Process { param($Name,$ErrorAction) return }
 function go {
  if($args[0] -eq 'build') {
   $out=$args[[Array]::IndexOf($args,'-o')+1]
   [IO.File]::WriteAllText($out,"#!/bin/sh`nexit 0`n")
   & chmod +x $out
  }
  $global:LASTEXITCODE=0
 }
 function New-Object {
  param($ComObject)
  if($ComObject -ne 'WScript.Shell'){throw 'Unexpected COM class'}
  $obj=[pscustomobject]@{}
  $obj | Add-Member ScriptMethod CreateShortcut {
   param($path)
   $link=[pscustomobject]@{TargetPath='';Arguments='';WorkingDirectory='';IconLocation='';Path=$path}
   $link | Add-Member ScriptMethod Save {
    Set-Content -LiteralPath $this.Path 'new shortcut'
    if($global:luiTestShortcutFailure){throw 'Injected shortcut save failure'}
    $global:luiTestShortcutSaved=$true
   }
   return $link
  }
  return $obj
 }
 # No real desktop is touched; CreateShortcut is mocked above.
 $global:luiTestShortcutSaved=$false;$global:luiTestShortcutFailure=$false
 & "$source/windows/install.ps1" -SourcePath $source -WowPath $wow -Account TEST -InstallRoot $install
 if(-not $global:luiTestShortcutSaved){throw 'Shortcut not created'}
 if(-not(Test-Path "$install/current/lamda-engine.exe")){throw 'Engine not installed'}
 if(-not(Test-Path "$wow/Interface/AddOns/lamdaUI/Modules/LamdaCD.lua")){throw 'Addon not installed'}
 $config=Get-Content "$env:APPDATA/LamdaUI/install.json" -Raw | ConvertFrom-Json
 if($config.retail -ne $wow){throw 'Incorrect config'}
 Set-Content "$install/current/keep.txt" 'preserve engine'
 Set-Content "$wow/Interface/AddOns/lamdaUI/keep.txt" 'preserve addon'
 function Copy-Item {
  [CmdletBinding()]param($LiteralPath,$Destination,[switch]$Recurse,[switch]$Force)
  if($Destination -eq "$wow/Interface/AddOns/lamdaUI"){throw 'Injected addon copy failure'}
  Microsoft.PowerShell.Management\Copy-Item @PSBoundParameters
 }
 $caught=$false
 try { & "$source/windows/install.ps1" -SourcePath $source -WowPath $wow -Account TEST -InstallRoot $install } catch {if($_ -notmatch 'Injected addon copy failure'){throw};$caught=$true}
 if(-not $caught){throw 'Failure injection did not execute'}
 if((Get-Content "$install/current/keep.txt") -ne 'preserve engine'){throw 'Engine rollback failed'}
 if((Get-Content "$wow/Interface/AddOns/lamdaUI/keep.txt") -ne 'preserve addon'){throw 'Addon rollback failed'}
 Remove-Item Function:\Copy-Item
 # A late shortcut failure must restore files already changed in this transaction.
 Set-Content "$root/desktop/lambaUI.lnk" 'previous shortcut'
 $beforeConfig=Get-Content "$env:APPDATA/LamdaUI/install.json" -Raw
 $global:luiTestShortcutFailure=$true
 $caught=$false
 try { & "$source/windows/install.ps1" -SourcePath $source -WowPath $wow -Account TEST -InstallRoot $install } catch {if($_ -notmatch 'Injected shortcut save failure'){throw};$caught=$true}
 if(-not $caught){throw 'Late failure injection did not execute'}
 if((Get-Content "$root/desktop/lambaUI.lnk") -ne 'previous shortcut'){throw 'Shortcut rollback failed'}
 if((Get-Content "$env:APPDATA/LamdaUI/install.json" -Raw) -ne $beforeConfig){throw 'Configuration rollback failed'}
 if((Get-Content "$install/current/keep.txt") -ne 'preserve engine'){throw 'Late engine rollback failed'}
 if((Get-Content "$wow/Interface/AddOns/lamdaUI/keep.txt") -ne 'preserve addon'){throw 'Late addon rollback failed'}
 # A first installation has nothing to restore: newly written files must disappear.
 Remove-Item "$env:APPDATA/LamdaUI/install.json","$root/desktop/lambaUI.lnk"
 $caught=$false
 try { & "$source/windows/install.ps1" -SourcePath $source -WowPath $wow -Account TEST -InstallRoot $install } catch {if($_ -notmatch 'Injected shortcut save failure'){throw};$caught=$true}
 if(-not $caught){throw 'Fresh configuration failure injection did not execute'}
 if(Test-Path "$env:APPDATA/LamdaUI/install.json"){throw 'Failed new configuration left behind'}
 if(Test-Path "$root/desktop/lambaUI.lnk"){throw 'Failed new shortcut left behind'}
 'PASS: installation; early and late rollback; existing and new configuration/shortcut cleanup'
} finally {
 foreach($key in $previous.Keys){[Environment]::SetEnvironmentVariable($key,$previous[$key])}
 Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}
