# Native WPF regression: no overlay window, WoW access, or installed-file writes.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'frame-estimates.ps1') -StatePath 'unused' -LoadOnly
function FindTagged($node,[string]$tag) {
 if($node.Tag -eq $tag){return $node}
 if($node -is [Windows.Controls.Panel]) {
  foreach($child in $node.Children){$match=FindTagged $child $tag;if($match){return $match}}
 } elseif($node -is [Windows.Controls.Decorator] -and $node.Child) {
  return (FindTagged $node.Child $tag)
 }
 return $null
}
$checks=0
$cases=@(
 @{status='unknown';ends=95;reasons=@('arrived_after_estimated_end');text='Calibrating';detail='Watching for a cast or cooldown window';filled=$false},
 @{status='unknown';ends=25;reasons=@('observation_stale');text='Calibrating';detail='Watching for a cast or cooldown window';filled=$false},
 @{status='unknown';ends=0;reasons=@('cooldown_not_observed');text='Calibrating';detail='Watching for a cast or cooldown window';filled=$false},
 @{status='calibrating';ends=112;reasons=@('party_cooldown_calibration');text='Calibrating';detail='Watching casts; Ready in ~12s';filled=$false},
 @{status='estimated_cooldown';ends=104;reasons=@('late_delivery');text='~ 4s';detail='Wind Shear';filled=$true},
 @{status='estimated_ready';ends=99;reasons=@();text='Ready';detail='Wind Shear';filled=$true}
)
foreach($case in $cases) {
 $bar=[pscustomobject]@{x=0;y=24;width=250;height=30;opacity=100;status=$case.status;ends=$case.ends;duration=12;
  color='#0070DD';spell=57994;player='Teammate';name='Wind Shear';deliverySeconds=8;reasons=$case.reasons}
 $data=[pscustomobject]@{width=800;height=600;icons=@();kickBars=@($bar);kickPanel=$null}
 $canvas=FrameCanvas $data 100
 if((FindTagged $canvas 'kick-timer').Text -cne $case.text){throw "Incorrect timer: $($case.text)"};$checks++
 if((FindTagged $canvas 'kick-detail').Text -cne $case.detail){throw "Incorrect detail: $($case.text)"};$checks++
 if([bool](FindTagged $canvas 'kick-fill') -ne $case.filled){throw "Incorrect availability fill: $($case.text)"};$checks++
 if($null -eq (FindTagged $canvas 'kick-art').Source){throw 'Missing interrupt texture'};$checks++
 $canvas.Measure([Windows.Size]::new(800,600));$canvas.Arrange([Windows.Rect]::new(0,0,800,600));$canvas.UpdateLayout()
 $label=FindTagged $canvas 'kick-timer'
 $formatted=[Windows.Media.FormattedText]::new($label.Text,[Globalization.CultureInfo]::InvariantCulture,
  [Windows.FlowDirection]::LeftToRight,[Windows.Media.Typeface]::new($label.FontFamily,$label.FontStyle,$label.FontWeight,$label.FontStretch),
  $label.FontSize,$label.Foreground)
 if($formatted.Width -gt $label.ActualWidth){throw "Clipped timer: $($case.text)"};$checks++
}
# Exercise zero, misses, exact successes and larger dungeon totals, at both
# the new count gutter and the old addon gutter before the player reloads.
foreach($x in @(90,28)) {
 $bar.x=$x
 foreach($fraction in @(@(0,0),@(0,1),@(3,4),@(7,7),@(123,150),@(9999,10000))) {
  $bar | Add-Member -NotePropertyName count -NotePropertyValue $fraction[0] -Force
  $bar | Add-Member -NotePropertyName attempts -NotePropertyValue $fraction[1] -Force
  $canvas=FrameCanvas $data 100
  $count=FindTagged $canvas 'kick-count'
  if($null -eq $count -or $count.Text -cne ($fraction -join '/')){throw 'Missing/incorrect fraction'};$checks++
  if([Windows.Controls.Canvas]::GetLeft($count) -ne [Math]::Max(0,$x-72)){throw 'Count not left of bar'};$checks++
  if([Windows.Controls.Canvas]::GetTop($count) -ne $bar.y){throw 'Count on wrong row'};$checks++
  $canvas.Measure([Windows.Size]::new(800,600));$canvas.Arrange([Windows.Rect]::new(0,0,800,600));$canvas.UpdateLayout()
  $formatted=[Windows.Media.FormattedText]::new($count.Text,[Globalization.CultureInfo]::InvariantCulture,
   [Windows.FlowDirection]::LeftToRight,[Windows.Media.Typeface]::new($count.FontFamily,$count.FontStyle,$count.FontWeight,$count.FontStretch),
   $count.FontSize,$count.Foreground)
  if($formatted.Width -gt $count.ActualWidth+.1){throw "Clipped fraction: $($count.Text) at gutter $x"};$checks++
  if([Windows.Controls.Canvas]::GetLeft((FindTagged $canvas 'kick-bar')) -ne $bar.x){throw 'Count shifted the bar'};$checks++
 }
}
$bar.PSObject.Properties.Remove('attempts')
if((FindTagged (FrameCanvas $data 100) 'kick-count').Text -cne '9999'){throw 'Legacy count missing'};$checks++
$bar.PSObject.Properties.Remove('count')
if($null -ne (FindTagged (FrameCanvas $data 100) 'kick-count')){throw 'Disabled count rendered'};$checks++
$bar | Add-Member -NotePropertyName native -NotePropertyValue $true
$bar | Add-Member -NotePropertyName count -NotePropertyValue 7
$bar | Add-Member -NotePropertyName attempts -NotePropertyValue 10
$canvas=FrameCanvas $data 100
if($null -ne (FindTagged $canvas 'kick-bar')){throw 'Overlay covers native own row'};$checks++
if((FindTagged $canvas 'kick-count').Text -cne '7/10'){throw 'Native row lost fraction'};$checks++
$bar.native=$false;$bar.status='calibrating';$bar.ends=112;$bar.width=160
foreach($height in @(18,30,60)) {
 $bar.height=$height;$canvas=FrameCanvas $data 100
 $canvas.Measure([Windows.Size]::new(800,600));$canvas.Arrange([Windows.Rect]::new(0,0,800,600));$canvas.UpdateLayout()
 $label=FindTagged $canvas 'kick-timer'
 $formatted=[Windows.Media.FormattedText]::new($label.Text,[Globalization.CultureInfo]::InvariantCulture,
  [Windows.FlowDirection]::LeftToRight,[Windows.Media.Typeface]::new($label.FontFamily,$label.FontStyle,$label.FontWeight,$label.FontStretch),
  $label.FontSize,$label.Foreground)
 if($formatted.Width -gt $label.ActualWidth){throw "Clipped Calibrating at height $height"};$checks++
}
$focus=[pscustomobject]@{window='123';windowPID=42;keepUIWhenUnfocused=$false}
if(-not (FrameFocusValid $focus '123' 42 $true $false $true)){throw 'Focused game hidden'};$checks++
if(FrameFocusValid $focus '456' 42 $true $false $true){throw 'Default allows background'};$checks++
$focus.keepUIWhenUnfocused=$true
if(-not (FrameFocusValid $focus '456' 42 $true $false $true)){throw 'Opt-in background hidden'};$checks++
foreach($case in @(@{owner=7;visible=$true;minimized=$false;alive=$true},
 @{owner=42;visible=$false;minimized=$false;alive=$true},
 @{owner=42;visible=$true;minimized=$true;alive=$true},
 @{owner=42;visible=$true;minimized=$false;alive=$false})) {
 if(FrameFocusValid $focus '456' $case.owner $case.visible $case.minimized $case.alive){throw 'Invalid background target shown'};$checks++
}
$state=[pscustomobject]@{pid=1;frameDisplay=[pscustomobject]@{enabled=$true;window='123';width=800;height=600;at=100;expires=101.2;icons=@();kickPanel=@{}}}
if(-not (FrameStateValid $state 100.5 1 '123' 800 600)){throw 'Fresh game rejected'};$checks++
foreach($stateCase in @(@(102,1,'123',800,600),@(100.5,2,'123',800,600),@(100.5,1,'456',800,600),@(100.5,1,'123',900,600))) {
 if(FrameStateValid $state $stateCase[0] $stateCase[1] $stateCase[2] $stateCase[3] $stateCase[4]){throw 'Invalid state/resize accepted'};$checks++
}
$notice=[pscustomobject]@{text='Fire locked ~';x=200;y=100;width=230;opacity=80;ends=106;countdown=$true}
$data=[pscustomobject]@{width=800;height=600;icons=@();notices=@($notice)}
$canvas=FrameCanvas $data 101
if((FindTagged $canvas 'arena-notice').Text -cne 'Fire locked ~5s'){throw 'Arena lockout countdown missing'};$checks++
if($null -ne (FindTagged (FrameCanvas $data 107) 'arena-notice')){throw 'Expired arena notice remains'};$checks++
$notice.countdown=$false;$notice.text='Divine Shield'
if((FindTagged (FrameCanvas $data 101) 'arena-notice').Text -cne 'Divine Shield'){throw 'Local arena announcement missing'};$checks++
$state.frameDisplay.kickPanel=$null;$state.frameDisplay | Add-Member -NotePropertyName notices -NotePropertyValue @($notice)
if(-not (FrameStateValid $state 100.5 1 '123' 800 600)){throw 'Notice-only arena display rejected'};$checks++
$catalog=Get-Content (Join-Path $PSScriptRoot '../internal/arena/catalog.json') -Raw | ConvertFrom-Json
foreach($property in $catalog.PSObject.Properties){
 if($null -eq (FrameIconImage ([int]$property.Name))){throw "Missing arena art $($property.Name)"};$checks++
}
Write-Output "Passed $checks native WPF overlay checks."
foreach($kind in @('icon','text','bar','arc','ring')) {
 $v=[pscustomobject]@{id='test';kind=$kind;text='Move now';size=96;font=24;opacity=90;color='#F4C879';transparent=$true;glow=$true;duration=5;ends=105;x=98;y=98}
 $d=[pscustomobject]@{width=800;height=600;overlays=@($v)}
 $canvas=FrameCanvas $d 100
 $item=FindTagged $canvas ('overlay-'+$kind)
 if($null -eq $item){throw "Missing custom $kind"}
 $canvas.Measure([Windows.Size]::new(800,600));$canvas.Arrange([Windows.Rect]::new(0,0,800,600));$canvas.UpdateLayout()
 if([Windows.Controls.Canvas]::GetLeft($item)+$item.ActualWidth -gt 800.1 -or [Windows.Controls.Canvas]::GetTop($item)+$item.ActualHeight -gt 600.1){throw 'Custom display leaves the game window'}
 if($null -ne (FindTagged (FrameCanvas $d 106) ('overlay-'+$kind))){throw 'Expired custom display remained'}
}
$customState=[pscustomobject]@{pid=1;frameDisplay=[pscustomobject]@{enabled=$true;window='123';width=800;height=600;at=100;expires=101.2;icons=@();overlays=@($v)}}
if(-not(FrameStateValid $customState 100.2 1 '123' 800 600)){throw 'Custom-only display rejected'}
if(FrameStateValid $customState 102 1 '123' 800 600){throw 'Stale custom display allowed'}
Write-Host 'Passed 17 custom overlay shape, placement and lifetime checks.'
# Persistent shapes keep a full fill while their short observation lease is renewed.
foreach($kind in @('icon','text','bar','arc','ring')) {
 $v=[pscustomobject]@{id='persistent';kind=$kind;text='Low mana';size=96;font=24;opacity=100;color='#65DDC7';transparent=$true;glow=$false;persistent=$true;duration=1.2;ends=101.2;x=50;y=50}
 $item=CustomOverlay $v 101.1
 if($null -eq $item){throw "Persistent $kind disappeared within its lease"}
 if($kind -eq 'bar' -and $item.Child.Children[0].Width -ne $item.Width){throw 'Persistent bar became a countdown'}
 if($kind -in @('arc','ring') -and -not $item.Child.Children[0].Data.Figures[0].Segments[0].IsLargeArc){throw 'Persistent shape became a countdown'}
 if($null -ne (CustomOverlay $v 101.3)){throw 'Expired persistent lease remained visible'}
}
Write-Host 'Passed persistent shape and observation-lease checks.'

# Text font size must scale the letters and their available height together.
$previousHeight=0;$previousWidth=0
foreach($font in @(12,24,48,72)) {
 $v=[pscustomobject]@{id='font';kind='text';text='Low mana';size=46;font=$font;opacity=100;color='#65DDC7';transparent=$true;glow=$false;duration=6;ends=106;x=20;y=20}
 $item=CustomOverlay $v 100;$item.Measure([Windows.Size]::new(1920,1080));$item.Arrange([Windows.Rect]::new(0,0,$item.Width,$item.Height));$item.UpdateLayout()
 $label=$item.Child.Children[0]
 if($label.FontSize -ne $font -or $label.ActualHeight -le $previousHeight -or $label.DesiredSize.Width -le $previousWidth -or $label.ActualHeight -gt $item.Height){throw 'Text font failed to grow in both directions'}
 $previousHeight=$label.ActualHeight;$previousWidth=$label.DesiredSize.Width
}
Write-Host 'Passed text font sizing and clipping checks.'

function RenderFramePixels($data) {
 $canvas=FrameCanvas $data 100
 $canvas.Measure([Windows.Size]::new($data.width,$data.height));$canvas.Arrange([Windows.Rect]::new(0,0,$data.width,$data.height));$canvas.UpdateLayout()
 $bitmap=[Windows.Media.Imaging.RenderTargetBitmap]::new($data.width,$data.height,96,96,[Windows.Media.PixelFormats]::Pbgra32)
 $bitmap.Render($canvas)
 $pixels=[byte[]]::new($data.width*$data.height*4);$bitmap.CopyPixels($pixels,$data.width*4,0)
 return ,$pixels
}
# Reproduce the actual alert: only the empty corner of its layout box touches
# Sim's capture area. The rendered text must remain byte-for-byte unchanged.
$v=[pscustomobject]@{id='mana';kind='text';text='NEED TO DRINK!!!!!';size=46;font=59;opacity=100;color='#F4C879';transparent=$true;glow=$false;persistent=$true;duration=1.2;ends=101.2;x=25.663325471698112;y=23.045987060674943}
$d=[pscustomobject]@{width=1920;height=1080;overlays=@($v);captureAreas=@()}
$before=RenderFramePixels $d
if(-not ($before | Where-Object {$_ -ne 0} | Select-Object -First 1)){throw 'Mana text has no visible pixels'}
$d.captureAreas=@([pscustomobject]@{x=288;y=224;width=256;height=32})
$after=RenderFramePixels $d
if([Convert]::ToBase64String($before) -cne [Convert]::ToBase64String($after)){throw 'Sim reservation changed the visible mana text'}

# Real intersections clip only the protected pixels, including glow and opaque
# backgrounds. Overlapping reservations must stay empty rather than cancel out.
foreach($kind in @('text','bar','icon','ring','arc')) {
 $v.kind=$kind;$v.size=100;$v.font=24;$v.text='Alert';$v.x=10;$v.y=10;$v.transparent=$false;$v.glow=$true
 $d=[pscustomobject]@{width=320;height=240;overlays=@($v);captureAreas=@(
  [pscustomobject]@{x=40;y=30;width=35;height=35},
  [pscustomobject]@{x=50;y=40;width=35;height=35})}
 $pixels=RenderFramePixels $d
 foreach($area in $d.captureAreas) {
  for($y=$area.y;$y -lt $area.y+$area.height;$y++) {
   for($x=$area.x;$x -lt $area.x+$area.width;$x++) {
    if($pixels[($y*$d.width+$x)*4+3] -ne 0){throw "$kind painted over capture at $x,$y"}
   }
  }
 }
 if($pixels[(28*$d.width+36)*4+3] -eq 0){throw "$kind disappeared outside the capture area"}
}
Write-Host 'Passed mana text preservation and rendered capture-pixel exclusion checks.'

# Stack arches: verify actual pixels at left, right, top and bottom, in order.
$a=[pscustomobject]@{start=0;sweep=70;thickness=8;gap=6;reverse=$false;layout='around';pair='sides';count=1;capacity=4;fraction=1;timed=$false}
$v=[pscustomobject]@{id='arches';kind='arc';text='';size=200;font=14;opacity=100;color='#65DDC7';transparent=$true;glow=$false;persistent=$true;duration=6;ends=106;x=0;y=0;arc=$a}
$d=[pscustomobject]@{width=240;height=240;overlays=@($v);captureAreas=@()}
$positions=@(@(14,100),@(186,100),@(100,14),@(100,186))
foreach($count in @(1,2,3,4)) {
 $a.count=$count;$item=CustomOverlay $v 100
 if($item.Child.Children.Count -ne $count){throw 'Incorrect arch count'}
 $pixels=RenderFramePixels $d
 for($i=0;$i -lt 4;$i++){
  $point=$positions[$i];$painted=$pixels[($point[1]*240+$point[0])*4+3] -gt 0
  if($painted -ne ($i -lt $count)){throw "Incorrect arch placement for $count stacks at position $i"}
 }
}
$a.capacity=2;$a.count=2;$a.sweep=160;$a.pair='vertical';$pixels=RenderFramePixels $d
for($i=0;$i -lt 4;$i++){$point=$positions[$i];if(($pixels[($point[1]*240+$point[0])*4+3] -gt 0) -ne ($i -ge 2)){throw 'Vertical pair position incorrect'}}
$a.pair='sides';$a.count=1;$a.fraction=.5;$a.reverse=$true
$item=CustomOverlay $v 100;$segment=$item.Child.Children[0].Data.Figures[0].Segments[0]
if($segment.SweepDirection -ne 'Counterclockwise' -or [Math]::Abs($segment.Point.X-14) -gt .001 -or [Math]::Abs($segment.Point.Y-100) -gt .001){throw 'Reversed half-filled arch geometry incorrect'}
$a.fraction=0;if($null -ne (CustomOverlay $v 100)){throw 'Zero fill visible'}
$a.fraction=1;$a.count=0;if($null -ne (CustomOverlay $v 100)){throw 'Zero stacks visible'}
$a.count=2;$a.timed=$true;$v.persistent=$false;$item=CustomOverlay $v 103
$end=$item.Child.Children[0].Data.Figures[0].Segments[0].Point
if([Math]::Abs($end.X-14) -gt .001 -or [Math]::Abs($end.Y-100) -gt .001){throw 'Timer did not drain to half'}
$a.timed=$false;$a.layout='nested';$a.capacity=20;$a.count=20;$v.size=16
$item=CustomOverlay $v 100
foreach($path in $item.Child.Children){if($path.Data.Figures[0].Segments[0].Size.Width -le 0 -or $path.StrokeThickness -le 0){throw 'Crowded arch invalid geometry'}}
if($item.Child.Children.Count -ne 20){throw 'Crowded stack count lost'}
Write-Host 'Passed stack arch pixels, horizontal/vertical pairs, fill, direction, timer and crowded geometry checks.'
