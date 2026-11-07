# Compatibility entry point for the current LamdaUI setup source.
# For normal installation, download Install-LamdaUI.cmd from the latest release.
[CmdletBinding()]
param(
    [string]$WowPath,
    [ValidateSet('retail','forever')][string]$Client = 'retail',
    [string]$InstallRoot
)
$ErrorActionPreference = 'Stop'
$setup = Join-Path $PSScriptRoot '..\setup\Install.ps1'
if (-not (Test-Path -LiteralPath $setup -PathType Leaf)) {
    throw 'Download Install-LamdaUI.cmd from https://github.com/tm-LBenson/lamda-engine/releases/latest, or use a full source checkout.'
}
$setupArguments = @{Client = $Client}
if ($WowPath) { $setupArguments.WowPath = $WowPath }
if ($InstallRoot) { $setupArguments.InstallRoot = $InstallRoot }
& $setup @setupArguments
