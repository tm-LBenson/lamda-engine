$ErrorActionPreference = 'Stop'
Start-Process -FilePath (Join-Path $PSScriptRoot 'lamda-engine.exe') -WorkingDirectory $PSScriptRoot -WindowStyle Hidden
