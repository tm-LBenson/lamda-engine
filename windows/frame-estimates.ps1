param(
 [Parameter(Mandatory)][string]$StatePath,
 [int]$EnginePID,
 [string]$RenderTest,
 [switch]$ManageWindows,
 [switch]$LoadOnly
)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName PresentationFramework
Add-Type @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class LamdaFrameWindow {
 [StructLayout(LayoutKind.Sequential)] public struct Rect {public int Left,Top,Right,Bottom;}
 [StructLayout(LayoutKind.Sequential)] public struct Point {public int X,Y;}
 [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr w,out Rect r);
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr w,out Rect r);
 [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr w,ref Point p);
 [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr w,out uint pid);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr w);
 [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr w);
 [DllImport("user32.dll",EntryPoint="GetWindowLongPtrW")] public static extern IntPtr GetWindowLongPtr(IntPtr w,int n);
 [DllImport("user32.dll",EntryPoint="SetWindowLongPtrW")] public static extern IntPtr SetWindowLongPtr(IntPtr w,int n,IntPtr v);
 [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr w,IntPtr after,int x,int y,int width,int height,uint flags);
 [DllImport("user32.dll")] public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
 [DllImport("user32.dll")] static extern int SetWindowRgn(IntPtr window,IntPtr region,bool redraw);
 [DllImport("gdi32.dll")] static extern IntPtr CreateRectRgn(int left,int top,int right,int bottom);
 [DllImport("gdi32.dll")] static extern int CombineRgn(IntPtr dest,IntPtr first,IntPtr second,int mode);
 [DllImport("gdi32.dll")] static extern bool DeleteObject(IntPtr value);
 public static bool CaptureHoles(IntPtr window,int width,int height,Rect[] holes) {
  IntPtr region=CreateRectRgn(0,0,width,height);
  if(region==IntPtr.Zero)return false;
  try {
   foreach(Rect hole in holes){
    IntPtr cut=CreateRectRgn(hole.Left,hole.Top,hole.Right,hole.Bottom);
    if(cut==IntPtr.Zero)return false;
    try{if(CombineRgn(region,region,cut,4)==0)return false;}finally{DeleteObject(cut);}
   }
   // On success Windows owns the region. Keep both input and drawing outside
   // capture strips, including when the game is behind another application.
   if(SetWindowRgn(window,region,true)==0)return false;
   region=IntPtr.Zero;return true;
  }finally{if(region!=IntPtr.Zero)DeleteObject(region);}
 }
}
'@
$brushes=[Windows.Media.BrushConverter]::new()
function FrameBrush([string]$value){$brushes.ConvertFromString($value)}
function ReadFrameStateFile([string]$path) {
 # The engine atomically replaces this file. Permit replacement while reading
 # the opened snapshot instead of briefly blocking the writer's rename.
 $stream=[IO.FileStream]::new($path,[IO.FileMode]::Open,[IO.FileAccess]::Read,([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
 $reader=[IO.StreamReader]::new($stream,[Text.Encoding]::UTF8)
 try {return ($reader.ReadToEnd() | ConvertFrom-Json)}finally{$reader.Dispose()}
}
function FrameLabel([string]$text,[double]$size,[string]$color) {
 $label=[Windows.Controls.TextBlock]::new();$label.Text=$text;$label.FontSize=$size
 $label.Foreground=FrameBrush $color;$label.TextAlignment='Center';$label.TextTrimming='CharacterEllipsis'
 return $label
}
$script:frameIconRoot=Join-Path $PSScriptRoot 'icons'
$script:frameIconCache=@{}
function FrameIconImage([int]$spell) {
 if($script:frameIconCache.ContainsKey($spell)){return $script:frameIconCache[$spell]}
 $art=$null
 try {
  if($spell -lt 0 -or $spell -gt 10000000){throw 'Invalid spell ID'}
  $path=Join-Path $script:frameIconRoot ($spell.ToString()+'.jpg')
  $bitmap=[Windows.Media.Imaging.BitmapImage]::new();$bitmap.BeginInit()
  $bitmap.CacheOption=[Windows.Media.Imaging.BitmapCacheOption]::OnLoad
  $bitmap.UriSource=[Uri]::new($path,[UriKind]::Absolute);$bitmap.EndInit();$bitmap.Freeze()
  if($bitmap.PixelWidth -lt 8 -or $bitmap.PixelHeight -lt 8 -or $bitmap.PixelWidth -gt 256 -or $bitmap.PixelHeight -gt 256){throw 'Invalid icon dimensions'}
  $art=[Windows.Media.Imaging.CroppedBitmap]::new($bitmap,[Windows.Int32Rect]::new(3,3,$bitmap.PixelWidth-6,$bitmap.PixelHeight-6));$art.Freeze()
 } catch {
  if($spell -ne 0){$art=FrameIconImage 0}
 }
 # Cache both hits and misses. No repeated disk reads or network calls in ticks.
 $script:frameIconCache[$spell]=$art
 return $art
}
function FrameBadge([string]$text,[double]$size,[string]$color,[string]$side) {
 $badge=[Windows.Controls.Border]::new();$badge.Background=FrameBrush '#DA080B10'
 $badge.HorizontalAlignment=$side;$badge.VerticalAlignment='Top';$badge.Padding='1,0,1,0'
 $label=FrameLabel $text ([Math]::Max(8,$size*.28)) $color;$label.FontWeight='Bold';$badge.Child=$label
 return $badge
}
$script:overlayIconCache=@{}
function OverlayIconImage($v) {
 if(-not $v.iconPath){return (FrameIconImage ([int]$v.spell))}
 $key=[string]$v.iconPath
 if($script:overlayIconCache.ContainsKey($key)){return $script:overlayIconCache[$key]}
 try {
  if([IO.Path]::GetFileName($key) -ne (([int]$v.spell).ToString()+'.png')){throw 'Invalid overlay icon'}
  $bitmap=[Windows.Media.Imaging.BitmapImage]::new();$bitmap.BeginInit();$bitmap.CacheOption='OnLoad'
  $bitmap.UriSource=[Uri]::new($key,[UriKind]::Absolute);$bitmap.EndInit();$bitmap.Freeze()
  if($bitmap.PixelWidth -lt 8 -or $bitmap.PixelHeight -lt 8 -or $bitmap.PixelWidth -gt 256 -or $bitmap.PixelHeight -gt 256){throw 'Invalid icon dimensions'}
  $art=[Windows.Media.Imaging.CroppedBitmap]::new($bitmap,[Windows.Int32Rect]::new(3,3,$bitmap.PixelWidth-6,$bitmap.PixelHeight-6));$art.Freeze()
  $script:overlayIconCache[$key]=$art;return $art
 }catch{return (FrameIconImage ([int]$v.spell))}
}
function FrameSwipe([double]$size,[double]$fraction) {
 if($fraction -le 0){return $null}
 $shape=[Windows.Shapes.Path]::new();$shape.Fill=FrameBrush '#AA000000';$shape.Tag='cooldown-swipe'
 if($fraction -ge 1){$shape.Data=[Windows.Media.RectangleGeometry]::new([Windows.Rect]::new(0,0,$size,$size));return $shape}
 $center=$size/2;$radius=$size*.72;$angle=-[Math]::PI/2+2*[Math]::PI*$fraction
 $figure=[Windows.Media.PathFigure]::new();$figure.StartPoint=[Windows.Point]::new($center,$center);$figure.IsClosed=$true
 [void]$figure.Segments.Add([Windows.Media.LineSegment]::new([Windows.Point]::new($center,$center-$radius),$true))
 $arc=[Windows.Media.ArcSegment]::new();$arc.Point=[Windows.Point]::new($center+$radius*[Math]::Cos($angle),$center+$radius*[Math]::Sin($angle))
 $arc.Size=[Windows.Size]::new($radius,$radius);$arc.IsLargeArc=$fraction -gt .5;$arc.SweepDirection='Clockwise'
 [void]$figure.Segments.Add($arc)
 $geometry=[Windows.Media.PathGeometry]::new();[void]$geometry.Figures.Add($figure);$geometry.Freeze();$shape.Data=$geometry
 return $shape
}
function OverlayProgressRect([double]$fraction,[string]$direction,[double]$width,[double]$height) {
 $fraction=[Math]::Max(0.0,[Math]::Min(1.0,$fraction));$x=0.0;$y=0.0;$w=$width;$h=$height
 switch($direction){'up'{$y=$height*(1-$fraction);$h=$height*$fraction} 'down'{$h=$height*$fraction} 'left'{$x=$width*(1-$fraction);$w=$width*$fraction} default{$w=$width*$fraction}}
 return [Windows.Rect]::new($x,$y,$w,$h)
}
function OverlayArt($v,[string]$color) {
 if(-not $color){$color=$v.color}
 if($v.spell -gt 0 -or $v.iconRequested){$art=[Windows.Controls.Image]::new();$art.Source=OverlayIconImage $v;$art.Stretch='UniformToFill';$art.Tag='spell-art';return $art}
 $path=[Windows.Shapes.Path]::new();$path.Data=[Windows.Media.Geometry]::Parse('M 24 3 L 42 10 L 42 22 C 42 33 32 40 24 45 C 16 40 6 33 6 22 L 6 10 Z M 24 13 L 24 33 M 16 22 L 32 22');$path.Stroke=FrameBrush $color;$path.StrokeThickness=2;$path.Stretch='Uniform';$path.Margin='3';return $path
}
$script:overlayShapePaths=@{
 rectangle='M0,0 L100,0 100,100 0,100 Z';circle='M50,0 A50,50 0 1 1 50,100 A50,50 0 1 1 50,0 Z';diamond='M50,0 L100,50 50,100 0,50 Z';triangle='M50,0 L100,100 0,100 Z';arrow='M0,30 L60,30 60,0 100,50 60,100 60,70 0,70 Z';cross='M35,0 L65,0 65,35 100,35 100,65 65,65 65,100 35,100 35,65 0,65 0,35 35,35 Z';star='M50,0 L62,35 100,38 70,61 81,100 50,77 19,100 30,61 0,38 38,35 Z'
}
function OverlayShapeLayer([string]$shape,[string]$color,[double]$opacity,[double]$fraction,[string]$direction) {
 $data=$script:overlayShapePaths[$shape];if(-not $data){$data=$script:overlayShapePaths.rectangle}
 $canvas=[Windows.Controls.Canvas]::new();$canvas.Width=100;$canvas.Height=100;$canvas.Opacity=$opacity
 $canvas.Clip=[Windows.Media.RectangleGeometry]::new((OverlayProgressRect $fraction $direction 100 100))
 $path=[Windows.Shapes.Path]::new();$path.Data=[Windows.Media.Geometry]::Parse($data);$path.Fill=FrameBrush $color;[void]$canvas.Children.Add($path)
 $view=[Windows.Controls.Viewbox]::new();$view.Stretch='Fill';$view.Child=$canvas;return $view
}
$script:overlayMediaCache=@{}
$script:overlayMediaBytes=0
function OverlayMediaPath([string]$path,[string]$id,[string]$kind) {
 if($id -notmatch '^[a-f0-9]{64}$' -or -not $path){return $null}
 try {
  $root=[IO.Path]::GetFullPath((Join-Path (Split-Path $StatePath) 'overlays\media'))
  $full=[IO.Path]::GetFullPath($path);if([IO.Path]::GetDirectoryName($full) -ne $root){return $null}
  $extension=[IO.Path]::GetExtension($full);if($kind -eq 'image' -and $extension -notin @('.png','.jpeg')){return $null};if($kind -eq 'sound' -and $extension -ne '.wav'){return $null}
  if([IO.Path]::GetFileNameWithoutExtension($full) -cne $id){return $null}
  $file=[IO.FileInfo]::new($full);if(-not $file.Exists -or $file.Length -gt 8388608 -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)){return $null}
  if(([IO.DirectoryInfo]::new($root).Attributes -band [IO.FileAttributes]::ReparsePoint)){return $null}
  return $full
 }catch{return $null}
}
function OverlayMediaImage($v) {
 $path=OverlayMediaPath $v.imagePath $v.visual.image 'image';if(-not $path){return $null}
 if($script:overlayMediaCache.ContainsKey($path)){return $script:overlayMediaCache[$path]}
 try {
  $bitmap=[Windows.Media.Imaging.BitmapImage]::new();$bitmap.BeginInit();$bitmap.CacheOption='OnLoad';$bitmap.UriSource=[Uri]::new($path,[UriKind]::Absolute);$bitmap.EndInit()
  if($bitmap.PixelWidth -lt 1 -or $bitmap.PixelHeight -lt 1 -or $bitmap.PixelWidth -gt 4096 -or $bitmap.PixelHeight -gt 4096){return $null};$bitmap.Freeze()
  $bytes=[long]$bitmap.PixelWidth*$bitmap.PixelHeight*4
  if($script:overlayMediaCache.Count -ge 64 -or $script:overlayMediaBytes+$bytes -gt 67108864){$script:overlayMediaCache.Clear();$script:overlayMediaBytes=0};$script:overlayMediaCache[$path]=$bitmap;$script:overlayMediaBytes+=$bytes;return $bitmap
 }catch{return $null}
}
function OverlayEase([double]$value,[string]$kind) {
 $t=[Math]::Max(0.0,[Math]::Min(1.0,$value))
 switch($kind){'in'{return $t*$t} 'out'{return 1-(1-$t)*(1-$t)} 'inOut'{if($t -lt .5){return 2*$t*$t};return 1-2*(1-$t)*(1-$t)} default{return $t}}
}
function OverlayMotionChanges($e) {
 return $e -and (($null -ne $e.alpha -and $e.alpha -ne 100) -or $e.x -or $e.y -or ($null -ne $e.scaleX -and $e.scaleX -ne 1) -or ($null -ne $e.scaleY -and $e.scaleY -ne 1) -or $e.rotation -or $e.color)
}
function OverlayMotionActive($m,[double]$elapsed) {
 if(-not $m){return $false}
 if($m.start -and $elapsed -lt $m.start.duration){return (OverlayMotionChanges $m.start)}
 return (OverlayMotionChanges $m.main)
}
function OverlayMotionSample($m,[double]$elapsed,[string]$color,[double]$finishAt=-1) {
 $out=@{alpha=1.0;x=0.0;y=0.0;scaleX=1.0;scaleY=1.0;rotation=0.0;color=$color}
 if(-not $m -or $elapsed -lt 0){return $out}
 if($m.finish -and $finishAt -ge 0){
  $out=OverlayMotionSample $m $finishAt $color;$e=$m.finish;$w=OverlayEase (($elapsed-$finishAt)/$e.duration) $e.easing
  if($null -ne $e.alpha){$out.alpha*=1+($e.alpha/100.0-1)*$w};$out.x+=[double]$e.x*$w;$out.y+=[double]$e.y*$w
  if($null -ne $e.scaleX){$out.scaleX*=1+($e.scaleX-1)*$w};if($null -ne $e.scaleY){$out.scaleY*=1+($e.scaleY-1)*$w};$out.rotation+=[double]$e.rotation*$w
  if($e.color){$hex='#';foreach($i in @(1,3,5)){$a=[Convert]::ToInt32($out.color.Substring($i,2),16);$b=[Convert]::ToInt32($e.color.Substring($i,2),16);$hex+=([int][Math]::Floor($a+($b-$a)*$w+.5)).ToString('X2')};$out.color=$hex};return $out
 }
 $e=$null;$w=0.0
 if($m.start -and $elapsed -lt $m.start.duration){$e=$m.start;$w=1-(OverlayEase ($elapsed/$e.duration) $e.easing)}
 elseif($m.main){
  $e=$m.main;$startDuration=0.0;if($m.start){$startDuration=$m.start.duration};$t=(($elapsed-$startDuration)/$e.duration)%1
  if($e.cycle -ne 'repeat'){if($t -lt .5){$t*=2}else{$t=2-$t*2}};$w=OverlayEase $t $e.easing
 }else{return $out}
 if($null -ne $e.alpha){$out.alpha=1+($e.alpha/100.0-1)*$w};$out.x=[double]$e.x*$w;$out.y=[double]$e.y*$w
 if($null -ne $e.scaleX){$out.scaleX=1+($e.scaleX-1)*$w};if($null -ne $e.scaleY){$out.scaleY=1+($e.scaleY-1)*$w};$out.rotation=[double]$e.rotation*$w
 if($e.color){$hex='#';foreach($i in @(1,3,5)){$a=[Convert]::ToInt32($color.Substring($i,2),16);$b=[Convert]::ToInt32($e.color.Substring($i,2),16);$hex+=([int][Math]::Floor($a+($b-$a)*$w+.5)).ToString('X2')};$out.color=$hex}
 return $out
}
function OverlaySpriteFrame($s,[double]$elapsed,[double]$fraction) {
 $frame=[Math]::Floor([Math]::Max(0.0,$elapsed)*$s.fps)
 switch($s.mode){
  'progress'{$frame=[Math]::Min($s.frames-1,[Math]::Floor([Math]::Max(0.0,[Math]::Min(1.0,$fraction))*$s.frames))}
  'once'{$frame=[Math]::Min($s.frames-1,$frame)}
  'bounce'{$period=[Math]::Max(1,$s.frames*2-2);$frame=$frame%$period;if($frame -ge $s.frames){$frame=$period-$frame}}
  default{$frame=$frame%$s.frames}
 }
 if($s.reverse){$frame=$s.frames-1-$frame};return [int]($s.start+$frame)
}
function OverlaySpriteImage($bitmap,$s,[double]$elapsed,[double]$fraction) {
 if($null -eq $s){return $bitmap}
 if($s.columns -lt 1 -or $s.rows -lt 1 -or $s.columns -gt 64 -or $s.rows -gt 64 -or $s.columns -gt $bitmap.PixelWidth -or $s.rows -gt $bitmap.PixelHeight -or $s.frames -lt 1 -or $s.frames -gt 256 -or $s.start -lt 0 -or $s.start+$s.frames -gt $s.columns*$s.rows){return $null}
 $frame=OverlaySpriteFrame $s $elapsed $fraction;$col=$frame%$s.columns;$row=[Math]::Floor($frame/$s.columns)
 $x=[int][Math]::Floor($col*$bitmap.PixelWidth/$s.columns);$y=[int][Math]::Floor($row*$bitmap.PixelHeight/$s.rows)
 $w=[int][Math]::Floor(($col+1)*$bitmap.PixelWidth/$s.columns)-$x;$h=[int][Math]::Floor(($row+1)*$bitmap.PixelHeight/$s.rows)-$y
 $crop=[Windows.Media.Imaging.CroppedBitmap]::new($bitmap,[Windows.Int32Rect]::new($x,$y,$w,$h));$crop.Freeze();return $crop
}
function OverlayAnimated($v,[double]$now) {
 if($v.finishing -gt 0 -and $v.ends -gt $now -and (OverlayMotionChanges $v.visual.motion.finish)){return $true}
 if(-not $v.finishing -and $v.ends -gt $now -and $v.started -gt 0 -and (OverlayMotionActive $v.visual.motion ([Math]::Max(0.0,$now-$v.started)))){return $true}
 $s=$v.visual.sprite
 return $v.kind -eq 'image' -and $s -and $v.ends -gt $now -and $s.frames -gt 1 -and $s.mode -ne 'progress' -and ($s.mode -ne 'once' -or $now-$v.started -lt ($s.frames-1)/$s.fps)
}
$script:overlayFontFamilies=@{ui='Segoe UI';arial='Arial';georgia='Georgia';consolas='Consolas'}
function OverlayText([string]$text,[double]$font,[string]$color,$visual,[double]$width,[double]$height) {
 $family='Segoe UI';if($visual.fontFamily -and $script:overlayFontFamilies.ContainsKey([string]$visual.fontFamily)){$family=$script:overlayFontFamilies[$visual.fontFamily]}
 $weight=[Windows.FontWeights]::Bold;if($null -ne $visual.bold -and -not $visual.bold){$weight=[Windows.FontWeights]::Normal}
 $style=[Windows.FontStyles]::Normal;if($visual.italic){$style=[Windows.FontStyles]::Italic}
 $align='Center';if($visual.textAlign){$align=$visual.textAlign}
 if($visual.textEffect -eq 'outline'){
  $canvas=[Windows.Controls.Canvas]::new();$canvas.Tag='outlined-text';$canvas.ClipToBounds=$true
  if($text.Length -eq 0){return $canvas}
  $face=[Windows.Media.Typeface]::new([Windows.Media.FontFamily]::new($family),$style,$weight,[Windows.FontStretches]::Normal)
  $formatted=[Windows.Media.FormattedText]::new($text,[Globalization.CultureInfo]::CurrentUICulture,[Windows.FlowDirection]::LeftToRight,$face,$font,(FrameBrush $color))
  $formatted.MaxTextWidth=[Math]::Max(1.0,$width-2);$formatted.MaxTextHeight=[Math]::Max(1.0,$height-2);$formatted.TextAlignment=$align;$formatted.Trimming='CharacterEllipsis'
  $geometry=$formatted.BuildGeometry([Windows.Point]::new(1,[Math]::Max(1.0,($height-$formatted.Height)/2)));$geometry.Freeze()
  $edge=[Windows.Shapes.Path]::new();$edge.Data=$geometry;$edge.Stroke=[Windows.Media.Brushes]::Black;$edge.StrokeThickness=2;$edge.StrokeLineJoin='Round';$edge.Fill=[Windows.Media.Brushes]::Black;[void]$canvas.Children.Add($edge)
  $fill=[Windows.Shapes.Path]::new();$fill.Data=$geometry;$fill.Fill=FrameBrush $color;[void]$canvas.Children.Add($fill);return $canvas
 }
 $label=FrameLabel $text $font $color;$label.FontFamily=[Windows.Media.FontFamily]::new($family);$label.FontWeight=$weight;$label.FontStyle=$style;$label.TextAlignment=$align;$label.VerticalAlignment='Center';$label.TextWrapping='Wrap';return $label
}
function CustomOverlay($v,[double]$now) {
 if($v.ends -le $now){return $null}
 $elapsed=-1.0;if($v.started -gt 0){$elapsed=[Math]::Max(0.0,$now-$v.started)}
 $finishAt=-1.0;if($v.finishing -gt 0){$finishAt=[Math]::Max(0.0,$v.finishing-$v.started)}
 $motion=OverlayMotionSample $v.visual.motion $elapsed $v.color $finishAt;$color=$motion.color
 $size=[Math]::Max(16,[Math]::Min(400,$v.size));$font=[Math]::Max(10,[Math]::Min(72,$v.font));$remaining=$v.ends-$now
 $box=[Windows.Controls.Border]::new();$box.Tag='overlay-'+$v.kind;$box.Opacity=$v.opacity/100*$motion.alpha
 $box.Width=$size;$box.Height=$size;$box.Background=[Windows.Media.Brushes]::Transparent
 if($v.kind -in @('text','bar')){$box.Width=[Math]::Max($size*3,([string]$v.text).Length*$font);$box.Height=[Math]::Max(32,$font*2)}
 $visual=$v.visual;if($visual.width -gt 0){$box.Width=[double]$visual.width};if($visual.height -gt 0){$box.Height=[double]$visual.height}
 $fraction=1.0;if(-not $v.persistent){$fraction=[Math]::Max(0.0,[Math]::Min(1.0,$remaining/$v.duration))};if($null -ne $v.fraction){$fraction=[Math]::Max(0.0,[Math]::Min(1.0,[double]$v.fraction))}
 $trackColor='#101923';if($visual.trackColor){$trackColor=$visual.trackColor}
 $trackOpacity=0.0;if($null -ne $visual.trackOpacity){$trackOpacity=$visual.trackOpacity/100.0}
 $fillOpacity=1.0;if($v.kind -eq 'bar'){$fillOpacity=.3};if($null -ne $visual.fillOpacity){$fillOpacity=$visual.fillOpacity/100.0}
 $grid=[Windows.Controls.Grid]::new();$box.Child=$grid
 if(-not $v.transparent){$box.Background=FrameBrush '#DC101923';$box.BorderBrush=FrameBrush $color;$box.BorderThickness='1'}
 switch($v.kind) {
  {$_ -in @('arc','ring')} {
   $fraction=1.0;if(-not $v.persistent){$fraction=[Math]::Max(0.0,[Math]::Min(1.0,$remaining/$v.duration))}
   $span=270.0;if($v.kind -eq 'ring'){$span=359.9}
   $rotation=-135.0;$thickness=[Math]::Max(3.0,$size*.06);$gap=0.0;$count=1;$capacity=1;$reverse=$false;$around=$false
   if($v.arc){
    $a=$v.arc;$span=[Math]::Min(359.9,$a.sweep);$rotation=[double]$a.start;$thickness=[double]$a.thickness;$gap=[double]$a.gap
    $count=[Math]::Max(0,[Math]::Min(20,[int]$a.count));$capacity=[Math]::Max(1,[Math]::Min(20,[int]$a.capacity));$count=[Math]::Min($count,$capacity)
    $reverse=[bool]$a.reverse;$around=$a.layout -eq 'around'
    if(-not $a.timed){$fraction=[Math]::Max(0.0,[Math]::Min(1.0,[double]$a.fraction))}
   }
   if($null -ne $v.fraction){$fraction=[Math]::Max(0.0,[Math]::Min(1.0,[double]$v.fraction))}
   if($fraction -le 0 -or $count -eq 0){return $null}
   $slots=1;if($around){$slots=[Math]::Min(4,$capacity);$span=[Math]::Min($span,180);if($capacity -gt 2){$span=[Math]::Min($span,90)}}
   $layers=[Math]::Ceiling($capacity/$slots);$radius=$size*.43;$center=$size/2
   $fit=[Math]::Min(1.0,($radius-1)/($thickness/2+($layers-1)*($thickness+$gap)));$thickness*=$fit;$gap*=$fit
   $sweep=$span*$fraction;$direction=1;if($reverse){$direction=-1}
   for($i=0;$i -lt $count;$i++){
    $r=$radius-[Math]::Floor($i/$slots)*($thickness+$gap);$anchor=$rotation+$span/2
    if($around){$positions=@(180,0,270,90);if($v.arc.pair -eq 'vertical'){$positions=@(270,90,180,0)};$anchor=$positions[$i%$slots]+$rotation}
    $start=($anchor-$direction*$span/2)*[Math]::PI/180;$end=($anchor-$direction*$span/2+$direction*$sweep)*[Math]::PI/180
    $figure=[Windows.Media.PathFigure]::new();$figure.StartPoint=[Windows.Point]::new($center+$r*[Math]::Cos($start),$center+$r*[Math]::Sin($start))
    $arc=[Windows.Media.ArcSegment]::new();$arc.Point=[Windows.Point]::new($center+$r*[Math]::Cos($end),$center+$r*[Math]::Sin($end));$arc.Size=[Windows.Size]::new($r,$r);$arc.IsLargeArc=($sweep -gt 180);$arc.SweepDirection='Clockwise';if($reverse){$arc.SweepDirection='Counterclockwise'};[void]$figure.Segments.Add($arc)
    $geometry=[Windows.Media.PathGeometry]::new();[void]$geometry.Figures.Add($figure)
    $path=[Windows.Shapes.Path]::new();$path.Tag='arch-'+$i;$path.Data=$geometry;$path.Stroke=FrameBrush $color;$path.StrokeThickness=$thickness;$path.StrokeStartLineCap='Round';$path.StrokeEndLineCap='Round';[void]$grid.Children.Add($path)
   }
  }
  'icon' {[void]$grid.Children.Add((OverlayArt $v $color))}
  'image' {
   $bitmap=OverlayMediaImage $v;if($null -eq $bitmap){return $null}
   if($visual.sprite){
    $elapsed=0.0;if($v.started -gt 0){$elapsed=[Math]::Max(0.0,$now-$v.started)}
    $bitmap=OverlaySpriteImage $bitmap $visual.sprite $elapsed $fraction;if($null -eq $bitmap){return $null}
    if($visual.sprite.mode -eq 'progress'){$fraction=1.0}
   }
   $stretch='Uniform';if($visual.fit -eq 'cover'){$stretch='UniformToFill'};if($visual.fit -eq 'stretch'){$stretch='Fill'}
   $w=$box.Width;$h=$box.Height;if(-not $v.transparent){$w-=2;$h-=2};$grid.Clip=[Windows.Media.RectangleGeometry]::new((OverlayProgressRect $fraction $visual.direction $w $h))
   if($visual.tint){
    $art=[Windows.Controls.Border]::new();$art.Background=FrameBrush $color;$mask=[Windows.Media.ImageBrush]::new($bitmap);$mask.Stretch=$stretch;$art.OpacityMask=$mask
   }else{$art=[Windows.Controls.Image]::new();$art.Source=$bitmap;$art.Stretch=$stretch}
   $art.Tag='media-image';[void]$grid.Children.Add($art)
  }
  'shape' {
   if($trackOpacity -gt 0){$track=OverlayShapeLayer $visual.shape $trackColor $trackOpacity 1 'right';$track.Tag='shape-track';[void]$grid.Children.Add($track)}
   $shape=OverlayShapeLayer $visual.shape $color $fillOpacity $fraction $visual.direction;$shape.Tag='shape-fill';[void]$grid.Children.Add($shape)
  }
  default {
   $grid.ClipToBounds=$true
   $w=$box.Width;$h=$box.Height;if(-not $v.transparent){$w-=2;$h-=2}
   if($v.kind -eq 'bar'){
    $w=$box.Width;$h=$box.Height;if(-not $v.transparent){$w-=2;$h-=2}
    if($visual.barIcon -in @('left','right','top','bottom')){
     $vertical=$visual.barIcon -in @('top','bottom');$axis=$w;if($vertical){$axis=$h};$iconSize=[Math]::Min([Math]::Min($w,$h),$axis/2)
     $root=[Windows.Controls.Grid]::new();$root.ClipToBounds=$true;$box.Child=$root
     $holder=[Windows.Controls.Border]::new();$holder.Width=$iconSize;$holder.Height=$iconSize;$holder.HorizontalAlignment='Left';$holder.VerticalAlignment='Top';$holder.ClipToBounds=$true;$holder.Child=OverlayArt $v $color;$holder.Tag='bar-icon'
     switch($visual.barIcon){'left'{$grid.Margin=[Windows.Thickness]::new($iconSize,0,0,0)} 'right'{$grid.Margin=[Windows.Thickness]::new(0,0,$iconSize,0);$holder.HorizontalAlignment='Right'} 'top'{$grid.Margin=[Windows.Thickness]::new(0,$iconSize,0,0)} 'bottom'{$grid.Margin=[Windows.Thickness]::new(0,0,0,$iconSize);$holder.VerticalAlignment='Bottom'}}
     if($vertical){$h-=$iconSize}else{$w-=$iconSize};[void]$root.Children.Add($holder);[void]$root.Children.Add($grid)
    }
    if($trackOpacity -gt 0){$track=[Windows.Controls.Border]::new();$track.Tag='bar-track';$track.Background=FrameBrush $trackColor;$track.Opacity=$trackOpacity;[void]$grid.Children.Add($track)}
    $fill=[Windows.Controls.Border]::new();$fill.Tag='progress-fill';$fill.Background=FrameBrush $color;$fill.Opacity=$fillOpacity;$fill.HorizontalAlignment='Left';$fill.VerticalAlignment='Top'
    $rect=OverlayProgressRect $fraction $visual.direction $w $h;$fill.Width=$rect.Width;$fill.Height=$rect.Height;$fill.Margin=[Windows.Thickness]::new($rect.X,$rect.Y,0,0);[void]$grid.Children.Add($fill)
   }
   $label=OverlayText ([string]$v.text) $font $color $visual $w $h;[void]$grid.Children.Add($label)
  }
 }
 if($v.kind -in @('arc','ring') -and ($box.Width -ne $size -or $box.Height -ne $size)){
  $box.Child=$null;$grid.Width=$size;$grid.Height=$size;$view=[Windows.Controls.Viewbox]::new();$view.Stretch='Fill';$view.Child=$grid;$box.Child=$view
 }
 if($visual.rotation -or $visual.flipX -or $visual.flipY -or $visual.motion){
  $transforms=[Windows.Media.TransformGroup]::new();$sx=1;$sy=1;if($visual.flipX){$sx=-1};if($visual.flipY){$sy=-1}
  [void]$transforms.Children.Add([Windows.Media.ScaleTransform]::new($sx*$motion.scaleX,$sy*$motion.scaleY));[void]$transforms.Children.Add([Windows.Media.RotateTransform]::new([double]$visual.rotation+$motion.rotation))
  [void]$transforms.Children.Add([Windows.Media.TranslateTransform]::new($motion.x,$motion.y))
  $box.RenderTransformOrigin=[Windows.Point]::new(.5,.5);$box.RenderTransform=$transforms
 }

 $shadow=[Windows.Media.Effects.DropShadowEffect]::new();$shadow.Color=[Windows.Media.Colors]::Black;$shadow.BlurRadius=3;$shadow.ShadowDepth=1;$shadow.Opacity=1
 if($v.glow){$shadow.Color=([Windows.Media.ColorConverter]::ConvertFromString($color));$shadow.BlurRadius=12;$shadow.ShadowDepth=0}
 if($v.glow -or $v.kind -notin @('text','bar') -or $visual.textEffect -notin @('none','outline')){$box.Effect=$shadow};return $box
}
$script:overlaySoundSeen=@{}
function OverlaySounds($data,[double]$now) {
 foreach($key in @($script:overlaySoundSeen.Keys)){if($now-$script:overlaySoundSeen[$key] -gt 3){$script:overlaySoundSeen.Remove($key)}}
 foreach($s in $data.overlaySounds){
  $custom=[string]$s.name -cmatch '^media:[a-f0-9]{64}$'
  if((-not $custom -and $s.name -notin @('chime','alert','soft','sonar')) -or $now-$s.at -gt 1 -or $now -lt $s.at -or $script:overlaySoundSeen.ContainsKey($s.id)){continue}
  $script:overlaySoundSeen[$s.id]=$now
  if($custom){$path=OverlayMediaPath $s.path $s.name.Substring(6) 'sound';if(-not $path){continue}}else{$path=Join-Path $PSScriptRoot ('sounds\'+$s.name+'.wav')}
  if($s.name -eq 'sonar'){$path=Join-Path (Split-Path $StatePath) 'overlays\sounds\sonar.wav'}
  if(Test-Path -LiteralPath $path){try{$player=[System.Media.SoundPlayer]::new($path);$player.Play()}catch{}}
 }
}

function MeterRasterImage([string]$path) {
 if(-not $path){return $null}
 $directory=[IO.Path]::GetFullPath((Join-Path (Split-Path $StatePath) 'augmentation\native-meter-images'))+'\'
 $full=[IO.Path]::GetFullPath($path)
 if(-not $full.StartsWith($directory,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($path) -notmatch '^[a-f0-9]{64}\.png$'){return $null}
 if(-not $script:meterRasterCache){$script:meterRasterCache=@{}}
 if($script:meterRasterCache.ContainsKey($full)){return $script:meterRasterCache[$full]}
 try{$bitmap=[Windows.Media.Imaging.BitmapImage]::new();$bitmap.BeginInit();$bitmap.CacheOption='OnLoad';$bitmap.UriSource=[Uri]::new($full,[UriKind]::Absolute);$bitmap.EndInit();$bitmap.Freeze();if($script:meterRasterCache.Count -lt 4096){$script:meterRasterCache[$full]=$bitmap};return $bitmap}catch{return $null}
}
function MeterNativeAction($navigation,[string]$action,[string]$value) {
 if(-not $navigation -or [int]$navigation.windowID -lt 1 -or [int]$navigation.windowID -gt 8){return}
 $port=if($Client -eq 'forever'){47642}else{47641}
 $payload=@{windowID=[int]$navigation.windowID;serial=[int]$navigation.serial;epoch=[string]$navigation.epoch;action=$action;value=$value}|ConvertTo-Json -Compress
 Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:$port/api/augmentation/navigation" -Headers @{'X-Lamda-Sim'='1'} -ContentType 'application/json' -Body $payload -TimeoutSec 2|Out-Null
}
function MeterNavigate($body,[string]$action,[string]$rowID='',[int]$delta=0) {
 if(-not $body.owner -or -not $body.segment){return}
 if($body.navigation -and $body.navigation.native -and -not($action -eq 'scroll' -and $body.view -eq 'actors' -and @($body.rows|Where-Object {$_.id -like 'row:*'}).Count -eq 0)){
  try{
   if($action -eq 'row'){
    if($rowID -match '^Player-[A-Za-z0-9-]{1,150}$'){MeterNativeAction $body.navigation 'actor' $rowID}
    elseif($rowID -match '^row:([0-9]{1,5})$'){MeterNativeAction $body.navigation 'row' $Matches[1]}
   }elseif($action -eq 'back'){MeterNativeAction $body.navigation 'back' '0'}
   elseif($action -eq 'scroll'){MeterNativeAction $body.navigation 'scroll' ([Math]::Max(0,[Math]::Min([Math]::Max(0,[int]$body.totalRows-1),[int]$body.offset-$delta*3))).ToString()}
  }catch{[Console]::Error.WriteLine('Meter details: '+$_.Exception.Message)}
  return
 }
 $value=@{schema=1;owner=[string]$body.owner;segment=[string]$body.segment;actor=[string]$body.actor;spell=[string]$body.spell;offset=[int]$body.offset}
 switch($action){
  'row' {
   if(-not $body.inspectable){return}
   if($body.view -eq 'actors') {if($rowID -notmatch '^Player-[A-Za-z0-9-]{1,150}$'){return};$value.actor=$rowID;$value.spell=''}
   elseif($body.view -eq 'spells'){if($rowID -notmatch '^(raw/[0-9]{1,8}|credit/[0-9]{1,8}|support/Player-[A-Za-z0-9-]{1,150}/[0-9]{1,8})$'){return};$value.spell=$rowID}
   else{return};$value.offset=0
  }
  'back' {if($body.view -eq 'targets'){$value.spell=''}else{$value.actor='';$value.spell=''};$value.offset=0}
  'scroll' {
   $now=[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds();$key=$value.owner+'|'+$value.segment+'|'+$value.actor+'|'+$value.spell
   $pending=$script:meterNavigationPending
   if($pending -and $pending.key -eq $key -and $pending.observed -eq [int]$body.offset -and $now-$pending.at -lt 300){$value.offset=$pending.offset}
   $value.offset=[Math]::Max(0,[Math]::Min([Math]::Max(0,[int]$body.totalRows-1),$value.offset-$delta*3))
   $script:meterNavigationPending=@{key=$key;observed=[int]$body.offset;offset=$value.offset;at=$now}
  }
  default {return}
 }
 $dir=Join-Path (Split-Path $StatePath) 'augmentation';[IO.Directory]::CreateDirectory($dir)|Out-Null
 $file='view.json';if([int]$body.windowID -ge 1 -and [int]$body.windowID -le 8){$file='view-'+[int]$body.windowID+'.json'}
 $path=Join-Path $dir $file;$tmp=Join-Path $dir ('view.'+[Guid]::NewGuid().ToString('N')+'.tmp')
 try {
  [IO.File]::WriteAllText($tmp,($value|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false))
  for($attempt=0;$attempt -lt 10;$attempt++){
   try{if([IO.File]::Exists($path)){[IO.File]::Replace($tmp,$path,[NullString]::Value)}else{[IO.File]::Move($tmp,$path)};break}
   catch [IO.IOException]{if($attempt -eq 9){throw};Start-Sleep -Milliseconds 15}
  }
 }finally{if([IO.File]::Exists($tmp)){[IO.File]::Delete($tmp)}}
}
function MeterBodyCanvas($body,[double]$width,[double]$height,[switch]$Interactive) {
 $canvas=[Windows.Controls.Canvas]::new();$canvas.Width=$width;$canvas.Height=$height;$canvas.ClipToBounds=$true;$canvas.IsHitTestVisible=[bool]$Interactive
 if($Interactive){
  $canvas.DataContext=$body;$canvas.Background=[Windows.Media.Brushes]::Transparent
  $canvas.Add_MouseWheel({param($sender,$event)MeterNavigate $sender.DataContext 'scroll' '' ([Math]::Sign($event.Delta));$event.Handled=$true})
  $canvas.Add_MouseRightButtonDown({param($sender,$event)MeterNavigate $sender.DataContext 'back';$event.Handled=$true})
 }
 $title=FrameLabel 'LamdaMeters' 12 '#FFFFFF';$title.TextAlignment='Left';$title.Width=[Math]::Max(20,$width-64);$title.Height=22;[Windows.Controls.Canvas]::SetLeft($title,7);[Windows.Controls.Canvas]::SetTop($title,2);[void]$canvas.Children.Add($title)
 $back=[Windows.Shapes.Rectangle]::new();$back.Width=$width;$back.Height=[Math]::Max(0,$height-24);$back.Fill=FrameBrush '#06090D';$back.Opacity=[Math]::Max(.2,[Math]::Min(1,[double]$body.opacity));[Windows.Controls.Canvas]::SetTop($back,24);[void]$canvas.Children.Add($back)
 $metric=FrameLabel ([string]$body.metric) 10 '#D6DDE6';$metric.TextAlignment='Left';$metric.Width=100;$metric.Height=20;[Windows.Controls.Canvas]::SetLeft($metric,27);[Windows.Controls.Canvas]::SetTop($metric,27);[void]$canvas.Children.Add($metric)
 $session=FrameLabel ([string]$body.session) 10 '#D6DDE6';$session.TextAlignment='Left';$session.Width=[Math]::Max(20,$width-175);$session.Height=20;[Windows.Controls.Canvas]::SetLeft($session,140);[Windows.Controls.Canvas]::SetTop($session,27);[void]$canvas.Children.Add($session)
 if($Interactive -and $body.view -ne 'actors'){
  $backButton=[Windows.Controls.Button]::new();$backButton.Width=20;$backButton.Height=20;$backButton.Content='‹';$backButton.FontSize=18;$backButton.Focusable=$false;$backButton.BorderThickness=0;$backButton.Opacity=.7;$backButton.Background=[Windows.Media.Brushes]::Transparent;$backButton.Foreground=FrameBrush '#C4D0DC';$backButton.ToolTip='Back';$backButton.DataContext=$body
  $backButton.Add_Click({param($sender,$event)MeterNavigate $sender.DataContext 'back';$event.Handled=$true});[Windows.Controls.Canvas]::SetLeft($backButton,4);[Windows.Controls.Canvas]::SetTop($backButton,27);[void]$canvas.Children.Add($backButton)
 }
 $header=53.0;if($body.headerHeight -gt 0){$header=[double]$body.headerHeight}
 $footer=22.0;if($body.footerHeight -gt 0){$footer=[double]$body.footerHeight}
 $top=$header;$index=0
 foreach($row in $body.rows){
  $rowHeight=[Math]::Max(8,[Math]::Min(72,[int]$body.rowHeight));$font=[Math]::Max(6,[Math]::Min(40,[int]$body.fontSize));$rowWidth=[Math]::Max(1,$width-10)
  if($top+$rowHeight -gt $height-$footer+1){break}
  $grid=[Windows.Controls.Grid]::new();$grid.Width=$rowWidth;$grid.Height=$rowHeight-1;$grid.Background=FrameBrush '#101B21';$grid.ClipToBounds=$true;$grid.Tag='meter-row'
  if($Interactive){
   $grid.DataContext=@{body=$body;row=$row}
   $grid.Add_MouseLeftButtonDown({param($sender,$event)MeterNavigate $sender.DataContext.body 'row' ([string]$sender.DataContext.row.id);$event.Handled=$true})
   if($row.tooltip){$grid.ToolTip=[string]$row.tooltip}
  }
  $fill=[Windows.Shapes.Rectangle]::new();$fill.HorizontalAlignment='Left';$fill.Width=$rowWidth*[Math]::Max(0.0,[Math]::Min(1.0,[double]$row.fraction));$fill.Fill=FrameBrush ([string]$row.color);$fill.Opacity=.8;[void]$grid.Children.Add($fill)
  $columns=[Windows.Controls.Grid]::new();$columns.Margin='3,0,5,0'
  $nameColumn=[Windows.Controls.ColumnDefinition]::new();$nameColumn.Width=[Windows.GridLength]::new(1,[Windows.GridUnitType]::Star);[void]$columns.ColumnDefinitions.Add($nameColumn)
  $valueColumn=[Windows.Controls.ColumnDefinition]::new();$valueColumn.Width=[Windows.GridLength]::Auto;[void]$columns.ColumnDefinitions.Add($valueColumn)
  $left=[Windows.Controls.StackPanel]::new();$left.Orientation='Horizontal';$left.VerticalAlignment='Center';$left.ClipToBounds=$true;[Windows.Controls.Grid]::SetColumn($left,0);[void]$columns.Children.Add($left)
  $art=MeterRasterImage ([string]$row.iconPath)
  if(-not $art -and $row.spellID){$art=FrameIconImage ([int]$row.spellID)}
  if($art){$icon=[Windows.Controls.Image]::new();$icon.Source=$art;$icon.Width=[Math]::Min(17,$rowHeight-2);$icon.Height=$icon.Width;$icon.Margin='0,0,4,0';[void]$left.Children.Add($icon)}
  $nameArt=MeterRasterImage ([string]$row.nameImage)
  $name=FrameLabel ([string]$row.name) $font '#FFFFFF';$name.Tag='meter-name';$name.TextAlignment='Left';$name.VerticalAlignment='Center';$name.Margin='0,0,10,0';$name.MinWidth=0;if($nameArt){$nameImage=[Windows.Controls.Image]::new();$nameImage.Source=$nameArt;$nameImage.Width=192*$font/12;$nameImage.Height=16*$font/12;$nameImage.Stretch='Uniform';[void]$left.Children.Add($nameImage)}else{[void]$left.Children.Add($name)}
  $value=FrameLabel ([string]$row.value) $font '#FFFFFF';$value.Tag='meter-value';$value.TextAlignment='Right';$value.VerticalAlignment='Center';$value.TextTrimming='None';[Windows.Controls.Grid]::SetColumn($value,1);[void]$columns.Children.Add($value)
  [void]$grid.Children.Add($columns);[Windows.Controls.Canvas]::SetLeft($grid,5);[Windows.Controls.Canvas]::SetTop($grid,$top);[void]$canvas.Children.Add($grid)
  $top+=$rowHeight;$index++
 }
 if($index -eq 0){$empty=FrameLabel ([string]$body.empty) 11 '#D6DDE6';$empty.TextAlignment='Left';$empty.Width=$width-20;[Windows.Controls.Canvas]::SetLeft($empty,10);[Windows.Controls.Canvas]::SetTop($empty,60);[void]$canvas.Children.Add($empty)}
 $status=FrameLabel ([string]$body.status) 10 '#BCC8D5';$status.TextAlignment='Left';$status.Width=$width-29;$status.Height=16;[Windows.Controls.Canvas]::SetLeft($status,7);[Windows.Controls.Canvas]::SetTop($status,$height-20);[void]$canvas.Children.Add($status)
 return $canvas
}

function FrameCanvas($data,[double]$now,[switch]$SeparateMeter,[switch]$InteractiveMeter) {
 $canvas=[Windows.Controls.Canvas]::new();$canvas.Width=$data.width;$canvas.Height=$data.height
 $canvas.Background=[Windows.Media.Brushes]::Transparent;$canvas.IsHitTestVisible=[bool]$InteractiveMeter;$canvas.ClipToBounds=$true
 # Clip the finished canvas so children and their glow/shadow effects cannot
 # paint over a color strip. A small intersection must not hide a whole alert.
 if($data.captureAreas -and @($data.captureAreas).Count -gt 0) {
  $holes=[Windows.Media.GeometryGroup]::new();$holes.FillRule='Nonzero'
  foreach($area in $data.captureAreas) {
   if($area.width -gt 0 -and $area.height -gt 0){[void]$holes.Children.Add([Windows.Media.RectangleGeometry]::new([Windows.Rect]::new($area.x,$area.y,$area.width,$area.height)))}
  }
  $bounds=[Windows.Media.RectangleGeometry]::new([Windows.Rect]::new(0,0,$data.width,$data.height))
  $clip=[Windows.Media.CombinedGeometry]::new([Windows.Media.GeometryCombineMode]::Exclude,$bounds,$holes);$clip.Freeze();$canvas.Clip=$clip
 }
 foreach($v in $data.overlays) {
  if($SeparateMeter -and $data.meter -and ([string]$v.id).StartsWith('lamdaaug/')){continue}
  if($data.meter.body -and ([string]$v.id).StartsWith('lamdaaug/')){continue}
  $custom=CustomOverlay $v $now;if($null -eq $custom){continue}
  $custom.Width=[Math]::Min($custom.Width,$data.width);$custom.Height=[Math]::Min($custom.Height,$data.height)
  $x=[Math]::Max(0,[Math]::Min($data.width-$custom.Width,$v.x/100*$data.width));if($null -ne $v.pixelX){$x=[double]$v.pixelX};[Windows.Controls.Canvas]::SetLeft($custom,$x)
  $y=[Math]::Max(0,[Math]::Min($data.height-$custom.Height,$v.y/100*$data.height));if($null -ne $v.pixelY){$y=[double]$v.pixelY};[Windows.Controls.Canvas]::SetTop($custom,$y)
  [void]$canvas.Children.Add($custom)
 }
 if($data.meter.body -and -not $SeparateMeter){$body=MeterBodyCanvas $data.meter.body $data.meter.width $data.meter.height -Interactive:$InteractiveMeter;[Windows.Controls.Canvas]::SetLeft($body,$data.meter.x);[Windows.Controls.Canvas]::SetTop($body,$data.meter.y);[void]$canvas.Children.Add($body)}
 if(-not $SeparateMeter){foreach($m in $data.meters){if($m.body){$body=MeterBodyCanvas $m.body $m.width $m.height -Interactive:$InteractiveMeter;[Windows.Controls.Canvas]::SetLeft($body,$m.x);[Windows.Controls.Canvas]::SetTop($body,$m.y);[void]$canvas.Children.Add($body)}}}
 foreach($notice in $data.notices) {
  if($notice.ends -le $now){continue}
  $caption=[string]$notice.text
  if($notice.countdown){$caption+=([Math]::Ceiling($notice.ends-$now)).ToString('0')+'s'}
  $label=FrameLabel $caption 11 '#F3D887';$label.Width=$notice.width;$label.Height=18
  $label.TextTrimming='CharacterEllipsis';$label.Opacity=$notice.opacity/100.0
  $label.Background=FrameBrush '#BA080B10';$label.Tag='arena-notice'
  [Windows.Controls.Canvas]::SetLeft($label,$notice.x);[Windows.Controls.Canvas]::SetTop($label,$notice.y)
  [void]$canvas.Children.Add($label)
 }
 foreach($icon in $data.icons) {
  $box=[Windows.Controls.Border]::new();$box.Width=$icon.size;$box.Height=$icon.size
  $box.Background=FrameBrush '#EA101923';$box.BorderThickness='1';$box.Opacity=$icon.opacity/100.0;$box.Tag=$icon.spell
  $remaining=[Math]::Ceiling($icon.ends-$now)
  $unknown=$icon.status -in @('unknown','calibrating') -or ($remaining -le 0 -and $icon.status -ne 'estimated_ready')
  $color='#6CD9CF'
  if($unknown -or $icon.chargesUnknown -or $icon.deliverySeconds -gt 2){$color='#F0BC70'}
  $box.BorderBrush=FrameBrush $color
  $grid=[Windows.Controls.Grid]::new();$grid.ClipToBounds=$true;$box.Child=$grid
  $art=[Windows.Controls.Image]::new();$art.Source=FrameIconImage ([int]$icon.spell);$art.Stretch='UniformToFill';$art.Tag='spell-art'
  [void]$grid.Children.Add($art)
  if($unknown) {
   $mark='?';if($icon.kick){$mark='C'}
   $art.Opacity=.45;$badge=FrameBadge $mark $icon.size '#F0BC70' 'Right';$badge.Tag='unknown-indicator'
   [void]$grid.Children.Add($badge)
  } else {
   if($icon.status -ne 'estimated_ready') {
    if($icon.duration -gt 0){$swipe=FrameSwipe ($icon.size-2) ([Math]::Min(1,[Math]::Max(0,($icon.ends-$now)/$icon.duration)));if($swipe){[void]$grid.Children.Add($swipe)}}
    $caption=[Math]::Max(0,$remaining).ToString('0')
    if($remaining -ge 60){$caption=[Math]::Floor($remaining/60).ToString('0')+':'+($remaining%60).ToString('00')}
    $timer=FrameLabel $caption ([Math]::Max(10,$icon.size*.43)) '#FFFFFF';$timer.FontWeight='Bold';$timer.Tag='countdown'
    $timer.HorizontalAlignment='Center';$timer.VerticalAlignment='Center';$timer.Background=FrameBrush '#66000000'
    [void]$grid.Children.Add($timer)
   }
   $badge=FrameBadge '~' $icon.size '#6CD9CF' 'Left';$badge.Tag='estimated-indicator';[void]$grid.Children.Add($badge)
  }
  [Windows.Controls.Canvas]::SetLeft($box,$icon.x);[Windows.Controls.Canvas]::SetTop($box,$icon.y)
  [void]$canvas.Children.Add($box)
 }
 if($data.kickPanel) {
  $panel=$data.kickPanel
  $header=[Windows.Controls.Border]::new();$header.Width=$panel.width;$header.Height=22
  $header.Background=FrameBrush '#EA101923';$header.Opacity=$panel.opacity/100.0;$header.Tag='kick-header'
  $caption='LamdaKick';if($panel.preview){$caption='LamdaKick   PREVIEW'}
  if($panel.omitted -gt 0){$caption+='   +'+$panel.omitted+' omitted'}
  $title=FrameLabel $caption 11 '#C9E7EC';$title.VerticalAlignment='Center';$header.Child=$title
  [Windows.Controls.Canvas]::SetLeft($header,$panel.x);[Windows.Controls.Canvas]::SetTop($header,$panel.y);[void]$canvas.Children.Add($header)
 }
 foreach($bar in $data.kickBars) {
  # A pending/omitted row must not throw and take every overlay window down.
  if($null -eq $bar -or $bar.width -le 2 -or $bar.height -le 2){continue}
  # Leave the complete native player's row unobstructed. Counts remain engine
  # observations and are drawn in the separate gutter for every player.
  if(-not $bar.native) {
  $box=[Windows.Controls.Border]::new();$box.Width=$bar.width;$box.Height=$bar.height
  $box.Background=FrameBrush '#EE101923';$box.BorderThickness='1';$box.BorderBrush=FrameBrush '#596574'
  $box.Opacity=$bar.opacity/100.0;$box.Tag='kick-bar'
  $grid=[Windows.Controls.Grid]::new();$grid.ClipToBounds=$true;$box.Child=$grid
  $remaining=[Math]::Ceiling($bar.ends-$now)
  $unknown=$bar.status -in @('unknown','calibrating') -or ($remaining -le 0 -and $bar.status -ne 'estimated_ready')
  $ready=$bar.status -eq 'estimated_ready'
  if(-not $unknown) {
   $fraction=1.0;if(-not $ready){$fraction=[Math]::Min(1,[Math]::Max(0,($bar.ends-$now)/[Math]::Max(1,$bar.duration)))}
   $fill=[Windows.Controls.Border]::new();$fill.HorizontalAlignment='Left';$fill.Width=($bar.width-2)*$fraction
   $fill.Background=FrameBrush $bar.color;$fill.Opacity=.38;$fill.Tag='kick-fill';[void]$grid.Children.Add($fill)
  }
  $art=[Windows.Controls.Image]::new();$art.Source=FrameIconImage ([int]$bar.spell);$art.Stretch='UniformToFill'
  $art.Width=$bar.height-2;$art.Height=$bar.height-2;$art.HorizontalAlignment='Left';$art.Tag='kick-art'
  if($unknown){$art.Opacity=.45};[void]$grid.Children.Add($art)
  $names=[Windows.Controls.StackPanel]::new();$names.HorizontalAlignment='Left';$names.VerticalAlignment='Center'
  $timerWidth=68;if($unknown){$timerWidth=88}
  $names.Margin=[Windows.Thickness]::new($bar.height+4,0,0,0);$names.Width=[Math]::Max(1,$bar.width-$bar.height-$timerWidth-9)
  $font=[Math]::Min(16,[Math]::Max(9,$bar.height*.40))
  $name=FrameLabel $bar.player $font '#FFFFFF';$name.TextAlignment='Left';$name.FontWeight='SemiBold';$name.Tag='kick-player'
  [void]$names.Children.Add($name)
  $detail=$bar.name
  $text='Calibrating';$color='#F0BC70'
  if($unknown) {
   $detail='Watching for a cast or cooldown window'
   if($bar.status -eq 'calibrating' -and $remaining -gt 0){$detail='Watching casts; Ready in ~'+$remaining.ToString('0')+'s'}
   elseif($bar.spell -eq 0){$detail='Identifying interrupt; watching casts'}
  }
  if($bar.height -ge 26){$spell=FrameLabel $detail ([Math]::Min(11,$bar.height*.30)) '#C4CED8';$spell.TextAlignment='Left';$spell.Tag='kick-detail';[void]$names.Children.Add($spell)}
  [void]$grid.Children.Add($names)
  if(-not $unknown){$color='#FFFFFF';if($ready){$text='Ready'}else{$text='~ '+$remaining.ToString('0')+'s'}}
  if($bar.deliverySeconds -gt 2){$color='#F0BC70'}
  $timerFont=[Math]::Min(16,[Math]::Max(10,$bar.height*.43));if($unknown){$timerFont=[Math]::Min(13,$timerFont)}
  $timer=FrameLabel $text $timerFont $color
  $timer.Width=$timerWidth;$timer.HorizontalAlignment='Right';$timer.VerticalAlignment='Center';$timer.FontWeight='Bold';$timer.Tag='kick-timer'
  [void]$grid.Children.Add($timer)
  [Windows.Controls.Canvas]::SetLeft($box,$bar.x);[Windows.Controls.Canvas]::SetTop($box,$bar.y);[void]$canvas.Children.Add($box)
  }
  if($null -ne $bar.count) {
   $text=([int]$bar.count).ToString()
   if($null -ne $bar.attempts){$text+='/'+([int]$bar.attempts).ToString()}
   # Older addon packets may still reserve 28px until the user's next reload.
   # Fit inside the available gutter without shifting the native own bar.
   $gutter=[Math]::Min(72,[Math]::Max(2,$bar.x));$width=$gutter-2
   $count=FrameLabel $text ([Math]::Min(16,[Math]::Max(10,$bar.height*.43))) '#FFFFFF'
   $count.Width=$width;$count.Height=$bar.height;$count.FontWeight='Bold';$count.Opacity=$bar.opacity/100.0;$count.Tag='kick-count'
   $formatted=[Windows.Media.FormattedText]::new($text,[Globalization.CultureInfo]::InvariantCulture,
    [Windows.FlowDirection]::LeftToRight,[Windows.Media.Typeface]::new($count.FontFamily,$count.FontStyle,$count.FontWeight,$count.FontStretch),$count.FontSize,$count.Foreground)
   if($width -gt 0 -and $formatted.Width -gt $width){$count.FontSize*=($width/$formatted.Width)}
   [Windows.Controls.Canvas]::SetLeft($count,[Math]::Max(0,$bar.x-$gutter));[Windows.Controls.Canvas]::SetTop($count,$bar.y)
   [void]$canvas.Children.Add($count)
  }
 }
 return $canvas
}
function FrameStateValid($state,[double]$now,[int]$expectedPID,[string]$window,[int]$width,[int]$height,[switch]$AllowEmpty) {
 if(-not $state -or $state.pid -ne $expectedPID -or -not $state.frameDisplay.enabled){return $false}
 $data=$state.frameDisplay
 return $data.window -eq $window -and $data.width -eq $width -and $data.height -eq $height -and
  $now -ge $data.at -and $now-$data.at -le 1.2 -and $now -lt $data.expires -and ($AllowEmpty -or @($data.icons).Count -gt 0 -or ($null -ne $data.notices -and @($data.notices).Count -gt 0) -or $null -ne $data.kickPanel -or ($data.meters -and @($data.meters).Count -gt 0) -or ($null -ne $data.overlays -and @($data.overlays).Count -gt 0) -or ($null -ne $data.overlaySounds -and @($data.overlaySounds).Count -gt 0))
}
function FrameFocusValid($data,[string]$foreground,[uint32]$owner,[bool]$visible,[bool]$minimized,[bool]$alive) {
 return $data.window -ne '0' -and $owner -gt 0 -and $owner -eq $data.windowPID -and
  $visible -and -not $minimized -and $alive -and
  ($data.window -eq $foreground -or $data.keepUIWhenUnfocused -eq $true)
}
function NewFrameWindow {
$window=[Windows.Window]::new();$window.Title='LamdaUI frame estimates'
$window.WindowStyle='None';$window.ResizeMode='NoResize';$window.AllowsTransparency=$true
$window.Background=[Windows.Media.Brushes]::Transparent;$window.Topmost=$true
$window.ShowInTaskbar=$false;$window.ShowActivated=$false;$window.Focusable=$false;$window.IsHitTestVisible=$false
$window.Width=1;$window.Height=1
$window.Add_SourceInitialized({param($sender,$event)
 $window=$sender
 $handle=[Windows.Interop.WindowInteropHelper]::new($window).Handle
 $style=[LamdaFrameWindow]::GetWindowLongPtr($handle,-20).ToInt64()
 # Layered + transparent passes mouse input through. Never activate or add a taskbar entry.
 [void][LamdaFrameWindow]::SetWindowLongPtr($handle,-20,[IntPtr]($style -bor 0x80000 -bor 0x20 -bor 0x08000000 -bor 0x80))
})
 return $window
}
function FrameRefreshRate($data,[bool]$animated){
 if($data.lowResourceRendering){if($animated){return 10};return 5}
 if($animated){return 30};return 10
}
function FrameCaptureHoles($window,$data) {
 $handle=[Windows.Interop.WindowInteropHelper]::new($window).Handle
 $key=$handle.ToString()+':'+$data.width.ToString()+','+$data.height.ToString()+':'+(ConvertTo-Json -InputObject @($data.captureAreas) -Depth 4 -Compress)
 if($key -eq $script:captureRegionKey){return}
 $holes=[Collections.Generic.List[LamdaFrameWindow+Rect]]::new()
 foreach($area in $data.captureAreas){
  if($area.width -gt 0 -and $area.height -gt 0){
   $hole=[LamdaFrameWindow+Rect]::new();$hole.Left=$area.x;$hole.Top=$area.y
   $hole.Right=$area.x+$area.width;$hole.Bottom=$area.y+$area.height;$holes.Add($hole)
  }
 }
 if(-not [LamdaFrameWindow]::CaptureHoles($handle,$data.width,$data.height,$holes.ToArray())){throw 'Capture region unavailable'}
 $script:captureRegionKey=$key
}
function MeterSavePlacement([string]$scope) {
 if(-not $script:meterWindow -or -not $script:meterContext){return}
 $ctx=$script:meterContext;$rect=[LamdaFrameWindow+Rect]::new()
 $hwnd=[Windows.Interop.WindowInteropHelper]::new($script:meterWindow).Handle
 if(-not [LamdaFrameWindow]::GetWindowRect($hwnd,[ref]$rect)){return}
 if($scope){$script:meterScope=$scope};if(-not $script:meterScope){$script:meterScope='boss'}
 $value=@{schema=1;x=[Math]::Max(0.0,[Math]::Min(1.0,($rect.Left-$ctx.left)/[double]$ctx.width));y=[Math]::Max(0.0,[Math]::Min(1.0,($rect.Top-$ctx.top)/[double]$ctx.height));width=[Math]::Max(300,[Math]::Min(1000,$rect.Right-$rect.Left));height=[Math]::Max(120,[Math]::Min(1000,$rect.Bottom-$rect.Top));locked=[bool]$script:meterLocked;scope=$script:meterScope}
 $dir=Join-Path (Split-Path $StatePath) 'augmentation';[IO.Directory]::CreateDirectory($dir) | Out-Null
 $path=Join-Path $dir 'display.json'
 if([IO.File]::Exists($path)){$previous=ReadFrameStateFile $path;if($previous.autoToken){$value.autoToken=$previous.autoToken}}
 $tmp=Join-Path $dir ('display.'+[Guid]::NewGuid().ToString('N')+'.tmp')
 try {
  [IO.File]::WriteAllText($tmp,($value | ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false))
  # Replace complete files atomically; tolerate a brief open reader on Windows.
  for($attempt=0;$attempt -lt 10;$attempt++){
   try{if([IO.File]::Exists($path)){[IO.File]::Replace($tmp,$path,[NullString]::Value)}else{[IO.File]::Move($tmp,$path)};break}
   catch [IO.IOException]{if($attempt -eq 9){throw};Start-Sleep -Milliseconds 15}
  }
  $script:meterPending=$value;$script:meterPendingUntil=[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()/1000.0+2
 } finally {if([IO.File]::Exists($tmp)){[IO.File]::Delete($tmp)}}
}
function NewMeterWindow {
 $w=[Windows.Window]::new();$w.Title='LamdaMeters';$w.WindowStyle='None';$w.ResizeMode='NoResize';$w.AllowsTransparency=$true
 $w.Background=FrameBrush '#ED0A1018';$w.Topmost=$true;$w.ShowInTaskbar=$false;$w.ShowActivated=$false;$w.Focusable=$false
 $w.Width=380;$w.Height=220
 $w.Add_SourceInitialized({param($sender,$event)
  $hwnd=[Windows.Interop.WindowInteropHelper]::new($sender).Handle
  $style=[LamdaFrameWindow]::GetWindowLongPtr($hwnd,-20).ToInt64()
  # Only this bounded meter accepts mouse input. Do not activate or steal focus.
  [void][LamdaFrameWindow]::SetWindowLongPtr($hwnd,-20,[IntPtr](($style -band (-bnot 0x20)) -bor 0x08000000 -bor 0x80))
 })
 $grid=[Windows.Controls.Grid]::new();$w.Content=$grid
 $canvas=[Windows.Controls.Canvas]::new();$canvas.IsHitTestVisible=$true;$canvas.ClipToBounds=$true;[void]$grid.Children.Add($canvas)
 $grip=[Windows.Controls.Border]::new();$grip.Height=24;$grip.VerticalAlignment='Top';$grip.Background=[Windows.Media.Brushes]::Transparent
 $grip.ToolTip='Drag title to move. Right-click for Lock/Unlock.'
 $grip.Add_MouseLeftButtonDown({param($sender,$event)
  if($script:meterLocked){return};$script:meterDragging=$true
  try {$script:meterWindow.DragMove();MeterSavePlacement ''}finally{$script:meterDragging=$false}
  $event.Handled=$true
 })
 $menu=[Windows.Controls.ContextMenu]::new()
 $lock=[Windows.Controls.MenuItem]::new();$lock.Header='Lock / unlock position';$lock.IsCheckable=$true
 $lock.Add_Click({$script:meterLocked=-not $script:meterLocked;MeterSavePlacement ''});[void]$menu.Items.Add($lock)
 foreach($option in @(@('Current','boss'),@('Overall','overall'),@('Previous','previous'),@('Whole key','key'))){
  $item=[Windows.Controls.MenuItem]::new();$item.Header=$option[0];$item.Tag=$option[1]
  $item.Add_Click({param($sender,$event)MeterSavePlacement ([string]$sender.Tag)});[void]$menu.Items.Add($item)
 }
 $review=[Windows.Controls.MenuItem]::new();$review.Header='Open Augmentation review'
 $review.Add_Click({Start-Process 'http://127.0.0.1:47641/augmentation'});[void]$menu.Items.Add($review)
 $grip.ContextMenu=$menu;[void]$grid.Children.Add($grip)
 $scopeButton=MeterIconButton 'clock' 'Combat history'
 $scopeMenu=[Windows.Controls.ContextMenu]::new();$scopeButton.ContextMenu=$scopeMenu
 $scopeButton.Add_Click({param($sender,$event)$sender.ContextMenu.PlacementTarget=$sender;$sender.ContextMenu.Placement='Bottom';$sender.ContextMenu.IsOpen=$true;$event.Handled=$true})
 [void]$grid.Children.Add($scopeButton)
 $shareButton=MeterIconButton 'chat' 'Share this window' 24
 $shareButton.Add_Click({param($sender,$event)MeterShareClick $sender 'meter';$event.Handled=$true});[void]$grid.Children.Add($shareButton)
 $awardsButton=MeterIconButton 'trophy' 'Share awards' 46
 $awardsButton.Add_Click({param($sender,$event)MeterShareClick $sender 'awards';$event.Handled=$true});[void]$grid.Children.Add($awardsButton)
 $resize=[Windows.Controls.Primitives.Thumb]::new();$resize.Width=18;$resize.Height=18;$resize.HorizontalAlignment='Right';$resize.VerticalAlignment='Bottom';$resize.Cursor='SizeNWSE';$resize.Opacity=.55
 $resize.Add_DragStarted({if(-not $script:meterLocked){$script:meterDragging=$true}})
 $resize.Add_DragDelta({param($sender,$event)
  if($script:meterLocked){return}
  $script:meterWindow.Width=[Math]::Max(300,[Math]::Min(1000,$script:meterWindow.Width+$event.HorizontalChange))
  $script:meterWindow.Height=[Math]::Max(120,[Math]::Min(1000,$script:meterWindow.Height+$event.VerticalChange))
 })
 $resize.Add_DragCompleted({if(-not $script:meterLocked){try{MeterSavePlacement ''}finally{$script:meterDragging=$false}}})
 [void]$grid.Children.Add($resize)
 $w.Tag=@{canvas=$canvas;resize=$resize;grip=$grip;lock=$lock;shareButton=$shareButton;awardsButton=$awardsButton;scopeButton=$scopeButton;scopeMenu=$scopeMenu;scopeKey='';key=''}
 return $w
}
function MeterRefreshOne($data,[double]$now,$point) {
 if(-not $data.meter){if($script:meterWindow){$script:meterWindow.Hide()};return}
 if(-not $script:meterWindow){$script:meterWindow=NewMeterWindow;$script:meterWindow.Show();$script:meterWindow.Hide()}
 if($script:meterDragging){return}
 $w=$script:meterWindow;$m=$data.meter;$script:meterContext=@{left=$point.X;top=$point.Y;width=$data.width;height=$data.height}
 if($script:meterPending){
  $pending=$script:meterPending
  $matched=$m.locked -eq $pending.locked -and $m.scope -eq $pending.scope -and [Math]::Abs($m.x-$pending.x*$data.width) -lt 2 -and [Math]::Abs($m.y-$pending.y*$data.height) -lt 2 -and $m.width -eq $pending.width -and $m.height -eq $pending.height
  if(-not $matched -and $now -lt $script:meterPendingUntil){return}
  $script:meterPending=$null
 }
 $script:meterScope=[string]$m.scope;if(-not $script:meterScope){$script:meterScope='boss'}
 $choices=@($m.choices | Where-Object {$_ -and $_.scope -and $_.label});if($choices.Count -eq 0){$choices=@(@{scope='boss';label='Current'},@{scope='overall';label='Overall'},@{scope='previous';label='Previous'},@{scope='key';label='Whole key'})}
 $choiceKey=($choices | ConvertTo-Json -Compress -Depth 4)+'|'+$script:meterScope
 if($w.Tag.scopeKey -ne $choiceKey){
  $w.Tag.scopeMenu.Items.Clear();$selected='Combat'
  foreach($choice in $choices){
   $item=[Windows.Controls.MenuItem]::new();$item.Header=[string]$choice.label;$item.Tag=[string]$choice.scope;$item.IsCheckable=$true;$item.IsChecked=$item.Tag -eq $script:meterScope
   if($item.IsChecked){$selected=[string]$choice.label}
   $item.Add_Click({param($sender,$event)MeterSavePlacement ([string]$sender.Tag);$event.Handled=$true})
   [void]$w.Tag.scopeMenu.Items.Add($item)
  }
  $w.Tag.scopeButton.ToolTip='Combat history - '+$selected;$w.Tag.scopeKey=$choiceKey
 }
 $script:meterLocked=[bool]$m.locked;$w.Tag.resize.IsEnabled=-not $script:meterLocked;$w.Tag.lock.IsChecked=$script:meterLocked
 $hwnd=[Windows.Interop.WindowInteropHelper]::new($w).Handle
 $visibility=if($m.nativeHost){[Windows.Visibility]::Collapsed}else{[Windows.Visibility]::Visible}
 $w.Tag.shareButton.Visibility=$visibility;$w.Tag.awardsButton.Visibility=$visibility;$w.Tag.grip.Visibility=$visibility;$w.Tag.scopeButton.Visibility=$visibility;$w.Tag.resize.Visibility=$visibility
 [void][LamdaFrameWindow]::SetWindowPos($hwnd,[IntPtr](-1),([int]($point.X+$m.x)),([int]($point.Y+$m.y)),([int]$m.width),([int]$m.height),0x0010)
 # Exclude every optical capture rectangle from this window too.
 $holes=[Collections.Generic.List[LamdaFrameWindow+Rect]]::new()
 if($m.nativeHost){
  $titleHole=[LamdaFrameWindow+Rect]::new();$titleHole.Left=0;$titleHole.Top=0;$titleHole.Right=[int]$m.width;$titleHole.Bottom=24;if($m.headerHeight -gt 0){$titleHole.Bottom=[int]$m.headerHeight};$holes.Add($titleHole)
  $resizeHole=[LamdaFrameWindow+Rect]::new();$resizeHole.Left=[int]$m.width-18;$resizeHole.Top=[int]$m.height-18;$resizeHole.Right=[int]$m.width;$resizeHole.Bottom=[int]$m.height;$holes.Add($resizeHole)
  if($m.menu){$menuHole=[LamdaFrameWindow+Rect]::new();$menuHole.Left=[int]($m.menu.x-$m.x);$menuHole.Top=[int]($m.menu.y-$m.y);$menuHole.Right=$menuHole.Left+$m.menu.width;$menuHole.Bottom=$menuHole.Top+$m.menu.height;$holes.Add($menuHole)}
 }
 foreach($area in $data.captureAreas){
  $hole=[LamdaFrameWindow+Rect]::new();$hole.Left=[int]($area.x-$m.x);$hole.Top=[int]($area.y-$m.y);$hole.Right=$hole.Left+$area.width;$hole.Bottom=$hole.Top+$area.height;$holes.Add($hole)
 }
 if(-not [LamdaFrameWindow]::CaptureHoles($hwnd,[int]$m.width,[int]$m.height,$holes.ToArray())){throw 'Meter capture region unavailable'}
 $canvas=$w.Tag.canvas;$canvas.Children.Clear()
 $matrix=[Windows.PresentationSource]::FromVisual($w).CompositionTarget.TransformFromDevice
 $canvas.LayoutTransform=[Windows.Media.ScaleTransform]::new($matrix.M11,$matrix.M22)
 $canvas.Width=$m.width;$canvas.Height=$m.height
 if($m.body){[void]$canvas.Children.Add((MeterBodyCanvas $m.body $m.width $m.height -Interactive))}
 foreach($v in $data.overlays){
  if(-not ([string]$v.id).StartsWith('lamdaaug/')){continue}
  if($m.body){continue}
  $custom=CustomOverlay $v $now;if($null -eq $custom){continue}
  [Windows.Controls.Canvas]::SetLeft($custom,([double]$v.pixelX-$m.x));[Windows.Controls.Canvas]::SetTop($custom,([double]$v.pixelY-$m.y));[void]$canvas.Children.Add($custom)
 }
 if(-not $w.IsVisible){$w.Show()}
}
function MeterRefresh($data,[double]$now,$point) {
 if(-not $script:meterWindows){$script:meterWindows=@{}}
 if($data.meters){
  $seen=@{};$legacy=$script:meterWindow
  try{
   foreach($m in $data.meters){
    $id=[int]$m.id;if($id -lt 1 -or $id -gt 8 -or $seen.ContainsKey($id)){continue};$seen[$id]=$true
    $script:meterWindow=$script:meterWindows[$id];$script:meterPending=$null
    $part=@{};foreach($p in $data.PSObject.Properties){$part[$p.Name]=$p.Value};$part.meter=$m
    MeterRefreshOne ([pscustomobject]$part) $now $point
    $script:meterWindows[$id]=$script:meterWindow
   }
  }finally{$script:meterWindow=$legacy}
  if($legacy){$legacy.Hide()}
  foreach($id in @($script:meterWindows.Keys)){if(-not $seen.ContainsKey($id)){$script:meterWindows[$id].Hide()}}
  return
 }
 foreach($w in $script:meterWindows.Values){$w.Hide()}
 MeterRefreshOne $data $now $point
}
function MeterHide {
 if($script:meterWindow -and $script:meterWindow.IsVisible -and -not $script:meterDragging){$script:meterWindow.Hide()}
 if($script:meterWindows){foreach($w in $script:meterWindows.Values){$w.Hide()}}
}
function MeterClose {
 if($script:meterWindow){$script:meterWindow.Close();$script:meterWindow=$null}
 if($script:meterWindows){foreach($w in $script:meterWindows.Values){$w.Close()};$script:meterWindows=@{}}
}
. (Join-Path $PSScriptRoot 'overlay-windows.ps1')
if($LoadOnly){return}
if($ManageWindows){
 $managerDPI=[LamdaFrameWindow]::SetThreadDpiAwarenessContext([IntPtr](-4))
 try{ShowDesktopManager}catch{[void][Windows.MessageBox]::Show($_.Exception.Message,'Overlay windows')}finally{if($managerDPI -ne [IntPtr]::Zero){[void][LamdaFrameWindow]::SetThreadDpiAwarenessContext($managerDPI)}}
 return
}
if($RenderTest) {
 $state=Get-Content -LiteralPath $StatePath -Raw -Encoding UTF8 | ConvertFrom-Json
 $canvas=FrameCanvas $state.frameDisplay $state.frameDisplay.at
 $canvas.Measure([Windows.Size]::new($canvas.Width,$canvas.Height))
 $canvas.Arrange([Windows.Rect]::new(0,0,$canvas.Width,$canvas.Height));$canvas.UpdateLayout()
 $bitmap=[Windows.Media.Imaging.RenderTargetBitmap]::new([int]$canvas.Width,[int]$canvas.Height,96,96,[Windows.Media.PixelFormats]::Pbgra32)
 $bitmap.Render($canvas);$encoder=[Windows.Media.Imaging.PngBitmapEncoder]::new()
 $encoder.Frames.Add([Windows.Media.Imaging.BitmapFrame]::Create($bitmap))
 $file=[IO.File]::Create($RenderTest);try{$encoder.Save($file)}finally{$file.Dispose()}
 exit 0
}
if($EnginePID -le 0){throw 'EnginePID is required.'}
$engine=Get-Process -Id $EnginePID -ErrorAction Stop
# Keep a handle to this process instance. Re-enumerating it with Get-Process on
# every UI tick cost ~110 ms/call on the live machine, even with no icons shown.
# The retained handle also prevents a reused PID from being mistaken for it.
$engineHandle=$engine.Handle
$stateFile=[IO.FileInfo]::new($StatePath)
$oldDPI=[LamdaFrameWindow]::SetThreadDpiAwarenessContext([IntPtr](-4))
$window=NewFrameWindow
$timer=[Windows.Threading.DispatcherTimer]::new();$timer.Interval=[TimeSpan]::FromMilliseconds(100)
$script:lastStateAt=0;$script:lastSecond=0;$script:lastBarStep=0;$script:lastDisplayError=''
$script:stateWriteTicks=0;$script:cachedState=$null;$script:lastVisualKey=$null;$script:lastBounds=''
$script:gameProcess=$null;$script:gameKey=''
$timer.Add_Tick({
 try {
  $timer.Interval=[TimeSpan]::FromMilliseconds(100)
  if($engine.HasExited){DesktopClose;MeterClose;$window.Close();return}
  $stateFile.Refresh()
  if(-not $stateFile.Exists -or ([DateTime]::UtcNow-$stateFile.LastWriteTimeUtc).TotalSeconds -gt 1.2){
   if($window.IsVisible){$window.Hide()};MeterHide;DesktopHide;return
  }
  if($stateFile.LastWriteTimeUtc.Ticks -ne $script:stateWriteTicks){
   # Cache only complete parses; a failed or partial read hides the output.
   $script:cachedState=ReadFrameStateFile $StatePath
   $script:stateWriteTicks=$stateFile.LastWriteTimeUtc.Ticks
  }
  $state=$script:cachedState
  # The engine verifies the game executable. Optional background drawing still
  # binds to that exact HWND/process and client size; it never targets the app
  # that happened to receive focus. Keep a process handle to reject PID reuse.
  $data=$state.frameDisplay
  $handle=[IntPtr]([long]$data.window)
  $foreground=[LamdaFrameWindow]::GetForegroundWindow().ToInt64().ToString()
  [uint32]$owner=0;[void][LamdaFrameWindow]::GetWindowThreadProcessId($handle,[ref]$owner)
  $visible=[LamdaFrameWindow]::IsWindowVisible($handle);$minimized=[LamdaFrameWindow]::IsIconic($handle)
  if(-not (FrameFocusValid $data $data.window $owner $visible $minimized $true)){
   if($window.IsVisible){$window.Hide()};MeterHide;DesktopHide;return
  }
  $gameKey=$data.window+':'+$owner.ToString()
  if($gameKey -ne $script:gameKey -or ($script:gameProcess.HasExited -and -not $data.background -and $data.window -eq $foreground)){
   if($script:gameProcess){$script:gameProcess.Dispose()}
   $script:gameProcess=[Diagnostics.Process]::GetProcessById($owner)
   $null=$script:gameProcess.Handle;$script:gameKey=$gameKey
  }
  if($script:gameProcess.HasExited){if($window.IsVisible){$window.Hide()};MeterHide;DesktopHide;return}
  $rect=[LamdaFrameWindow+Rect]::new();$point=[LamdaFrameWindow+Point]::new()
  if(-not [LamdaFrameWindow]::GetClientRect($handle,[ref]$rect) -or -not [LamdaFrameWindow]::ClientToScreen($handle,[ref]$point)){
   if($window.IsVisible){$window.Hide()};MeterHide;DesktopHide;return
  }
  $now=[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()/1000.0
  $desktopReady=DesktopLoadForFrame
  $positioning=@($script:desktopPlacements.Values|Where-Object {$_.detached -and -not $_.locked}).Count -gt 0
  if(-not (FrameStateValid $state $now $EnginePID $handle.ToInt64().ToString() $rect.Right $rect.Bottom -AllowEmpty:$positioning)){
   if($window.IsVisible){$window.Hide()};MeterHide;DesktopHide;return
  }
  $data=$state.frameDisplay
  if(-not (FrameFocusValid $data $foreground $owner $visible $minimized $true)){
   if($window.IsVisible){$window.Hide()};MeterHide
   DesktopRefresh $data $now $point -Loaded
   return
  }
  $mainData=DesktopData $data '' -Remaining
  # Heartbeats change timestamps four times a second without changing a single
  # visible icon. Rebuild only for visual changes or a new countdown second.
  $visualKey=$script:desktopLayoutTicks.ToString()+':'+$data.width.ToString()+','+$data.height.ToString()+':'+(ConvertTo-Json -InputObject @($mainData.icons,$mainData.kickBars,$mainData.kickPanel,$mainData.notices,$mainData.overlays,$data.captureAreas) -Depth 8 -Compress)
  if($data.window -eq $foreground){OverlaySounds $data $now}
  $barStep=0;$animated=$false
  foreach($v in $data.overlays){if(OverlayAnimated $v $now){$animated=$true;break}}
  $rate=FrameRefreshRate $data $animated;$timer.Interval=[TimeSpan]::FromMilliseconds(1000.0/$rate)
  if($null -ne $data.overlays -and @($data.overlays).Count -gt 0){$barStep=[Math]::Floor($now*$rate)}
  if($barStep -eq 0){foreach($bar in $data.kickBars){if($bar.status -eq 'estimated_cooldown'){$barStep=[Math]::Floor($now*5);break}}}
  if($visualKey -ne $script:lastVisualKey -or [Math]::Floor($now) -ne $script:lastSecond -or $barStep -ne $script:lastBarStep){
   $matrix=[Windows.PresentationSource]::FromVisual($window).CompositionTarget.TransformFromDevice
   $canvas=FrameCanvas $mainData $now -SeparateMeter
   $canvas.LayoutTransform=[Windows.Media.ScaleTransform]::new($matrix.M11,$matrix.M22)
   $window.Content=$canvas;$script:lastVisualKey=$visualKey;$script:lastSecond=[Math]::Floor($now);$script:lastBarStep=$barStep
  }
  $bounds=$point.X.ToString()+','+$point.Y.ToString()+','+$rect.Right.ToString()+','+$rect.Bottom.ToString()
  if(-not $window.IsVisible -or $bounds -ne $script:lastBounds){
   $ourHandle=[Windows.Interop.WindowInteropHelper]::new($window).Handle
   [void][LamdaFrameWindow]::SetWindowPos($ourHandle,[IntPtr](-1),$point.X,$point.Y,$rect.Right,$rect.Bottom,0x0010)
   $script:lastBounds=$bounds
  }
  MeterRefresh $mainData $now $point
  DesktopRefresh $data $now $point -Loaded
  FrameCaptureHoles $window $data
  if(-not $window.IsVisible){$window.Show()}
 } catch {
  if($window.IsVisible){$window.Hide()};MeterHide;DesktopHide;$message=$_.Exception.Message
  if($message -ne $script:lastDisplayError){$script:lastDisplayError=$message;[Console]::Error.WriteLine('Frame display: '+$message)}
 }
})
$window.Add_Closed({$timer.Stop();DesktopClose;MeterClose;if($script:gameProcess){$script:gameProcess.Dispose()}})
$app=[Windows.Application]::new();$app.ShutdownMode='OnMainWindowClose';$app.MainWindow=$window
try{$window.Show();$window.Hide();$timer.Start();[void]$app.Run()}
finally{$engine.Dispose();if($oldDPI -ne [IntPtr]::Zero){[void][LamdaFrameWindow]::SetThreadDpiAwarenessContext($oldDPI)}}
