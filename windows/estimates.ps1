param(
 [Parameter(Mandatory)][string]$StatePath,
 [int]$EnginePID,
 [switch]$Demo,
 [string]$RenderTest
)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName PresentationFramework
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class LamdaEstimateWindow {
 [StructLayout(LayoutKind.Sequential)] public struct Rect {public int Left,Top,Right,Bottom;}
 [StructLayout(LayoutKind.Sequential)] public struct Point {public int X,Y;}
 [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr w,out Rect r);
 [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr w,ref Point p);
 [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr w,out uint p);
 [DllImport("user32.dll",EntryPoint="GetWindowLongPtrW")] public static extern IntPtr GetWindowLongPtr(IntPtr w,int n);
 [DllImport("user32.dll",EntryPoint="SetWindowLongPtrW")] public static extern IntPtr SetWindowLongPtr(IntPtr w,int n,IntPtr v);
}
'@
$brushes=[Windows.Media.BrushConverter]::new()
function Brush([string]$color){$brushes.ConvertFromString($color)}
function Label([string]$text,[double]$size,[string]$color) {
 $label=[Windows.Controls.TextBlock]::new()
 $label.Text=$text;$label.FontSize=$size;$label.Foreground=Brush $color
 $label.TextTrimming='CharacterEllipsis'
 return $label
}
function Panel($state,[double]$now) {
 $data=$state.estimates
 $root=[Windows.Controls.StackPanel]::new();$root.Width=440;$root.Background=Brush '#101b29'
 $head=[Windows.Controls.StackPanel]::new();$head.Margin='14,12,14,10'
 $title=Label $data.heading 17 '#dbe9f5';$title.FontWeight='SemiBold';[void]$head.Children.Add($title)
 $status=Label $data.status 11 '#a5bbcf';$status.Margin='0,5,0,0';$status.TextWrapping='Wrap'
 [void]$head.Children.Add($status);[void]$root.Children.Add($head)
 foreach($row in $data.rows) {
  $card=[Windows.Controls.Border]::new();$card.Margin='8,0,8,5';$card.Padding='10,6,10,6'
  $card.Background=Brush '#192a3b';$card.CornerRadius='4'
  $grid=[Windows.Controls.Grid]::new();$card.Child=$grid
  $left=[Windows.Controls.ColumnDefinition]::new();$left.Width=[Windows.GridLength]::new(1,[Windows.GridUnitType]::Star)
  $right=[Windows.Controls.ColumnDefinition]::new();$right.Width=[Windows.GridLength]::new(112)
  [void]$grid.ColumnDefinitions.Add($left);[void]$grid.ColumnDefinitions.Add($right)
  $labels=[Windows.Controls.StackPanel]::new()
  $who=if($row.self){'YOU'}else{$row.player}
  [void]$labels.Children.Add((Label $who 10 '#66d2bd'))
  [void]$labels.Children.Add((Label $row.name 14 '#edf4fb'))
  [void]$labels.Children.Add((Label $row.detail 10 '#a5bbcf'))
  [void]$grid.Children.Add($labels)
  $timer=[Windows.Controls.StackPanel]::new();$timer.VerticalAlignment='Center';$timer.HorizontalAlignment='Right'
  $remaining=[Math]::Max(0,[Math]::Ceiling($row.ends-$now))
  $caption='ESTIMATE';$color='#d3e4f5'
  if($row.status -eq 'estimated_ready'){$value='~ Ready';$caption='UNCONFIRMED'}
  elseif($remaining -le 0){$value='Unknown';$caption='AWAITING UPDATE';$color='#eac078'}
  else {
   $value='~ '+[Math]::Floor($remaining/60).ToString('0')+':'+($remaining%60).ToString('00')
   if($row.status -eq 'estimated_recharge'){$caption='RECHARGE ?';$color='#eac078'}
  }
  $number=Label $value 19 $color;$number.TextAlignment='Right'
  $kind=Label $caption 9 $color;$kind.TextAlignment='Right'
  [void]$timer.Children.Add($number);[void]$timer.Children.Add($kind)
  [Windows.Controls.Grid]::SetColumn($timer,1);[void]$grid.Children.Add($timer)
  [void]$root.Children.Add($card)
 }
 if(@($data.rows).Count -eq 0){
  $empty=Label $data.empty 13 '#c4d7e8';$empty.Margin='14,12,14,16';$empty.TextWrapping='Wrap';[void]$root.Children.Add($empty)
 }
 $footerText=if($data.demo){'DEMO  |  Close this window to stop'}else{'Defensives observed this session  |  /lui to configure'}
 if($data.hidden -gt 0){$footerText=[string]$data.hidden+' more hidden by your row limit  |  /lui'}
 $footer=Label $footerText 10 '#859db3';$footer.Margin='14,5,14,10';[void]$root.Children.Add($footer)
 return $root
}

if($RenderTest) {
 $state=Get-Content -LiteralPath $StatePath -Raw -Encoding UTF8 | ConvertFrom-Json
 $panel=Panel $state ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()/1000.0)
 $panel.Measure([Windows.Size]::new(440,[double]::PositiveInfinity))
 $panel.Arrange([Windows.Rect]::new($panel.DesiredSize));$panel.UpdateLayout()
 $bitmap=[Windows.Media.Imaging.RenderTargetBitmap]::new(440,[int][Math]::Ceiling($panel.ActualHeight),96,96,[Windows.Media.PixelFormats]::Pbgra32)
 $bitmap.Render($panel);$encoder=[Windows.Media.Imaging.PngBitmapEncoder]::new()
 $encoder.Frames.Add([Windows.Media.Imaging.BitmapFrame]::Create($bitmap))
 $file=[IO.File]::Create($RenderTest)
 try{$encoder.Save($file)}finally{$file.Dispose()}
 exit 0
}
if($EnginePID -le 0){throw 'EnginePID is required.'}
$window=[Windows.Window]::new();$window.Title='LamdaUI - Solo demo'
$window.SizeToContent='WidthAndHeight';$window.ResizeMode='NoResize';$window.Background=Brush '#101b29'
if(-not $Demo) {
 $window.WindowStyle='None';$window.AllowsTransparency=$true;$window.Topmost=$true
 $window.ShowInTaskbar=$false;$window.ShowActivated=$false
 $window.Add_SourceInitialized({
  $handle=[Windows.Interop.WindowInteropHelper]::new($window).Handle
  $style=[LamdaEstimateWindow]::GetWindowLongPtr($handle,-20).ToInt64()
  [void][LamdaEstimateWindow]::SetWindowLongPtr($handle,-20,[IntPtr]($style -bor 0x20 -bor 0x08000000 -bor 0x80))
 })
}
$timer=[Windows.Threading.DispatcherTimer]::new();$timer.Interval=[TimeSpan]::FromMilliseconds(250)
$script:lastDisplayError=''
$timer.Add_Tick({
 try {
  if(-not(Get-Process -Id $EnginePID -ErrorAction SilentlyContinue)){$window.Close();return}
  $file=Get-Item -LiteralPath $StatePath -ErrorAction SilentlyContinue
  if(-not $file -or ([DateTime]::UtcNow-$file.LastWriteTimeUtc).TotalSeconds -gt 3){$window.Hide();return}
  $state=Get-Content -LiteralPath $StatePath -Raw -Encoding UTF8 | ConvertFrom-Json
  if($state.pid -ne $EnginePID -or -not $state.estimates.enabled -or [bool]$state.estimates.demo -ne [bool]$Demo){$window.Hide();return}
  $handle=[LamdaEstimateWindow]::GetForegroundWindow()
  if(-not $Demo){
   [uint32]$foregroundPID=0;[void][LamdaEstimateWindow]::GetWindowThreadProcessId($handle,[ref]$foregroundPID)
   $foreground=Get-Process -Id $foregroundPID -ErrorAction SilentlyContinue
   if(-not $foreground -or $foreground.ProcessName -notin @('Wow','WowT')){$window.Hide();return}
  }
  $panel=Panel $state ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()/1000.0)
  $scale=@(0.85,1.0,1.2)[[Math]::Max(0,[Math]::Min(2,$state.config.estimateSize-1))]
  $matrix=[Windows.PresentationSource]::FromVisual($window).CompositionTarget.TransformFromDevice
  if($Demo){$panel.LayoutTransform=[Windows.Media.ScaleTransform]::new($scale,$scale)}
  else{$panel.LayoutTransform=[Windows.Media.ScaleTransform]::new($scale*$matrix.M11,$scale*$matrix.M22)}
  $window.Content=$panel;$window.UpdateLayout()
  if(-not $Demo){
   $rect=[LamdaEstimateWindow+Rect]::new();$point=[LamdaEstimateWindow+Point]::new()
   if(-not [LamdaEstimateWindow]::GetClientRect($handle,[ref]$rect) -or -not [LamdaEstimateWindow]::ClientToScreen($handle,[ref]$point)){$window.Hide();return}
   $origin=$matrix.Transform([Windows.Point]::new($point.X,$point.Y))
   $x=[Math]::Max(0,[Math]::Min($state.config.estimateX,$rect.Right-$window.ActualWidth/$matrix.M11))
   $y=[Math]::Max(0,[Math]::Min($state.config.estimateY,$rect.Bottom-$window.ActualHeight/$matrix.M22))
   $window.Left=$origin.X+$x*$matrix.M11;$window.Top=$origin.Y+$y*$matrix.M22
  }
  $window.Show()
 } catch {
  $window.Hide();$message=$_.Exception.Message
  if($message -ne $script:lastDisplayError){$script:lastDisplayError=$message;[Console]::Error.WriteLine('Estimate display: '+$message)}
 }
})
$window.Add_Closed({$timer.Stop()})
$app=[Windows.Application]::new();$app.ShutdownMode='OnMainWindowClose';$app.MainWindow=$window
$window.Show();$window.Hide();$timer.Start();[void]$app.Run()
