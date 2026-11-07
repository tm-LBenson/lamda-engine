param([switch]$NoPause,[switch]$CheckOnly)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$stage=Join-Path ([IO.Path]::GetTempPath()) ('LamdaUI-Updater-'+[guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($stage) | Out-Null
try {
    foreach($name in @('Install.ps1','Core.ps1')) {Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination (Join-Path $stage $name)}
    if($CheckOnly) {
        . (Join-Path $stage 'Core.ps1')
        $null=Get-LamdaRelease $stage (Join-Path $root 'Updater\data\status.json')
    } else {
        & (Join-Path $stage 'Install.ps1') -Update -InstallRoot $root
    }
} finally {Remove-Item -LiteralPath $stage -Recurse -Force}
