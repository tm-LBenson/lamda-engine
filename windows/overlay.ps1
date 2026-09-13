param([Parameter(Mandatory)][string]$StatePath,[Parameter(Mandatory)][int]$EnginePID)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName PresentationFramework
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class LUIWindow {
 [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr w, out uint p);
 [DllImport("user32.dll", EntryPoint="GetWindowLongPtrW")] public static extern IntPtr GetWindowLongPtr(IntPtr w,int n);
 [DllImport("user32.dll", EntryPoint="SetWindowLongPtrW")] public static extern IntPtr SetWindowLongPtr(IntPtr w,int n,IntPtr v);
}
'@
$window = New-Object Windows.Window
$window.Title = 'LamdaUI'
$window.WindowStyle = 'None'
$window.ResizeMode = 'NoResize'
$window.AllowsTransparency = $true
$window.Background = [Windows.Media.Brushes]::Transparent
$window.Topmost = $true
$window.ShowInTaskbar = $false
$window.ShowActivated = $false
$window.SizeToContent = 'WidthAndHeight'
$panel = New-Object Windows.Controls.StackPanel
$window.Content = $panel
$window.Add_SourceInitialized({
 $handle = (New-Object Windows.Interop.WindowInteropHelper($window)).Handle
 $style = [LUIWindow]::GetWindowLongPtr($handle,-20).ToInt64()
 [void][LUIWindow]::SetWindowLongPtr($handle,-20,[IntPtr]($style -bor 0x20 -bor 0x08000000 -bor 0x80))
})
$timer = New-Object Windows.Threading.DispatcherTimer
$timer.Interval = [TimeSpan]::FromMilliseconds(250)
$timer.Add_Tick({
 try {
  if (-not (Get-Process -Id $EnginePID -ErrorAction SilentlyContinue)) { $window.Close(); return }
  [uint32]$foregroundPID = 0
  [void][LUIWindow]::GetWindowThreadProcessId([LUIWindow]::GetForegroundWindow(),[ref]$foregroundPID)
  $foreground = Get-Process -Id $foregroundPID -ErrorAction SilentlyContinue
  if (-not $foreground -or $foreground.ProcessName -notin @('Wow','WowT')) { $window.Hide(); return }
  $file = Get-Item -LiteralPath $StatePath -ErrorAction SilentlyContinue
  if (-not $file -or ([DateTime]::UtcNow-$file.LastWriteTimeUtc).TotalSeconds -gt 3) { $window.Hide(); return }
  $state = Get-Content -LiteralPath $StatePath -Raw | ConvertFrom-Json
  if ($state.pid -ne $EnginePID) { $window.Hide(); return }
  $now = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()/1000.0
  $panel.Children.Clear()
  $window.Left = [double]$state.config.x
  $window.Top = [double]$state.config.y
  $panel.LayoutTransform = New-Object Windows.Media.ScaleTransform([double]$state.config.scale,[double]$state.config.scale)
  if ($state.config.preview) {
   $label=New-Object Windows.Controls.TextBlock
   $label.Text="Preview"; $label.Foreground=[Windows.Media.Brushes]::Turquoise; $label.Background=[Windows.Media.Brushes]::Black; $label.Padding="10,5"
   [void]$panel.Children.Add($label)
  }
  foreach ($row in $state.rows) {
   $remaining = [int][Math]::Ceiling($row.ends-$now)
   if ($remaining -le 0 -or -not $state.config.enabled) { continue }
   $border = New-Object Windows.Controls.Border
   $border.Background = New-Object Windows.Media.SolidColorBrush([Windows.Media.Color]::FromArgb(210,15,23,35))
   $border.CornerRadius = '5'; $border.Padding = '10,5'; $border.Margin = '0,0,0,3'
   $text = New-Object Windows.Controls.TextBlock
   $text.Foreground = [Windows.Media.Brushes]::White; $text.FontFamily = 'Segoe UI'; $text.FontSize = 14
   $charge = if ($row.charges) { '*' } else { '' }
   $text.Text = if ($row.observedOnly) { '{0}  {1}' -f $row.player,$row.name } else { '{0}  {1}  ~{2}s{3}' -f $row.player,$row.name,$remaining,$charge }
   $border.Child = $text; [void]$panel.Children.Add($border)
  }
  if ($state.update) {
   $notice = New-Object Windows.Controls.TextBlock
   $notice.Text = 'LamdaUI update available: '+$state.update
   $notice.Foreground = [Windows.Media.Brushes]::Turquoise
   $notice.Background = [Windows.Media.Brushes]::Black
   $notice.Padding = '10,5'; [void]$panel.Children.Add($notice)
  }
  if ($panel.Children.Count -gt 0) { $window.Show() } else { $window.Hide() }
 } catch { $window.Hide() }
})
$window.Add_Closed({$timer.Stop()})
$timer.Start()
$app = New-Object Windows.Application
$app.ShutdownMode = 'OnMainWindowClose'
$app.MainWindow = $window
$window.Show(); $window.Hide()
[void]$app.Run()
