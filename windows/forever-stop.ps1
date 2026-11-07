$ErrorActionPreference='Stop'
$engine=Join-Path $PSScriptRoot 'lamda-engine.exe'
Get-Process lamda-engine -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq $engine} | Stop-Process
Write-Host 'LamdaUI Forever engine stopped.'
