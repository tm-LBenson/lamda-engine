param([Parameter(Mandatory)][string]$StatePath,[Parameter(Mandatory)][int]$EnginePID)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName PresentationFramework
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class LUIWindow {
 [StructLayout(LayoutKind.Sequential)] public struct Rect {public int Left,Top,Right,Bottom;}
 [StructLayout(LayoutKind.Sequential)] public struct Point {public int X,Y;}
 [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr w,out Rect r);
 [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr w,ref Point p);
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
$panel = New-Object Windows.Controls.Canvas
$panel.Background = [Windows.Media.BrushConverter]::new().ConvertFromString('#0f1723')
$panel.ClipToBounds = $true
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
  $cfg=$state.config; $layout=$state.layout
  if ($layout.width -le 0 -or $layout.height -le 0) { $window.Hide(); return }
  $panel.Children.Clear()
  $panel.Width=$layout.width; $panel.Height=$layout.height
  $matrix=[Windows.PresentationSource]::FromVisual($window).CompositionTarget.TransformFromDevice
  $panel.LayoutTransform=[Windows.Media.ScaleTransform]::new($matrix.M11,$matrix.M22)
  $window.Opacity=$cfg.opacity/100.0
  $brushes=[Windows.Media.BrushConverter]::new()
  $accent=$brushes.ConvertFromString(@('#35bfa7','#3485d5','#9567d8','#d88934')[$cfg.accent-1])
  $header=0
  foreach ($caption in @($(if($cfg.preview -and $cfg.enabled){'Preview'}),$(if($state.update){'LamdaUI update available: '+$state.update}))) {
   if(-not $caption){continue}
   $label=New-Object Windows.Controls.TextBlock
   $label.Text=$caption; $label.Foreground=$accent; $label.FontSize=$layout.fontSize; $label.Padding='8,2'
   [Windows.Controls.Canvas]::SetTop($label,$header*$layout.headerStep)
   [void]$panel.Children.Add($label);$header++
  }
  for($i=0;$i -lt $state.rows.Count;$i++) {
   $row=$state.rows[$i];$cell=$layout.cells[$i]
   $remaining=[Math]::Max(0,$row.ends-$now)
   if($remaining -le 0 -or -not $cfg.enabled){continue}
   $border=New-Object Windows.Controls.Border
   $border.Width=$layout.rowWidth;$border.Height=$layout.rowHeight
   $border.Background=$brushes.ConvertFromString('#142331')
   if($cfg.border){$border.BorderBrush=$accent;$border.BorderThickness='1'}
   $grid=New-Object Windows.Controls.Grid
   $grid.ClipToBounds=$true;$border.Child=$grid
   if($cfg.bars -and $row.duration -gt 0 -and -not $row.observedOnly) {
    $fill=New-Object Windows.Shapes.Rectangle
    $fill.Fill=$accent;$fill.Opacity=0.25;$fill.HorizontalAlignment='Left'
    $fill.Width=($layout.rowWidth-2)*[Math]::Max(0,[Math]::Min(1,$remaining/$row.duration))
    [void]$grid.Children.Add($fill)
   }
   $timerText=''
   if($cfg.showTimers -and -not $row.observedOnly){$timerText='~'+[Math]::Ceiling($remaining)+'s'+$(if($row.charges){'*'})}
   $reserve=0
   if($timerText) {
    $timerLabel=New-Object Windows.Controls.TextBlock
    $timerLabel.Text=$timerText;$timerLabel.FontSize=$layout.fontSize;$timerLabel.Foreground=[Windows.Media.Brushes]::White
    $timerLabel.VerticalAlignment='Center';$timerLabel.HorizontalAlignment='Right';$timerLabel.Margin='0,0,8,0'
    $timerLabel.Measure([Windows.Size]::new([double]::PositiveInfinity,[double]::PositiveInfinity));$reserve=$timerLabel.DesiredSize.Width+12
    [void]$grid.Children.Add($timerLabel)
   }
   $parts=@();if($cfg.showNames){$parts+=$row.player};if($cfg.showSpells){$parts+=$row.name}
   $label=New-Object Windows.Controls.TextBlock
   $label.Text=$parts -join '  ';$label.FontSize=$layout.fontSize;$label.Foreground=[Windows.Media.Brushes]::White
   $label.Margin=[Windows.Thickness]::new(8,0,$reserve+8,0);$label.VerticalAlignment='Center';$label.TextTrimming='CharacterEllipsis'
   [void]$grid.Children.Add($label)
   [Windows.Controls.Canvas]::SetLeft($border,$cell.x);[Windows.Controls.Canvas]::SetTop($border,$cell.y)
   [void]$panel.Children.Add($border)
  }
  $handle=[LUIWindow]::GetForegroundWindow()
  $rect=[LUIWindow+Rect]::new();$point=[LUIWindow+Point]::new()
  if(-not [LUIWindow]::GetClientRect($handle,[ref]$rect) -or -not [LUIWindow]::ClientToScreen($handle,[ref]$point)){$window.Hide();return}
  $matrix=[Windows.PresentationSource]::FromVisual($window).CompositionTarget.TransformFromDevice
  $origin=$matrix.Transform([Windows.Point]::new($point.X,$point.Y))
  $size=$matrix.Transform([Windows.Point]::new($rect.Right-$rect.Left,$rect.Bottom-$rect.Top))
  $window.Left=$origin.X+($size.X-$layout.width*$matrix.M11)*$layout.anchorX+$cfg.x*$matrix.M11
  $window.Top=$origin.Y+($size.Y-$layout.height*$matrix.M22)*$layout.anchorY+$cfg.y*$matrix.M22
  $window.Show()

 } catch { $window.Hide() }
})
$window.Add_Closed({$timer.Stop()})
$timer.Start()
$app = New-Object Windows.Application
$app.ShutdownMode = 'OnMainWindowClose'
$app.MainWindow = $window
$window.Show(); $window.Hide()
[void]$app.Run()
