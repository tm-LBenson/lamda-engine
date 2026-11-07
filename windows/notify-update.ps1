param([ValidatePattern('^v?[0-9]+\.[0-9]+\.[0-9]+(-(retail|forever)\.[0-9]+)?$')][string]$Version)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$notice=New-Object System.Windows.Forms.NotifyIcon
try{
 $notice.Icon=[System.Drawing.SystemIcons]::Information
 $notice.Text='Lamda Engine'
 $notice.Visible=$true
 $notice.ShowBalloonTip(8000,'Lamda Engine update available',"Version $Version is available. Nothing has been installed.",[System.Windows.Forms.ToolTipIcon]::Info)
 Start-Sleep -Seconds 9
}finally{$notice.Visible=$false;$notice.Dispose()}
