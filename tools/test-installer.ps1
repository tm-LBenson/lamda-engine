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
   $link=[pscustomobject]@{TargetPath='';Arguments='';WorkingDirectory='';IconLocation=''}
   $link | Add-Member ScriptMethod Save { $global:luiTestShortcutSaved=$true }
   return $link
  }
  return $obj
 }
 # No real desktop is touched; CreateShortcut is mocked above.
 $global:luiTestShortcutSaved=$false
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
 'PASS: installer deployment, shortcut generation, config and rollback after addon copy failure'
} finally {
 foreach($key in $previous.Keys){[Environment]::SetEnvironmentVariable($key,$previous[$key])}
 Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}
