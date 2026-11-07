# Shared desktop placement for every Engine-rendered display. Capture remains
# bound to the verified game; this file contains no game input or settings writes.
Add-Type -AssemblyName System.Windows.Forms
$script:desktopWindows=@{};$script:desktopPlacements=@{};$script:desktopRuleKeys=@{};$script:desktopNames=@{}
$script:desktopLayoutPath=Join-Path (Split-Path $StatePath) 'overlay-windows.json'
$script:desktopLayoutTicks=-1;$script:desktopRulesTicks=-1;$script:desktopEditing=$null
function DesktopMonitors {
 return @([Windows.Forms.Screen]::AllScreens | ForEach-Object {
  [pscustomobject]@{id=$_.DeviceName;primary=$_.Primary;x=$_.WorkingArea.X;y=$_.WorkingArea.Y;width=$_.WorkingArea.Width;height=$_.WorkingArea.Height;label=($_.DeviceName+'  '+$_.Bounds.Width+'x'+$_.Bounds.Height+$(if($_.Primary){' (primary)'}))}
 })
}
function DesktopReadPlacements {
 $out=@{}
 if(-not [IO.File]::Exists($script:desktopLayoutPath)){return ,$out}
 $info=[IO.FileInfo]::new($script:desktopLayoutPath);if($info.Length -gt 131072){throw 'Overlay window settings exceed limit'}
 $doc=ReadFrameStateFile $script:desktopLayoutPath
 if($doc.schema -ne 1 -or @($doc.windows.PSObject.Properties).Count -gt 240){throw 'Invalid overlay window settings'}
 foreach($entry in $doc.windows.PSObject.Properties){
  if($entry.Name -notmatch '^(meters[2-8]?|cd|kick|arena|overlay/[A-Za-z0-9_.:-]{1,100}|group/[A-Za-z0-9_.:-]{1,100})$'){throw 'Invalid overlay window identity'}
  $p=$entry.Value
  if($p.detached -isnot [bool] -or $p.locked -isnot [bool] -or [string]$p.monitor -notmatch '^\\\\\.\\DISPLAY[0-9]+$'){throw 'Invalid overlay window monitor'}
  foreach($key in @('x','y','workX','workY','width','height')){
   $v=$p.$key
   if($null -eq $v -or $v -is [string] -or $v -is [bool] -or [double]::IsNaN([double]$v) -or [double]::IsInfinity([double]$v) -or [Math]::Abs([double]$v) -gt 131072){throw 'Invalid overlay window position'}
  }
  if($p.width -lt 0 -or $p.width -gt 8192 -or $p.height -lt 0 -or $p.height -gt 8192){throw 'Invalid overlay window size'}
  $out[$entry.Name]=$p
 }
 return ,$out
}
function DesktopSave([string]$id,$placement) {
 # The manager and renderer can both edit. Merge under one per-client mutex,
 # then publish a complete file so dragging cannot erase another window's move.
 $sha=[Security.Cryptography.SHA256]::Create()
 try{$hash=[BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes([IO.Path]::GetFullPath($script:desktopLayoutPath).ToLowerInvariant()))).Replace('-','')}finally{$sha.Dispose()}
 $mutex=[Threading.Mutex]::new($false,('Local\LamdaUI.OverlayWindows.'+$hash))
 $held=$false;$tmp=$script:desktopLayoutPath+'.'+[Guid]::NewGuid().ToString('N')+'.tmp'
 try{
  try{$held=$mutex.WaitOne(1000)}catch [Threading.AbandonedMutexException]{$held=$true}
  if(-not $held){throw 'Another overlay window is being moved. Try again.'}
  $entries=DesktopReadPlacements;$entries[$id]=$placement
  [IO.Directory]::CreateDirectory((Split-Path $script:desktopLayoutPath))|Out-Null
  [IO.File]::WriteAllText($tmp,(@{schema=1;windows=$entries}|ConvertTo-Json -Depth 8 -Compress),[Text.UTF8Encoding]::new($false))
  for($attempt=0;$attempt -lt 10;$attempt++){
   try{if([IO.File]::Exists($script:desktopLayoutPath)){[IO.File]::Replace($tmp,$script:desktopLayoutPath,[NullString]::Value)}else{[IO.File]::Move($tmp,$script:desktopLayoutPath)};break}
   catch [IO.IOException]{if($attempt -eq 9){throw};Start-Sleep -Milliseconds 15}
  }
  $script:desktopLayoutTicks=-1;DesktopLoad
 }finally{if([IO.File]::Exists($tmp)){[IO.File]::Delete($tmp)};if($held){$mutex.ReleaseMutex()};$mutex.Dispose()}
}
function DesktopLoad {
 $info=[IO.FileInfo]::new($script:desktopLayoutPath);$ticks=0;if($info.Exists){$ticks=$info.LastWriteTimeUtc.Ticks}
 if($ticks -ne $script:desktopLayoutTicks){
  $script:desktopPlacements=DesktopReadPlacements;$script:desktopLayoutTicks=$ticks
 }
 $script:desktopNames=@{meters='LamdaMeters';cd='LamdaCD';kick='LamdaKick';arena='LamdaGladius'};for($i=2;$i -le 8;$i++){$script:desktopNames['meters'+$i]='LamdaMeters '+$i}
 $rules=Join-Path (Split-Path $StatePath) 'overlays\rules.json';$ri=[IO.FileInfo]::new($rules)
 if($ri.Exists -and $ri.Length -le 4194304){
  if($ri.LastWriteTimeUtc.Ticks -ne $script:desktopRulesTicks){
   $doc=ReadFrameStateFile $rules;$script:desktopRuleKeys=@{};$script:desktopRuleNames=@{}
   foreach($r in $doc.rules){
    $id='overlay/'+$r.id;$name=[string]$r.name
    if($r.style.group.id){$id='group/'+$r.style.group.id;$name=[string]$r.style.group.name}
    $script:desktopRuleKeys[[string]$r.id]=$id;$script:desktopRuleNames[$id]=$name
   }
   $script:desktopRulesTicks=$ri.LastWriteTimeUtc.Ticks
  }
  foreach($id in $script:desktopRuleNames.Keys){$script:desktopNames[$id]=$script:desktopRuleNames[$id]}
 }
 foreach($id in $script:desktopPlacements.Keys){if(-not $script:desktopNames.ContainsKey($id)){$script:desktopNames[$id]=$id}}
}
function DesktopLoadForFrame {
 try{DesktopLoad;$script:desktopConfigError='';return $true}
 catch{
  DesktopHide;$script:desktopPlacements=@{}
  $message=$_.Exception.Message
  if($message -ne $script:desktopConfigError){[Console]::Error.WriteLine('Overlay windows: '+$message);$script:desktopConfigError=$message}
  # A bad desktop layout must not disable unrelated displays or rewrite the
  # owner's settings. Render everything at its original in-game placement.
  return $false
 }
}
function DesktopOverlayKey($v) {
 if(([string]$v.id).StartsWith('lamdaaug/')){return 'meters'}
 $rule=[string]$v.rule
 if($script:desktopRuleKeys.ContainsKey($rule)){return $script:desktopRuleKeys[$rule]}
 if($rule -match '^[A-Za-z0-9_.:-]{1,100}$'){return 'overlay/'+$rule}
 return ''
}
function DesktopIconKey($v) {if($v.module -eq 'arena'){return 'arena'};if($v.kick){return 'kick'};return 'cd'}
function DesktopDetached([string]$id) {return $script:desktopPlacements.ContainsKey($id) -and $script:desktopPlacements[$id].detached}
function DesktopData($data,[string]$id,[switch]$Remaining) {
 $copy=@{};foreach($p in $data.PSObject.Properties){$copy[$p.Name]=$p.Value}
 $copy.icons=@($data.icons | Where-Object {$key=DesktopIconKey $_;if($Remaining){-not (DesktopDetached $key)}else{$key -eq $id}})
 $copy.overlays=@($data.overlays | Where-Object {$key=DesktopOverlayKey $_;if($Remaining){-not (DesktopDetached $key)}else{$key -eq $id}})
 $takeKick=if($Remaining){-not (DesktopDetached 'kick')}else{$id -eq 'kick'}
 $copy.kickBars=@();$copy.kickPanel=$null;if($takeKick){$copy.kickBars=@($data.kickBars);$copy.kickPanel=$data.kickPanel}
 $copy.notices=@();if(($Remaining -and -not (DesktopDetached 'arena')) -or (-not $Remaining -and $id -eq 'arena')){$copy.notices=@($data.notices)}
 $copy.meter=$null;if(($Remaining -and -not (DesktopDetached 'meters')) -or (-not $Remaining -and $id -match '^meters[2-8]?$')){$copy.meter=$data.meter}
 if($data.meters){
  $copy.meters=@();$copy.meter=$null
  foreach($m in $data.meters){$key=if([int]$m.id -eq 1){'meters'}else{'meters'+[int]$m.id}
   if($Remaining){if(-not (DesktopDetached $key)){$copy.meters+=,$m}}
   elseif($key -eq $id){$copy.meter=$m}
  }
 }
 if(-not $Remaining){$copy.captureAreas=@();$copy.overlaySounds=@()}
 return [pscustomobject]$copy
}
function DesktopBounds($data,[double]$now) {
 if($data.meter){return [pscustomobject]@{x=[double]$data.meter.x;y=[double]$data.meter.y+24;width=[double]$data.meter.width;height=[Math]::Max(96,[double]$data.meter.height-24)}}
 $rects=[Collections.Generic.List[object]]::new()
 foreach($v in $data.overlays){$c=CustomOverlay $v $now;if($c){$x=[Math]::Max(0.0,[Math]::Min($data.width-$c.Width,$v.x/100*$data.width));$y=[Math]::Max(0.0,[Math]::Min($data.height-$c.Height,$v.y/100*$data.height));if($null -ne $v.pixelX){$x=$v.pixelX};if($null -ne $v.pixelY){$y=$v.pixelY};$rects.Add(@{x=$x;y=$y;width=$c.Width;height=$c.Height})}}
 foreach($v in $data.icons){$rects.Add(@{x=$v.x;y=$v.y;width=$v.size;height=$v.size})}
 foreach($v in $data.kickBars){$rects.Add(@{x=[Math]::Max(0,$v.x-72);y=$v.y;width=$v.width+[Math]::Min(72,$v.x);height=$v.height})}
 if($data.kickPanel){$v=$data.kickPanel;$rects.Add(@{x=$v.x;y=$v.y;width=$v.width;height=22})}
 foreach($v in $data.notices){$rects.Add(@{x=$v.x;y=$v.y;width=$v.width;height=18})}
 if($rects.Count -eq 0){return $null}
 $left=[double]::PositiveInfinity;$top=$left;$right=[double]::NegativeInfinity;$bottom=$right
 foreach($r in $rects){$left=[Math]::Min($left,$r.x);$top=[Math]::Min($top,$r.y);$right=[Math]::Max($right,$r.x+$r.width);$bottom=[Math]::Max($bottom,$r.y+$r.height)}
 return [pscustomobject]@{x=$left;y=$top;width=[Math]::Max(1,$right-$left);height=[Math]::Max(1,$bottom-$top)}
}
function DesktopResolve($p,$bounds,$monitors) {
 $screen=@($monitors|Where-Object {$_.id -eq $p.monitor})|Select-Object -First 1
 $missing=-not $screen;if($missing){$screen=@($monitors|Where-Object {$_.primary})|Select-Object -First 1};if(-not $screen){throw 'No display is available'}
 $width=$p.width;$height=$p.height
 if($width -le 0 -or $height -le 0){$width=[Math]::Max(200,[Math]::Min($screen.width,$bounds.width));$height=[Math]::Max(100,[Math]::Min($screen.height,$bounds.height+24))}
 $width=[Math]::Min($screen.width,[Math]::Max(160,$width));$height=[Math]::Min($screen.height,[Math]::Max(80,$height))
 $x=$p.x+$screen.x-$p.workX;$y=$p.y+$screen.y-$p.workY
 if($missing){$x=$screen.x+40;$y=$screen.y+40}
 # Keep the title reachable. Unchanged monitor layouts preserve signed physical
 # coordinates, including placements spanning two monitors.
 $reachable=$false
 foreach($m in $monitors){if([Math]::Min($x+$width,$m.x+$m.width)-[Math]::Max($x,$m.x) -ge [Math]::Min(80,$width) -and $y -ge $m.y -and $y+24 -le $m.y+$m.height){$reachable=$true}}
 if(-not $reachable){$x=[Math]::Max($screen.x,[Math]::Min($screen.x+$screen.width-$width,$x));$y=[Math]::Max($screen.y,[Math]::Min($screen.y+$screen.height-$height,$y))}
 return [pscustomobject]@{x=[int]$x;y=[int]$y;width=[int]$width;height=[int]$height;monitor=$screen.id}
}
function DesktopWindowSave($w) {
 $r=[LamdaFrameWindow+Rect]::new();$handle=[Windows.Interop.WindowInteropHelper]::new($w).Handle
 if(-not [LamdaFrameWindow]::GetWindowRect($handle,[ref]$r)){return}
 $s=[Windows.Forms.Screen]::FromRectangle([Drawing.Rectangle]::new($r.Left,$r.Top,$r.Right-$r.Left,$r.Bottom-$r.Top))
 $old=$script:desktopPlacements[$w.Tag.id]
 DesktopSave $w.Tag.id ([pscustomobject]@{detached=$true;locked=[bool]$old.locked;monitor=$s.DeviceName;x=$r.Left;y=$r.Top;workX=$s.WorkingArea.X;workY=$s.WorkingArea.Y;width=$r.Right-$r.Left;height=$r.Bottom-$r.Top})
 if($w.Tag.id -match '^meters[2-8]?$' -and -not $w.Tag.navigation){DesktopMeterSettings '' ($r.Right-$r.Left) ($r.Bottom-$r.Top)}
}
function DesktopMeterSettings([string]$scope,[int]$width=0,[int]$height=0) {
 $dir=Join-Path (Split-Path $StatePath) 'augmentation';[IO.Directory]::CreateDirectory($dir)|Out-Null
 $path=Join-Path $dir 'display.json';$value=@{schema=1;x=0.02;y=0.25;width=380;height=220;locked=$false;scope='boss'}
 if([IO.File]::Exists($path)){ $old=ReadFrameStateFile $path;foreach($key in @('x','y','width','height','locked','scope','autoToken')){if($null -ne $old.$key){$value[$key]=$old.$key}} }
 if($scope){$value.scope=$scope};if($width -gt 0){$value.width=[Math]::Max(300,[Math]::Min(1000,$width));$value.height=[Math]::Max(120,[Math]::Min(1000,$height))}
 $tmp=$path+'.'+[Guid]::NewGuid().ToString('N')+'.tmp'
 try{
  [IO.File]::WriteAllText($tmp,($value|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false))
  for($attempt=0;$attempt -lt 10;$attempt++){try{if([IO.File]::Exists($path)){[IO.File]::Replace($tmp,$path,[NullString]::Value)}else{[IO.File]::Move($tmp,$path)};break}catch [IO.IOException]{if($attempt -eq 9){throw};Start-Sleep -Milliseconds 15}}
 }finally{if([IO.File]::Exists($tmp)){[IO.File]::Delete($tmp)}}
}
function MeterIconButton([string]$kind,[string]$tooltip,[int]$right=2) {
 $button=[Windows.Controls.Button]::new();$button.Width=20;$button.Height=20;$button.Margin="0,2,$right,0";$button.HorizontalAlignment='Right';$button.VerticalAlignment='Top';$button.Focusable=$false
 $button.Background=[Windows.Media.Brushes]::Transparent;$button.BorderThickness=0;$button.Opacity=.58;$button.ToolTip=$tooltip
 $paths=@{clock='M 10,2 A 8,8 0 1 1 9.99,2 M 10,5 L 10,10 14,12';chat='M 3,3 L 17,3 17,13 10,13 5,17 5,13 3,13 Z M 6,7 L 14,7 M 6,10 L 12,10';trophy='M 6,3 L 14,3 14,9 12,12 8,12 6,9 Z M 6,5 L 3,5 3,8 6,10 M 14,5 L 17,5 17,8 14,10 M 10,12 L 10,16 M 6,17 L 14,17'}
 $icon=[Windows.Shapes.Path]::new();$icon.Data=[Windows.Media.Geometry]::Parse($paths[$kind]);$icon.Stroke=FrameBrush '#C4D0DC';$icon.StrokeThickness=1.3;$icon.Width=16;$icon.Height=16;$icon.Stretch='Uniform';$icon.IsHitTestVisible=$false;$button.Content=$icon
 $template=[Windows.Controls.ControlTemplate]::new([Windows.Controls.Button]);$border=[Windows.FrameworkElementFactory]::new([Windows.Controls.Border]);$border.SetValue([Windows.Controls.Border]::BackgroundProperty,[Windows.TemplateBindingExtension]::new([Windows.Controls.Control]::BackgroundProperty));$border.SetValue([Windows.Controls.Border]::CornerRadiusProperty,[Windows.CornerRadius]::new(3))
 $presenter=[Windows.FrameworkElementFactory]::new([Windows.Controls.ContentPresenter]);$presenter.SetValue([Windows.FrameworkElement]::HorizontalAlignmentProperty,[Windows.HorizontalAlignment]::Center);$presenter.SetValue([Windows.FrameworkElement]::VerticalAlignmentProperty,[Windows.VerticalAlignment]::Center);$border.AppendChild($presenter);$template.VisualTree=$border;$button.Template=$template
 $button.Add_MouseEnter({param($sender,$event)$sender.Opacity=1;$sender.Background=FrameBrush '#302F4155'})
 $button.Add_MouseLeave({param($sender,$event)$sender.Opacity=.58;$sender.Background=[Windows.Media.Brushes]::Transparent})
 return $button
}
function MeterShareClick($sender,[ValidateSet('meter','awards')][string]$mode='meter') {
 try{
  if($sender.Tag -and $sender.Tag.windowID){MeterNativeAction $sender.Tag 'share' $mode;return}
  $sender.ToolTip='Reload LamdaMeters to enable sharing from this window.'
 }catch{$sender.ToolTip='Summary unavailable. Reload the addon and keep WoW focused.'}
}
function NewDesktopWindow([string]$id) {
 $w=[Windows.Window]::new();$w.Title=$script:desktopNames[$id];$w.WindowStyle='None';$w.ResizeMode='NoResize';$w.AllowsTransparency=$true
 $w.Background=FrameBrush '#ED0A1018';$w.Topmost=$true;$w.ShowInTaskbar=$false;$w.ShowActivated=$false;$w.Focusable=$false;$w.Width=1;$w.Height=1
 $w.Add_SourceInitialized({param($sender,$event)$h=[Windows.Interop.WindowInteropHelper]::new($sender).Handle;$s=[LamdaFrameWindow]::GetWindowLongPtr($h,-20).ToInt64();[void][LamdaFrameWindow]::SetWindowLongPtr($h,-20,[IntPtr]($s -bor 0x08000000 -bor 0x80))})
 $root=[Windows.Controls.Grid]::new();$root.ClipToBounds=$true;$w.Content=$root
 $row=[Windows.Controls.RowDefinition]::new();$row.Height=[Windows.GridLength]::new(24);[void]$root.RowDefinitions.Add($row);[void]$root.RowDefinitions.Add([Windows.Controls.RowDefinition]::new())
 $body=[Windows.Controls.Viewbox]::new();$body.Stretch='Uniform';$body.HorizontalAlignment='Left';$body.VerticalAlignment='Top';$body.IsHitTestVisible=$id -match '^meters[2-8]?$';[Windows.Controls.Grid]::SetRow($body,1);[void]$root.Children.Add($body)
 $title=[Windows.Controls.Border]::new();$title.Background=FrameBrush '#EE142234';$title.Cursor='SizeAll'
 $label=FrameLabel $w.Title 12 '#D9E5FF';$label.TextAlignment='Left';$label.Margin='6,0,0,0';$label.VerticalAlignment='Center';$title.Child=$label
 $title.Add_MouseLeftButtonDown({param($sender,$event)$w=[Windows.Window]::GetWindow($sender);if($script:desktopPlacements[$w.Tag.id].locked){return};$script:desktopEditing=$w.Tag.id;try{$w.DragMove();DesktopWindowSave $w}finally{$script:desktopEditing=$null};$event.Handled=$true})
 $menu=[Windows.Controls.ContextMenu]::new();$lock=[Windows.Controls.MenuItem]::new();$lock.Header='Lock position';$lock.IsCheckable=$true
 $lock.Add_Click({param($sender,$event)$w=$sender.Tag;$p=$script:desktopPlacements[$w.Tag.id];$p.locked=-not $p.locked;DesktopSave $w.Tag.id $p});$lock.Tag=$w;[void]$menu.Items.Add($lock)
 $reset=[Windows.Controls.MenuItem]::new();$reset.Header='Return to WoW';$reset.Tag=$w
 $reset.Add_Click({param($sender,$event)$w=$sender.Tag;$p=$script:desktopPlacements[$w.Tag.id];$p.detached=$false;DesktopSave $w.Tag.id $p;$w.Hide()});[void]$menu.Items.Add($reset)
 $title.ContextMenu=$menu;[void]$root.Children.Add($title)
 $scopeButton=$null;$shareButton=$null;$awardsButton=$null
 if($id -match '^meters[2-8]?$'){
  $scopeButton=MeterIconButton 'clock' 'Combat history'
  $scopeButton.ContextMenu=[Windows.Controls.ContextMenu]::new()
  $scopeButton.Add_Click({param($sender,$event)$sender.ContextMenu.PlacementTarget=$sender;$sender.ContextMenu.Placement='Bottom';$sender.ContextMenu.IsOpen=$true;$event.Handled=$true})
  [void]$root.Children.Add($scopeButton)
  $shareButton=MeterIconButton 'chat' 'Share this window' 24
  $shareButton.Add_Click({param($sender,$event)MeterShareClick $sender 'meter';$event.Handled=$true});[void]$root.Children.Add($shareButton)
  $awardsButton=MeterIconButton 'trophy' 'Share awards' 46
  $awardsButton.Add_Click({param($sender,$event)MeterShareClick $sender 'awards';$event.Handled=$true});[void]$root.Children.Add($awardsButton)
  $label.Margin='6,0,70,0'
 }
 $resize=[Windows.Controls.Primitives.Thumb]::new();$resize.Width=18;$resize.Height=18;$resize.HorizontalAlignment='Right';$resize.VerticalAlignment='Bottom';$resize.Cursor='SizeNWSE';$resize.Opacity=.6;[Windows.Controls.Grid]::SetRowSpan($resize,2)
 $resize.Add_DragStarted({param($sender,$event)$w=[Windows.Window]::GetWindow($sender);if(-not $script:desktopPlacements[$w.Tag.id].locked){$script:desktopEditing=$w.Tag.id}})
 $resize.Add_DragDelta({param($sender,$event)$w=[Windows.Window]::GetWindow($sender);if($script:desktopEditing -ne $w.Tag.id){return};$w.Width=[Math]::Max(160,$w.Width+$event.HorizontalChange);$w.Height=[Math]::Max(80,$w.Height+$event.VerticalChange)})
 $resize.Add_DragCompleted({param($sender,$event)$w=[Windows.Window]::GetWindow($sender);if($script:desktopEditing -eq $w.Tag.id){try{DesktopWindowSave $w}finally{$script:desktopEditing=$null}}})
 [void]$root.Children.Add($resize)
 $w.Tag=@{shareButton=$shareButton;awardsButton=$awardsButton;id=$id;root=$root;body=$body;title=$title;label=$label;lock=$lock;resize=$resize;scopeButton=$scopeButton;scopeKey='';key=''}
 return $w
}
function DesktopHide {foreach($w in $script:desktopWindows.Values){if($script:desktopEditing -ne $w.Tag.id){$w.Hide()}}}
function DesktopClose {foreach($w in $script:desktopWindows.Values){$w.Close()};$script:desktopWindows=@{}}
function DesktopRefresh($data,[double]$now,$point,[switch]$Loaded) {
 if(-not $Loaded){if(-not (DesktopLoadForFrame)){return}};$monitors=DesktopMonitors
 foreach($id in @($script:desktopWindows.Keys)){if(-not (DesktopDetached $id)){$script:desktopWindows[$id].Hide()}}
 foreach($id in @($script:desktopPlacements.Keys)){
  if(-not (DesktopDetached $id) -or $script:desktopEditing -eq $id){continue}
  $p=$script:desktopPlacements[$id];$part=DesktopData $data $id
  $step=[Math]::Floor($now)
  foreach($v in $part.overlays){if(OverlayAnimated $v $now){$step=[Math]::Floor($now*(FrameRefreshRate $data $true));break}}
  foreach($v in $part.kickBars){if($v.status -eq 'estimated_cooldown'){$step=[Math]::Max($step,[Math]::Floor($now*5));break}}
  $sourceKey=ConvertTo-Json -InputObject @($part.meter,$part.icons,$part.overlays,$part.kickBars,$part.kickPanel,$part.notices) -Depth 10 -Compress
  $old=$script:desktopWindows[$id];$b=$null
  $placeholder=-not ($part.meter -or @($part.icons).Count -gt 0 -or @($part.overlays).Count -gt 0 -or @($part.kickBars).Count -gt 0 -or $part.kickPanel -or @($part.notices).Count -gt 0)
  if($placeholder -and -not $p.locked){$b=[pscustomobject]@{x=0;y=0;width=300;height=72}}
  elseif(-not $placeholder){if($old -and -not $old.Tag.placeholder -and $old.Tag.sourceKey -eq $sourceKey){$b=$old.Tag.bounds}else{$b=DesktopBounds $part $now}}
  if(-not $b){if($script:desktopWindows.ContainsKey($id)){$script:desktopWindows[$id].Hide()};continue}
  if(-not $script:desktopWindows.ContainsKey($id)){$script:desktopWindows[$id]=NewDesktopWindow $id;$script:desktopWindows[$id].Show();$script:desktopWindows[$id].Hide()}
  $w=$script:desktopWindows[$id];$rect=DesktopResolve $p $b $monitors;$h=[Windows.Interop.WindowInteropHelper]::new($w).Handle
  $windowBounds=$rect.x.ToString()+','+$rect.y+','+$rect.width+','+$rect.height
  if($windowBounds -ne $w.Tag.windowBounds){[void][LamdaFrameWindow]::SetWindowPos($h,[IntPtr](-1),$rect.x,$rect.y,$rect.width,$rect.height,0x0010);$w.Tag.windowBounds=$windowBounds}
  $style=[LamdaFrameWindow]::GetWindowLongPtr($h,-20).ToInt64();$style=$style -band (-bnot 0x20);if($p.locked -and $id -notmatch '^meters[2-8]?$'){$style=$style -bor 0x20};[void][LamdaFrameWindow]::SetWindowLongPtr($h,-20,[IntPtr]$style)
  $matrix=[Windows.PresentationSource]::FromVisual($w).CompositionTarget.TransformFromDevice
  $w.Tag.root.LayoutTransform=[Windows.Media.ScaleTransform]::new($matrix.M11,$matrix.M22);$w.Tag.root.Width=$rect.width;$w.Tag.root.Height=$rect.height
  $w.Tag.lock.IsChecked=[bool]$p.locked;$w.Tag.resize.IsEnabled=-not $p.locked
  if($id -match '^meters[2-8]?$' -and $part.meter){
   $w.Tag.label.Text='LamdaMeters · '+$part.meter.metric;$w.Tag.navigation=$part.meter.body.navigation;$w.Tag.shareButton.Tag=$part.meter.body.navigation;$w.Tag.awardsButton.Tag=$part.meter.body.navigation
   $key=($part.meter.choices|ConvertTo-Json -Depth 4 -Compress)+'|'+$part.meter.scope
   if($key -ne $w.Tag.scopeKey){
    $w.Tag.scopeButton.ContextMenu.Items.Clear();$selected='Current'
    foreach($choice in $part.meter.choices){
     $item=[Windows.Controls.MenuItem]::new();$item.Header=[string]$choice.label;$item.Tag=@{scope=[string]$choice.scope;navigation=$part.meter.body.navigation};$item.IsCheckable=$true;$item.IsChecked=$choice.scope -eq $part.meter.scope
     if($item.IsChecked){$selected=[string]$choice.label}
     $item.Add_Click({param($sender,$event)if($sender.Tag.navigation){$scope=$sender.Tag.scope;$value=switch($scope){'boss'{'Current'}'overall'{'Overall'}'previous'{'Expired'}default{$scope}};MeterNativeAction $sender.Tag.navigation 'session' $value}else{DesktopMeterSettings ([string]$sender.Tag.scope)};$event.Handled=$true});[void]$w.Tag.scopeButton.ContextMenu.Items.Add($item)
    }
    $w.Tag.scopeButton.ToolTip='Combat history - '+$selected;$w.Tag.scopeKey=$key
   }
   foreach($item in $w.Tag.scopeButton.ContextMenu.Items){$item.Tag.navigation=$part.meter.body.navigation}
  }
  # Copy native-owned kick rows into the external presentation using their
  # existing Engine evidence. Native in-game controls are never moved.
  $visualKey=$sourceKey+'|'+$step+'|'+$rect.width+','+$rect.height+'|'+$matrix.M11+','+$matrix.M22
  if($visualKey -ne $w.Tag.key){
   $bars=@();foreach($bar in $part.kickBars){$copy=@{};foreach($v in $bar.PSObject.Properties){$copy[$v.Name]=$v.Value};$copy.native=$false;$bars+=([pscustomobject]$copy)};$part.kickBars=$bars
   if($placeholder){$label=FrameLabel 'Positioning' 14 '#B8CADB';$label.Width=300;$label.Height=72;$label.VerticalAlignment='Center';$w.Tag.body.Child=$label}
   else{
    $c=FrameCanvas $part $now -InteractiveMeter:($id -match '^meters[2-8]?$');$c.RenderTransform=[Windows.Media.TranslateTransform]::new(-$b.x,-$b.y)
    $crop=[Windows.Controls.Canvas]::new();$crop.Width=$b.width;$crop.Height=$b.height;$crop.ClipToBounds=$true;[void]$crop.Children.Add($c);$w.Tag.body.Child=$crop
   }
   $w.Tag.key=$visualKey;$w.Tag.sourceKey=$sourceKey;$w.Tag.bounds=$b;$w.Tag.placeholder=$placeholder
  }
  $regionKey=$windowBounds+'|'+$point.X+','+$point.Y+'|'+(ConvertTo-Json -InputObject @($data.captureAreas) -Depth 4 -Compress)
  if($regionKey -ne $w.Tag.regionKey){
   $holes=[Collections.Generic.List[LamdaFrameWindow+Rect]]::new()
   foreach($a in $data.captureAreas){$r=[LamdaFrameWindow+Rect]::new();$r.Left=$point.X+$a.x-$rect.x;$r.Top=$point.Y+$a.y-$rect.y;$r.Right=$r.Left+$a.width;$r.Bottom=$r.Top+$a.height;$holes.Add($r)}
   if(-not [LamdaFrameWindow]::CaptureHoles($h,$rect.width,$rect.height,$holes.ToArray())){throw 'Desktop capture region unavailable'}
   $w.Tag.regionKey=$regionKey
  }
  if(-not $w.IsVisible){$w.Show()}
 }
}
function ShowDesktopManager([switch]$RenderOnly) {
 DesktopLoad;$monitors=DesktopMonitors
 $w=[Windows.Window]::new();$w.Title='LamdaUI — Overlay windows';$w.Width=680;$w.Height=420;$w.ResizeMode='NoResize';$w.Background=FrameBrush '#101923';$w.Foreground=FrameBrush '#EAF0F7'
 $screen=@($monitors|Where-Object {-not $_.primary})|Select-Object -First 1;if(-not $screen){$screen=$monitors[0]};$w.WindowStartupLocation='Manual';$w.Left=$screen.x+40;$w.Top=$screen.y+40
 $w.Tag=@{targetX=$screen.x+40;targetY=$screen.y+40}
 $w.Add_SourceInitialized({param($sender,$event)$h=[Windows.Interop.WindowInteropHelper]::new($sender).Handle;[void][LamdaFrameWindow]::SetWindowPos($h,[IntPtr]::Zero,$sender.Tag.targetX,$sender.Tag.targetY,0,0,0x0015)})
 $panel=[Windows.Controls.StackPanel]::new();$panel.Margin='24';$w.Content=$panel
 $heading=FrameLabel 'Overlay windows' 22 '#EAF0F7';$heading.TextAlignment='Left';[void]$panel.Children.Add($heading)
 $caption=FrameLabel 'Display' 13 '#B8CADB';$caption.TextAlignment='Left';$caption.Margin='0,20,0,5';[void]$panel.Children.Add($caption)
 $list=[Windows.Controls.ComboBox]::new();$list.Height=28;$list.DisplayMemberPath='name';[void]$panel.Children.Add($list)
 $ids=@('meters','cd','kick','arena')+@($script:desktopNames.Keys|Where-Object {$_ -notin @('meters','cd','kick','arena')}|Sort-Object)
 foreach($id in $ids){$list.Items.Add([pscustomobject]@{id=$id;name=$script:desktopNames[$id]})|Out-Null};$list.SelectedIndex=0
 $screens=[Windows.Controls.ComboBox]::new();$screens.Height=28;$screens.Margin='0,14,0,0';$screens.DisplayMemberPath='label';foreach($m in $monitors){$screens.Items.Add($m)|Out-Null};$screens.SelectedIndex=0;[void]$panel.Children.Add($screens)
 for($i=0;$i -lt $screens.Items.Count;$i++){if($screens.Items[$i].id -eq $screen.id){$screens.SelectedIndex=$i}}
 $buttons=[Windows.Controls.StackPanel]::new();$buttons.Orientation='Horizontal';$buttons.Margin='0,18,0,0';[void]$panel.Children.Add($buttons)
 $move=[Windows.Controls.Button]::new();$move.Content='Move to monitor';$move.Width=160;$move.Height=32;[void]$buttons.Children.Add($move)
 $reset=[Windows.Controls.Button]::new();$reset.Content='Return to WoW';$reset.Width=140;$reset.Margin='10,0,0,0';[void]$buttons.Children.Add($reset)
 $lock=[Windows.Controls.Button]::new();$lock.Content='Lock / unlock';$lock.Width=140;$lock.Margin='10,0,0,0';[void]$buttons.Children.Add($lock)
 $status=FrameLabel '' 12 '#B8CADB';$status.TextAlignment='Left';$status.Margin='0,18,0,0';[void]$panel.Children.Add($status)
 $move.Add_Click({try{$id=$list.SelectedItem.id;$m=$screens.SelectedItem;if($id -match '^meters[2-8]?$' -and -not (DesktopDetached $id)){if([IO.File]::Exists($StatePath)){$state=ReadFrameStateFile $StatePath;$meter=$state.frameDisplay.meter;if($meter){DesktopMeterSettings ([string]$meter.scope) ([int]$meter.width) ([int]$meter.height)}}};DesktopSave $id ([pscustomobject]@{detached=$true;locked=$false;monitor=$m.id;x=$m.x+40;y=$m.y+40;workX=$m.x;workY=$m.y;width=0;height=0});$status.Text='Drag its title to position it. Lock when finished.'}catch{$status.Text=$_.Exception.Message}})
 $reset.Add_Click({try{$id=$list.SelectedItem.id;DesktopLoad;if($script:desktopPlacements.ContainsKey($id)){$p=$script:desktopPlacements[$id];$p.detached=$false;DesktopSave $id $p};$status.Text='Returned to the original in-game position.'}catch{$status.Text=$_.Exception.Message}})
 $lock.Add_Click({try{$id=$list.SelectedItem.id;DesktopLoad;if($script:desktopPlacements.ContainsKey($id)){$p=$script:desktopPlacements[$id];$p.locked=-not $p.locked;DesktopSave $id $p;$status.Text=$(if($p.locked){'Locked; mouse clicks pass through.'}else{'Unlocked; drag its title to move.'})}}catch{$status.Text=$_.Exception.Message}})
 $list.Add_SelectionChanged({DesktopLoad;$id=$list.SelectedItem.id;$p=$script:desktopPlacements[$id];$status.Text=if($p -and $p.detached){'Desktop window'+$(if($p.locked){' — locked'}else{' — unlocked'})}else{'Inside WoW'};if($p){for($i=0;$i -lt $screens.Items.Count;$i++){if($screens.Items[$i].id -eq $p.monitor){$screens.SelectedIndex=$i}}}})
 if($RenderOnly){return $w}
 [void]$w.ShowDialog()
}
