param(
    [switch]$Update,
    [string]$InstallRoot,
    [string]$WowPath,
    [ValidateSet('retail','forever')][string]$Client='retail',
    [string]$DesktopPath,
    [string]$RenderTest,
    [string]$CleanupFolder,
    [string]$SmokeTest
)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[Windows.Forms.Application]::EnableVisualStyles()
. (Join-Path $PSScriptRoot 'Core.ps1')

function Find-LamdaGames {
    $roots=New-Object Collections.Generic.List[string]
    foreach($drive in [IO.DriveInfo]::GetDrives()) {
        if($drive.IsReady -and $drive.DriveType -eq [IO.DriveType]::Fixed) {
            foreach($suffix in @('World of Warcraft','Games\World of Warcraft','Program Files (x86)\World of Warcraft','Program Files\World of Warcraft')){$roots.Add((Join-Path $drive.RootDirectory.FullName $suffix))}
        }
    }
    foreach($key in @('HKLM:\SOFTWARE\WOW6432Node\Blizzard Entertainment\World of Warcraft','HKLM:\SOFTWARE\Blizzard Entertainment\World of Warcraft')) {
        try {$path=(Get-ItemProperty -LiteralPath $key -Name InstallPath -ErrorAction Stop).InstallPath;if($path){$roots.Add($path);$roots.Add((Split-Path -Parent $path))}}catch{}
    }
    try {
        foreach($process in Get-Process -Name Wow,WowB -ErrorAction SilentlyContinue) {
            if($process.Path){$roots.Insert(0,(Split-Path -Parent (Split-Path -Parent $process.Path)))}
        }
    } catch {}
    $found=@{}
    foreach($root in $roots) {
        foreach($key in @('retail','forever')) {
            foreach($candidate in @(Get-LamdaGameCandidates $root $key)) {
                if([IO.File]::Exists((Join-Path $candidate (Get-LamdaGameName $key $candidate))) -and -not $found.ContainsKey($key)){$found[$key]=$candidate}
            }
        }
    }
    return $found
}
function Show-LamdaSetup {
    $isUpdate=[bool]$Update;$readOnlyTest=[bool]$SmokeTest;$smokeReceipt=$SmokeTest
    if(-not [Environment]::Is64BitOperatingSystem){throw 'LamdaUI needs 64-bit Windows.'}
    if(-not $InstallRoot) {
        $InstallRoot=Join-Path $env:LOCALAPPDATA 'LamdaUI'
        if(Test-Path -LiteralPath 'D:\LamdaUI\Updater\updater.json') {
            try {if((Read-LamdaJson 'D:\LamdaUI\Updater\updater.json').product -eq 'LamdaUI'){$InstallRoot='D:\LamdaUI'}}catch{}
        }
    }
    $InstallRoot=[IO.Path]::GetFullPath($InstallRoot)
    if(-not $DesktopPath){$DesktopPath=[Environment]::GetFolderPath('DesktopDirectory')}
    $games=if($RenderTest){@{retail='C:\Games\World of Warcraft\_retail_'}}else{Find-LamdaGames}
    if($WowPath){$games[$Client]=$WowPath}
    if($Update) {
        $config=Read-LamdaJson (Join-Path $InstallRoot 'Updater\updater.json')
        if($config.product -ne 'LamdaUI' -or $config.schema -ne 2){throw 'This installation needs the new LamdaUI setup. Download Install-LamdaUI.cmd from the release page.'}
        $games=@{};foreach($property in $config.clients.PSObject.Properties){$games[$property.Name]=$property.Value}
    }
    $form=New-Object Windows.Forms.Form
    $form.Text=if($Update){'Update LamdaUI'}else{'Install LamdaUI'}
    $form.ClientSize=New-Object Drawing.Size(540,340)
    $form.StartPosition='CenterScreen';$form.FormBorderStyle='FixedDialog';$form.MaximizeBox=$false
    $form.BackColor=[Drawing.Color]::FromArgb(17,25,35);$form.ForeColor=[Drawing.Color]::FromArgb(234,241,248)
    $form.Font=New-Object Drawing.Font('Segoe UI',10)
    function Add-Label([string]$Text,[int]$Y,[int]$Height=28) {
        $label=New-Object Windows.Forms.Label;$label.Text=$Text;$label.Location=New-Object Drawing.Point(26,$Y)
        $label.Size=New-Object Drawing.Size(488,$Height);$form.Controls.Add($label);return $label
    }
    $heading=Add-Label 'LamdaUI' 22 42;$heading.Font=New-Object Drawing.Font('Segoe UI',24,[Drawing.FontStyle]::Bold)
    $subtitle=Add-Label 'Ready for WoW in a few clicks.' 69
    $gameLabel=Add-Label 'Your game' 112
    $choice=New-Object Windows.Forms.ComboBox;$choice.DropDownStyle='DropDownList'
    $choice.Items.AddRange(@('World of Warcraft','WoW Forever'));$choice.SelectedIndex=if($Client -eq 'forever'){1}else{0}
    $choice.Location=New-Object Drawing.Point(26,140);$choice.Size=New-Object Drawing.Size(326,30);$form.Controls.Add($choice)
    $browse=New-Object Windows.Forms.Button;$browse.Text='Find WoW...';$browse.Location=New-Object Drawing.Point(364,139);$browse.Size=New-Object Drawing.Size(150,32)
    $browse.BackColor=[Drawing.Color]::FromArgb(45,65,83);$browse.ForeColor=[Drawing.Color]::White;$browse.FlatStyle='Flat';$form.Controls.Add($browse)
    $location=Add-Label '' 179 41;$location.ForeColor=[Drawing.Color]::FromArgb(157,179,198)
    $status=Add-Label 'We will install the addon and its companion app.' 228 44
    $progress=New-Object Windows.Forms.ProgressBar;$progress.Location=New-Object Drawing.Point(26,275);$progress.Size=New-Object Drawing.Size(312,8);$progress.Style='Marquee';$progress.Visible=$false;$form.Controls.Add($progress)
    $action=New-Object Windows.Forms.Button;$action.Text=if($Update){'Update'}else{'Install'}
    $action.BackColor=[Drawing.Color]::FromArgb(90,205,227);$action.ForeColor=[Drawing.Color]::Black;$action.FlatStyle='Flat'
    $action.Location=New-Object Drawing.Point(364,285);$action.Size=New-Object Drawing.Size(150,36);$form.Controls.Add($action)
    $form.AcceptButton=$action
    $state=@{job=$null;folder=$null;done=$false;result=$null;games=$games;root=$InstallRoot;desktop=$DesktopPath}
    $refresh={
        $key=if($choice.SelectedIndex -eq 1){'forever'}else{'retail'}
        if($state.games.ContainsKey($key)){$location.Text='WoW found: '+$state.games[$key];$action.Enabled=$true}
        else{$location.Text='Choose your World of Warcraft folder to continue.';$action.Enabled=$false}
    }.GetNewClosure()
    $choice.Add_SelectedIndexChanged($refresh)
    $browse.Add_Click({
        $dialog=New-Object Windows.Forms.FolderBrowserDialog;$dialog.Description='Choose your World of Warcraft folder';$dialog.ShowNewFolderButton=$false
        if($dialog.ShowDialog($form) -eq 'OK') {
            $key=if($choice.SelectedIndex -eq 1){'forever'}else{'retail'}
            $path=$dialog.SelectedPath
            $foundPaths=@(Get-LamdaGameCandidates $path $key | Where-Object {[IO.File]::Exists((Join-Path $_ (Get-LamdaGameName $key $_)))})
            if($foundPaths.Count){$state.games[$key]=$foundPaths[0];& $refresh}
            else{[Windows.Forms.MessageBox]::Show($form,'That folder does not contain the selected World of Warcraft game. Please choose its folder.','Find WoW') | Out-Null}
        }
        $dialog.Dispose()
    }.GetNewClosure())
    $timer=New-Object Windows.Forms.Timer;$timer.Interval=300
    $coreSource=Join-Path $PSScriptRoot 'Core.ps1'
    $installerSource=$PSCommandPath
    $start={
        if($state.done){Start-Process (Join-Path $state.root 'Desktop\LamdaUI.exe');$form.Close();return}
        $selection=@{}
        if($isUpdate){foreach($key in $state.games.Keys){$selection[$key]=$state.games[$key]}}
        else {$key=if($choice.SelectedIndex -eq 1){'forever'}else{'retail'};$selection[$key]=$state.games[$key]}
        $writable=$true
        foreach($targetFolder in @($state.root)+@($selection.Values | ForEach-Object {Join-Path $_ 'Interface\AddOns'})) {
            $probeFolder=$targetFolder
            while(-not [IO.Directory]::Exists($probeFolder)){$probeFolder=[IO.Path]::GetDirectoryName($probeFolder)}
            $probe=Join-Path $probeFolder ('.lamda-write-test-'+[guid]::NewGuid().ToString('N'))
            try {[IO.File]::WriteAllBytes($probe,[byte[]]@())}catch [UnauthorizedAccessException]{$writable=$false}
            finally {if([IO.File]::Exists($probe)){[IO.File]::Delete($probe)}}
        }
        if(-not $writable) {
            $answer=[Windows.Forms.MessageBox]::Show($form,'Windows needs your permission to install into this WoW folder. Continue with the Windows permission prompt?','Install LamdaUI',[Windows.Forms.MessageBoxButtons]::YesNo,[Windows.Forms.MessageBoxIcon]::Information)
            if($answer -eq 'Yes') {
                $elevatedFolder=Join-Path ([IO.Path]::GetTempPath()) ('LamdaUI-Elevated-'+[guid]::NewGuid().ToString('N'))
                [IO.Directory]::CreateDirectory($elevatedFolder) | Out-Null
                Copy-Item -LiteralPath $installerSource -Destination (Join-Path $elevatedFolder 'Install.ps1')
                Copy-Item -LiteralPath $coreSource -Destination (Join-Path $elevatedFolder 'Core.ps1')
                $arguments='-NoProfile -STA -WindowStyle Hidden -ExecutionPolicy Bypass -File "'+(Join-Path $elevatedFolder 'Install.ps1')+'" -InstallRoot "'+$state.root+'" -DesktopPath "'+$state.desktop+'" -CleanupFolder "'+$elevatedFolder+'"'
                if($isUpdate){$arguments+=' -Update'}else{$arguments+=' -Client '+$key+' -WowPath "'+$selection[$key]+'"'}
                try {Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -Verb RunAs -ArgumentList $arguments | Out-Null;$form.Close()}catch{Remove-Item -LiteralPath $elevatedFolder -Recurse -Force;[Windows.Forms.MessageBox]::Show($form,'Windows did not grant permission. You can try again when ready.','LamdaUI setup') | Out-Null}
            }
            return
        }
        $state.folder=Join-Path ([IO.Path]::GetTempPath()) ('LamdaUI-'+[guid]::NewGuid().ToString('N'))
        [IO.Directory]::CreateDirectory($state.folder) | Out-Null
        $statusFile=Join-Path $state.folder 'status.json'
        $state.job=Start-Job -ArgumentList $coreSource,$state.folder,$state.root,$selection,$statusFile,$state.desktop,$readOnlyTest -ScriptBlock {
            param($Core,$Stage,$Root,$Games,$StatusFile,$Desktop,$ReadOnly)
            . $Core
            try {
                if(Test-Path -LiteralPath (Join-Path $Root 'Updater\updater.json')) {
                    $installed=Read-LamdaJson (Join-Path $Root 'Updater\updater.json')
                    if($installed.product -eq 'LamdaUI' -and $installed.schema -eq 2){$script:LamdaStatusMirror=Join-Path $Root 'Updater\data\status.json'}
                }
                $archive=Get-LamdaRelease $Stage $StatusFile
                if($ReadOnly){return @{downloaded=$true}}
                $release=Join-Path $Stage 'release';Expand-LamdaRelease $archive $release
                Set-LamdaStatus $StatusFile 'verifying' 'Checking your download...'
                $result=Install-LamdaRelease $release $Root $Games $StatusFile $Desktop
                $script:LamdaStatusMirror=Join-Path $Root 'Updater\data\status.json'
                Complete-LamdaSetup $result
                Set-LamdaStatus $StatusFile 'complete' 'LamdaUI is ready. Use /reload in WoW if you kept it open.'
                return $result
            } catch {Set-LamdaStatus $StatusFile 'failed' $_.Exception.Message;throw}
        }
        $action.Enabled=$false;$browse.Enabled=$false;$choice.Enabled=$false;$progress.Visible=$true
        $status.Text='Checking for the latest LamdaUI release...';$timer.Start()
    }.GetNewClosure()
    $action.Add_Click($start)
    $timer.Add_Tick({
        if($null -eq $state.job){return}
        $file=Join-Path $state.folder 'status.json'
        try {$current=[IO.File]::ReadAllText($file)|ConvertFrom-Json;$status.Text=$current.message}catch{}
        if($state.job.State -in @('Completed','Failed','Stopped')) {
            $timer.Stop();$progress.Visible=$false
            try {
                $result=Receive-Job $state.job -ErrorAction Stop
                if($state.job.State -ne 'Completed'){throw 'Setup stopped. Please try again.'}
                $state.done=$true;$state.result=$result;$status.Text='LamdaUI is ready. Open WoW normally.'
                if($isUpdate){$status.Text='Update complete. Use /reload in WoW when ready.'}
                $action.Text='Open LamdaUI';$action.Enabled=$true
            } catch {
                $status.Text=$_.Exception.Message
                $action.Text='Try again';$action.Enabled=$true;$browse.Enabled=-not $isUpdate;$choice.Enabled=-not $isUpdate
            }
            Remove-Job $state.job -Force;$state.job=$null
            if(Test-Path -LiteralPath $state.folder){Remove-Item -LiteralPath $state.folder -Recurse -Force}
            if($readOnlyTest){[IO.File]::WriteAllText($smokeReceipt,(@{workerFinished=$true;message=$status.Text;button=$action.Text;downloadOnly=$true}|ConvertTo-Json));$form.Close()}
        }
    }.GetNewClosure())
    $form.Add_FormClosing({param($sender,$event) if($null -ne $state.job){$event.Cancel=$true}}.GetNewClosure())
    & $refresh
    if($Update){$choice.Enabled=$false;$browse.Enabled=$false;$subtitle.Text='Keep playing. Reload WoW when you are ready.';$location.Text=(@($games.Keys | ForEach-Object {(Get-Culture).TextInfo.ToTitleCase($_)}) -join ' + ')}
    if($RenderTest) {
        $form.StartPosition='Manual';$form.Location=New-Object Drawing.Point(-10000,-10000);$form.ShowInTaskbar=$false
        $form.Show();[Windows.Forms.Application]::DoEvents()
        $bitmap=New-Object Drawing.Bitmap($form.Width,$form.Height)
        try {$form.DrawToBitmap($bitmap,(New-Object Drawing.Rectangle(0,0,$form.Width,$form.Height)));$bitmap.Save($RenderTest)}finally{$bitmap.Dispose();$form.Close()}
    } else {
        if($SmokeTest){$form.StartPosition='Manual';$form.Location=New-Object Drawing.Point(-10000,-10000);$form.ShowInTaskbar=$false;$form.Add_Shown($start)}
        elseif($Update){$form.Add_Shown($start)}
        [void]$form.ShowDialog()
    }
    $timer.Dispose();$form.Dispose()
}
try {Show-LamdaSetup} catch {if($RenderTest -or $SmokeTest){throw};[Windows.Forms.MessageBox]::Show($_.Exception.Message,'LamdaUI setup',[Windows.Forms.MessageBoxButtons]::OK,[Windows.Forms.MessageBoxIcon]::Information) | Out-Null;exit 1}
finally {
    if($CleanupFolder -and [IO.Path]::GetDirectoryName($CleanupFolder).TrimEnd('\') -eq [IO.Path]::GetTempPath().TrimEnd('\') -and [IO.Path]::GetFileName($CleanupFolder) -match '^LamdaUI-Elevated-[0-9a-f]{32}$') {
        Remove-Item -LiteralPath $CleanupFolder -Recurse -Force -ErrorAction SilentlyContinue
    }
}
