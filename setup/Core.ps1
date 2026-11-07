# Shared Windows setup/update operations. No development tools are required.
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$script:LamdaRepository='tm-LBenson/lamda-engine'
$script:LamdaAddons=@('lamdaUI','LamdaFrames','LamdaMover')
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -AssemblyName System.IO.Compression
$script:LamdaStatusMirror=$null

function Write-LamdaAtomic([string]$Path,[byte[]]$Bytes) {
    $parent=Split-Path -Parent $Path
    [IO.Directory]::CreateDirectory($parent) | Out-Null
    $temp=Join-Path $parent ('.lamda-setup-'+[guid]::NewGuid().ToString('N'))
    try {
        [IO.File]::WriteAllBytes($temp,$Bytes)
        for($attempt=0;;$attempt++) {
            try {
                if([IO.File]::Exists($Path)){[IO.File]::Replace($temp,$Path,[NullString]::Value)}else{[IO.File]::Move($temp,$Path)}
                break
            } catch [IO.IOException] { if($attempt -ge 5){throw};Start-Sleep -Milliseconds (50*[math]::Pow(2,$attempt)) }
        }
    } finally { if([IO.File]::Exists($temp)){[IO.File]::Delete($temp)} }
}
function Write-LamdaJson([string]$Path,$Value) {
    Write-LamdaAtomic $Path ([Text.Encoding]::UTF8.GetBytes(($Value | ConvertTo-Json -Depth 30)+"`n"))
}
function Read-LamdaJson([string]$Path) { return ([IO.File]::ReadAllText($Path) | ConvertFrom-Json) }
function Get-LamdaHash([string]$Path) {
    if(-not [IO.File]::Exists($Path)){return $null}
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}
function Get-LamdaMember($Value,[string]$Name,$Default=$null) {
    if($null -eq $Value){return $Default}
    if($Value -is [Collections.IDictionary]) {if($Value.Contains($Name)){return $Value[$Name]};return $Default}
    $property=$Value.PSObject.Properties[$Name]
    if($null -eq $property){return $Default};return $property.Value
}
function Set-LamdaStatus([string]$File,[string]$State,[string]$Message) {
    $value=@{status=$State;message=$Message;updated_at=[DateTime]::UtcNow.ToString('o')}
    if($File){Write-LamdaJson $File $value}
    if($script:LamdaStatusMirror -and $script:LamdaStatusMirror -ne $File){Write-LamdaJson $script:LamdaStatusMirror $value}
}
function Assert-LamdaPath([string]$Name) {
    if(-not $Name -or $Name.StartsWith('/') -or $Name -match '[\\:<>"|?*\x00-\x1f]' -or $Name.EndsWith('/')){throw 'The download contains an invalid file name.'}
    foreach($part in $Name.Split('/')) {
        if($part -in @('','.','..') -or $part -match '[ .]$' -or $part -match '^(?i:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)'){throw 'The download contains an unsafe file name.'}
    }
}
function Assert-LamdaOrdinaryPath([string]$Path) {
    $current=[IO.Path]::GetFullPath($Path)
    while($current) {
        if(Test-Path -LiteralPath $current) {
            $item=Get-Item -Force -LiteralPath $current
            # OneDrive placeholders are also reparse points, but are not path redirects.
            if(($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -and (Get-LamdaMember $item 'LinkType' '') -in @('Junction','SymbolicLink')){throw 'Choose a normal folder, rather than a linked folder, for this installation.'}
        }
        $parent=[IO.Path]::GetDirectoryName($current)
        if($parent -eq $current){break};$current=$parent
    }
}
function Get-LamdaVersion([string]$Value,[string]$Client='') {
    $pattern=if($Client){'^v?(\d+)\.(\d+)\.(\d+)-'+$Client+'\.(\d+)$'}else{'^(\d+)\.(\d+)\.(\d+)$'}
    if($Value -notmatch $pattern){throw 'This download is for a different game version.'}
    $revision=if($Client){[int]$Matches[4]}else{0}
    return [Version]::new([int]$Matches[1],[int]$Matches[2],[int]$Matches[3],$revision)
}
function Assert-LamdaURL([string]$Url) {
    $uri=New-Object Uri $Url
    if($uri.Scheme -ne 'https' -or $uri.Port -ne 443 -or $uri.UserInfo -or $uri.Host -notin @('api.github.com','github.com','release-assets.githubusercontent.com','objects.githubusercontent.com')) {
        throw 'The update download did not come from the expected GitHub location.'
    }
}
function Receive-LamdaURL([string]$Url,[string]$File,[long]$Limit) {
    [Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12
    for($redirect=0;$redirect -le 5;$redirect++) {
        Assert-LamdaURL $Url
        $request=[Net.HttpWebRequest]::Create($Url)
        $request.UserAgent='LamdaUI-Setup';$request.Accept='application/vnd.github+json'
        $request.Headers.Add('X-GitHub-Api-Version','2026-03-10')
        $request.AllowAutoRedirect=$false;$request.Timeout=30000;$request.ReadWriteTimeout=30000
        try {$response=$request.GetResponse()} catch {throw 'Could not download LamdaUI from GitHub. Check your internet connection and try again.'}
        try {
            if([int]$response.StatusCode -in @(301,302,303,307,308)) {
                $Url=(New-Object Uri ([Uri]$Url),$response.Headers['Location']).AbsoluteUri
                continue
            }
            if($response.ContentLength -gt $Limit){throw 'The download is larger than expected.'}
            $inputStream=$response.GetResponseStream();$output=[IO.File]::Open($File,[IO.FileMode]::CreateNew)
            try {
                $buffer=New-Object byte[] 65536;$total=0L
                while(($read=$inputStream.Read($buffer,0,$buffer.Length)) -gt 0) {
                    $total+=$read;if($total -gt $Limit){throw 'The download is larger than expected.'}
                    $output.Write($buffer,0,$read)
                }
            } finally {$output.Dispose();$inputStream.Dispose()}
            return
        } finally {$response.Dispose()}
    }
    throw 'GitHub redirected the download too many times. Please try again later.'
}
function Get-LamdaRelease([string]$Stage,[string]$StatusFile) {
    Set-LamdaStatus $StatusFile 'checking' 'Checking for the latest LamdaUI release...'
    $listing=Join-Path $Stage 'releases.json'
    Receive-LamdaURL ('https://api.github.com/repos/'+$script:LamdaRepository+'/releases?per_page=100') $listing 4194304
    $candidates=@(Read-LamdaJson $listing | Where-Object {
        -not $_.draft -and -not $_.prerelease -and $_.published_at -and @($_.assets | Where-Object name -eq 'lamdaui-update.zip').Count -eq 1
    } | Sort-Object published_at -Descending)
    if(-not $candidates.Count){throw 'LamdaUI 1.0 is not available to download yet. Please check again later.'}
    $release=$candidates[0];$asset=@($release.assets | Where-Object name -eq 'lamdaui-update.zip')[0]
    if($asset.state -ne 'uploaded' -or $asset.size -le 0 -or $asset.size -gt 536870912 -or (Get-LamdaMember $asset 'digest' '') -notmatch '^sha256:([0-9a-f]{64})$'){throw 'The release download is not ready yet. Please try again later.'}
    $expected=$Matches[1]
    $prefix='https://github.com/'+$script:LamdaRepository+'/releases/download/'
    if(-not $asset.browser_download_url.StartsWith($prefix,[StringComparison]::Ordinal)){throw 'The download belongs to a different repository.'}
    Set-LamdaStatus $StatusFile 'downloading' 'Downloading LamdaUI...'
    $archive=Join-Path $Stage 'lamdaui-update.zip'
    Receive-LamdaURL $asset.browser_download_url $archive $asset.size
    if((Get-Item -LiteralPath $archive).Length -ne $asset.size -or (Get-LamdaHash $archive) -ne $expected){throw 'The download was incomplete or damaged. Please try again.'}
    return $archive
}
function Expand-LamdaRelease([string]$Archive,[string]$Destination) {
    $zip=[IO.Compression.ZipFile]::OpenRead($Archive)
    try {
        if($zip.Entries.Count -gt 30000){throw 'The update contains too many files.'}
        $names=@{};$total=0L
        foreach($entry in $zip.Entries) {
            Assert-LamdaPath $entry.FullName
            $mode=($entry.ExternalAttributes -shr 16) -band 61440
            if($mode -notin @(0,32768)){throw 'The update contains a linked or unexpected file.'}
            if($names.ContainsKey($entry.FullName)){throw 'The update contains duplicate files.'}
            $names[$entry.FullName]=$true;$total+=$entry.Length
            if($total -gt 1073741824){throw 'The expanded update is too large.'}
        }
        foreach($entry in $zip.Entries) {
            $path=Join-Path $Destination $entry.FullName
            [IO.Directory]::CreateDirectory((Split-Path -Parent $path)) | Out-Null
            $inputStream=$entry.Open();$output=[IO.File]::Open($path,[IO.FileMode]::CreateNew)
            try {$inputStream.CopyTo($output)}finally{$inputStream.Dispose();$output.Dispose()}
        }
    } finally {$zip.Dispose()}
}
function Test-LamdaRelease([string]$Release) {
    $manifest=Read-LamdaJson (Join-Path $Release 'release.json')
    if($manifest.product -ne 'LamdaUI' -or $manifest.schema -ne 1 -or $manifest.id -notmatch '^\d{8}-\d{6}-\d{6}$'){throw 'This is not a LamdaUI update package.'}
    $expected=@{};$actual=@(Get-ChildItem -LiteralPath $Release -Recurse -File)
    foreach($property in $manifest.files.PSObject.Properties) {
        Assert-LamdaPath $property.Name
        if($expected.ContainsKey($property.Name) -or $property.Value -notmatch '^[0-9a-f]{64}$'){throw 'The package manifest contains duplicate or invalid files.'}
        if((Get-LamdaHash (Join-Path $Release $property.Name)) -ne $property.Value){throw 'A downloaded file failed its integrity check.'}
        $expected[$property.Name]=$property.Value
    }
    if($actual.Count -ne $expected.Count+1){throw 'Unexpected files were found in the download.'}
    $allowed=@{};$bundles=@{}
    foreach($client in @('retail','forever')) {
        $bundles[$client]=@{}
        foreach($kind in @('addon','engine')) {
            $prefix=$client+'/'+$kind+'/';$bundle=Read-LamdaJson (Join-Path $Release ($prefix+'manifest.json'))
            $null=Get-LamdaVersion $bundle.version $client
            if($kind -eq 'addon') {
                $interface=if($client -eq 'retail'){120100}else{16001}
                if($bundle.interface -ne $interface -or @($bundle.addons).Count -lt 1 -or 'lamdaUI' -notin @($bundle.addons) -or @($bundle.addons | Where-Object {$_ -notin $script:LamdaAddons}).Count){throw 'The addon package is for a different game.'}
            } elseif($bundle.product -ne 'LamdaUI' -or $bundle.client -ne $client){throw 'The companion package is for a different game.'}
            $allowed[$prefix+'manifest.json']=$true
            foreach($property in $bundle.files.PSObject.Properties) {
                Assert-LamdaPath $property.Name
                $top=$property.Name.Split('/')[0]
                if(($kind -eq 'addon' -and $top -notin $script:LamdaAddons) -or ($kind -eq 'engine' -and $top -in @('data','manifest.json'))){throw 'The update contains a file outside LamdaUI.'}
                $name=$prefix+'package/'+$property.Name
                if(-not $expected.ContainsKey($name) -or $expected[$name] -ne $property.Value){throw 'The package and release manifests do not agree.'}
                $allowed[$name]=$true
            }
            if($kind -eq 'addon') {
                foreach($addon in @($bundle.addons)) {
                    $toc=[IO.File]::ReadAllText((Join-Path $Release ($prefix+'package/'+$addon+'/'+$addon+'.toc')))
                    if(-not $toc.Contains('## X-LamdaFrames-Managed: 1')){throw 'An addon package is not managed by LamdaUI.'}
                }
            } else {
                foreach($name in @('lamda-engine.exe','Start.ps1','Stop.ps1')) {
                    if(-not $allowed.ContainsKey($prefix+'package/'+$name)){throw 'A companion package is incomplete.'}
                }
            }
            $bundles[$client][$kind]=$bundle
        }
    }
    foreach($kind in @('desktop','updater')) {
        $bundle=Read-LamdaJson (Join-Path $Release ($kind+'/manifest.json'))
        if($bundle.product -ne 'LamdaUI' -or $bundle.component -ne $kind){throw 'The setup package is incomplete.'}
        $null=Get-LamdaVersion $bundle.version
        $names=if($kind -eq 'desktop'){@('LamdaUI.exe','LamdaUI.exe.config','lui.ico')}else{@('Install.ps1','Core.ps1','Update.ps1')}
        if(@($bundle.files.PSObject.Properties).Count -ne $names.Count){throw 'The setup package contains unexpected files.'}
        $allowed[$kind+'/manifest.json']=$true
        foreach($name in $names) {
            $key=$kind+'/package/'+$name
            if(-not $expected.ContainsKey($key) -or $expected[$key] -ne (Get-LamdaMember $bundle.files $name)){throw 'The setup files failed their integrity check.'}
            $allowed[$key]=$true
        }
        $bundles[$kind]=$bundle
    }
    if($allowed.Count -ne $expected.Count){throw 'The release contains unexpected files.'}
    return $bundles
}
function Get-LamdaModulePlan([string]$Release,[string]$Client,[string]$ComponentDir,[string]$Addons,$Bundle,[string]$PlanRoot) {
    $effective=@{}
    foreach($property in $Bundle.files.PSObject.Properties){$effective[$property.Name]=@{hash=$property.Value;source=(Join-Path $Release ($Client+'/addon/package/'+$property.Name))}}
    if((Get-LamdaMember $Bundle 'moduleCatalog' 0) -ne 1){return $effective}
    $moduleRoot=Join-Path $Release ($Client+'/engine/package/modules')
    $catalog=Read-LamdaJson (Join-Path $moduleRoot 'catalog.json')
    if($catalog.product -ne 'LamdaUI' -or $catalog.schema -ne 1 -or $catalog.client -ne $Client){throw 'The addon catalog is for a different game.'}
    $available=@{};foreach($summary in $catalog.modules){$available[$summary.id]=Read-LamdaJson (Join-Path $moduleRoot ($summary.id+'/manifest.json'))}
    $records=@{};$inventoryPath=Join-Path $ComponentDir 'data\addons\installed.json'
    if([IO.File]::Exists($inventoryPath)) {
        $inventory=Read-LamdaJson $inventoryPath
        if($inventory.schema -ne 1){throw 'The installed addon list is invalid.'}
        foreach($property in $inventory.installed.PSObject.Properties) {
            if(-not $available.ContainsKey($property.Name)){throw 'An installed addon is missing from this catalog.'}
            $records[$property.Name]=$property.Value
            foreach($name in $property.Value.files) {
                Assert-LamdaPath $name
                $target=Join-Path $Addons $name;$hash=Get-LamdaHash $target
                if($hash){$effective[$name]=@{hash=$hash;source=$null}}
            }
        }
    } else {
        $tocPath=Join-Path $Addons 'lamdaUI\lamdaUI.toc';$selected=@()
        if([IO.File]::Exists($tocPath)) {
            $toc=[IO.File]::ReadAllText($tocPath)
            if($toc -match '(?m)^## X-Lamda-Modules:\s*(.*)$'){$selected=@($Matches[1].Split(',') | ForEach-Object {$_.Trim()} | Where-Object {$_})}
            else {
                foreach($summary in $catalog.modules) {
                    $present=$true;foreach($property in $available[$summary.id].files.PSObject.Properties){if(-not [IO.File]::Exists((Join-Path $Addons $property.Name))){$present=$false;break}}
                    if($present){$selected+=$summary.id}
                }
            }
        }
        foreach($id in $selected) {
            if(-not $available.ContainsKey($id)){throw 'The installed addon list is not in this catalog.'}
            $module=$available[$id];$metadata=@();$lines=Get-LamdaMember $catalog.metadata $id @()
            foreach($line in @($lines)){$metadata+=$line.Replace('{client}',$Client)}
            $current=$true
            foreach($property in $module.files.PSObject.Properties){if((Get-LamdaHash (Join-Path $Addons $property.Name)) -ne $property.Value){$current=$false}}
            $records[$id]=@{version=if($current){$module.version}else{'legacy'};files=@($module.files.PSObject.Properties.Name);entries=@($module.entries);metadata=$metadata}
            foreach($property in $module.files.PSObject.Properties) {
                $hash=Get-LamdaHash (Join-Path $Addons $property.Name)
                if($hash){$effective[$property.Name]=@{hash=$hash;source=$null}}
            }
        }
    }
    $lines=New-Object Collections.Generic.List[string]
    foreach($line in $catalog.headers){$lines.Add([string]$line)}
    $lines.Add('## X-Lamda-Modules: '+(($records.Keys | Sort-Object) -join ','))
    foreach($summary in $catalog.modules){if($records.ContainsKey($summary.id)){foreach($line in @((Get-LamdaMember $records[$summary.id] 'metadata' @()))){$lines.Add([string]$line)}}}
    $lines.Add('');$entrySets=@{};$emitted=@{}
    foreach($id in $records.Keys) {
        $entrySets[$id]=@{};$emitted[$id]=@{}
        foreach($entry in $records[$id].entries){$entrySets[$id][$entry]=$true}
    }
    foreach($item in $catalog.order) {
        if($item.module -eq 'core'){$lines.Add($item.path.Replace('/','\'))}
        elseif($records.ContainsKey($item.module) -and $entrySets[$item.module].ContainsKey($item.path)) {
            $lines.Add($item.path.Replace('/','\'));$emitted[$item.module][$item.path]=$true
        }
    }
    foreach($summary in $catalog.modules) {
        if($records.ContainsKey($summary.id)) {
            foreach($entry in $records[$summary.id].entries){if(-not $emitted[$summary.id].ContainsKey($entry)){$lines.Add($entry.Replace('/','\'))}}
        }
    }
    $tocSource=Join-Path $PlanRoot ($Client+'-lamdaUI.toc')
    Write-LamdaAtomic $tocSource ([Text.Encoding]::UTF8.GetBytes(($lines -join "`n")+"`n"))
    $effective['lamdaUI/lamdaUI.toc']=@{hash=(Get-LamdaHash $tocSource);source=$tocSource}
    return $effective
}
function Get-LamdaGameName([string]$Client,[string]$Game) {
    if($Client -eq 'retail'){return 'Wow.exe'}
    if($Client -eq 'forever') {
        if($Game -and (Split-Path -Leaf $Game.TrimEnd('\','/')) -ieq '_forever_'){return 'Wow.exe'}
        return 'WowB.exe'
    }
    throw 'Unknown game version.'
}
function Get-LamdaGameCandidates([string]$Root,[string]$Client) {
    $folders=if($Client -eq 'retail'){@('_retail_')}elseif($Client -eq 'forever'){@('_forever_','_classic_beta_')}else{throw 'Unknown game version.'}
    foreach($folder in $folders){[IO.Path]::Combine($Root,$folder)}
    $Root
}
function Test-LamdaGameOpen([string]$Client,[string]$Game) {
    $name=Get-LamdaGameName $Client $Game;$exe=Join-Path $Game $name
    return @(Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($name)) -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq $exe}).Count -gt 0
}
function Get-LamdaProcess([string]$Exe) {
    $name=[IO.Path]::GetFileName($Exe)
    return @(Get-CimInstance Win32_Process -Filter ("Name='"+$name+"'") | Where-Object {$_.ExecutablePath -eq $Exe})
}
function Get-LamdaLaunch([string]$Exe) {
    $processes=@(Get-LamdaProcess $Exe)
    if($processes.Count -gt 1){throw 'More than one companion is running for this game. Close the extra companion and try again.'}
    if(-not $processes.Count){return $null}
    $process=$processes[0];$prefix='"'+$Exe+'" '
    if($process.CommandLine.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)){$arguments=$process.CommandLine.Substring($prefix.Length)}
    elseif($process.CommandLine.StartsWith($Exe+' ',[StringComparison]::OrdinalIgnoreCase)){$arguments=$process.CommandLine.Substring($Exe.Length+1)}
    else{throw 'Could not preserve the companion settings. Try restarting LamdaUI first.'}
    return @{pid=$process.ProcessId;arguments=$arguments}
}
function Stop-LamdaCompanion([string]$Exe) {
    $launch=Get-LamdaLaunch $Exe
    if($null -eq $launch){return $null}
    Stop-Process -Id $launch.pid
    Wait-Process -Id $launch.pid -Timeout 15 -ErrorAction SilentlyContinue
    if(@(Get-LamdaProcess $Exe).Count){throw 'The companion has not finished closing.'}
    return $launch.arguments
}
function Start-LamdaCompanion([string]$Exe,[string]$Arguments,[string]$Client,[string]$Version) {
    $process=Start-Process -FilePath $Exe -ArgumentList $Arguments -WorkingDirectory (Split-Path -Parent $Exe) -WindowStyle Hidden -PassThru
    $state=Join-Path (Split-Path -Parent $Exe) 'data\state.json'
    for($i=0;$i -lt 80;$i++) {
        if($process.HasExited){throw 'The companion could not start. Your previous installation will be restored.'}
        try {$value=Read-LamdaJson $state;if($value.pid -eq $process.Id -and $value.client -eq $Client -and (-not $Version -or $value.version -eq $Version)){return}}catch{}
        Start-Sleep -Milliseconds 250
    }
    throw 'The companion did not finish starting. Your previous installation will be restored.'
}
function Stop-LamdaPanel([string]$Root) {
    $exe=Join-Path $Root 'Desktop\LamdaUI.exe'
    $running=@(Get-LamdaProcess $exe)
    if(-not $running.Count){return $false}
    $request=Start-Process -FilePath $exe -ArgumentList '--quit' -PassThru
    if(-not $request.WaitForExit(10000)){throw 'LamdaUI is busy. Finish its current action and try again.'}
    foreach($process in $running){Wait-Process -Id $process.ProcessId -Timeout 15 -ErrorAction SilentlyContinue}
    if(@(Get-LamdaProcess $exe).Count){throw 'LamdaUI is still busy. Please try again shortly.'}
    return $true
}
function Assert-LamdaRecovery($Journal,[string]$JournalFile,[string]$Root,[hashtable]$Games) {
    if([IO.Path]::GetFullPath($Journal.root) -ne [IO.Path]::GetFullPath($Root)){throw 'The recovery record belongs to another installation.'}
    $allowed=New-Object Collections.Generic.List[string]
    foreach($name in @('Retail','Forever','Desktop','Updater')){$allowed.Add((Join-Path $Root $name).TrimEnd('\')+'\')}
    foreach($engine in $Journal.engines) {
        if(-not $Games.ContainsKey($engine.client) -or (Get-LamdaMember $Journal.games $engine.client) -ne $Games[$engine.client]){throw 'An earlier update used a different WoW folder. Choose that folder to recover it first.'}
        $expected=Join-Path (Join-Path $Root (Get-Culture).TextInfo.ToTitleCase($engine.client)) 'lamda-engine.exe'
        if($engine.exe -ne $expected){throw 'The recovery record contains an unexpected companion.'}
    }
    foreach($game in $Games.Values){foreach($addon in $script:LamdaAddons){$allowed.Add((Join-Path $game ('Interface\AddOns\'+$addon)).TrimEnd('\')+'\')}}
    $backupRoot=(Split-Path -Parent $JournalFile).TrimEnd('\')+'\files\'
    foreach($change in $Journal.changes) {
        $target=[IO.Path]::GetFullPath($change.target)
        if(-not @($allowed | Where-Object {$target.StartsWith($_,[StringComparison]::OrdinalIgnoreCase)}).Count){throw 'The recovery record points outside LamdaUI.'}
        foreach($name in @('Retail','Forever','Desktop','Updater')) {
            if($target.StartsWith((Join-Path $Root ($name+'\data')).TrimEnd('\')+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'The recovery record would overwrite saved companion data.'}
        }
        if($change.before -and (-not $change.backup -or -not [IO.Path]::GetFullPath($change.backup).StartsWith($backupRoot,[StringComparison]::OrdinalIgnoreCase))){throw 'The recovery backup is outside its expected folder.'}
        Assert-LamdaOrdinaryPath $target
    }
}
function Test-LamdaGeneratedAfterSeed($Change) {
    if($null -ne $Change.before -or -not [IO.File]::Exists($Change.target) -or (Get-Item -LiteralPath $Change.target).Length -gt 67108864){return $false}
    $name=[IO.Path]::GetFileName($Change.target)
    if($name -eq 'KBBData.lua'){$marker='-- LamdaUI managed KBB data v1';$variable='LamdaKBBData';$pattern='(?s)^-- LamdaUI managed KBB data v1\n-- Payload digest: ([0-9a-f]{64})\n(.*)$'}
    elseif($name -eq 'SimResults.lua'){$marker='-- LamdaUI managed sim results v1';$variable='LamdaSimResults';$pattern='(?s)^-- LamdaUI managed sim results v1\n-- Content digest: [0-9a-f]{64}\n-- Payload digest: ([0-9a-f]{64})\n(.*)$'}
    elseif($name -eq 'OverlaysData.lua'){$marker='-- LamdaUI managed Overlays data v1';$variable='LamdaOverlaysData';$pattern='(?s)^-- LamdaUI managed Overlays data v1\n-- Payload digest: ([0-9a-f]{64})\n(.*)$'}
    else{return $false}
    $sha=[Security.Cryptography.SHA256]::Create()
    try {
        $seedBody=if($name -eq 'OverlaysData.lua'){'LamdaOverlaysData = {["hash"]="4f53cda18c2baa0c",["watches"]={},}'}else{$variable+' = {}'}
        $seed=[Text.Encoding]::UTF8.GetBytes($marker+"`n"+$seedBody+"`n")
        $seedHash=[BitConverter]::ToString($sha.ComputeHash($seed)).Replace('-','').ToLowerInvariant()
        if($seedHash -ne $Change.after){return $false}
        $text=[IO.File]::ReadAllText($Change.target)
        if($text -notmatch $pattern){return $false}
        $expected=$Matches[1];$body=[Text.Encoding]::UTF8.GetBytes($Matches[2])
        return [BitConverter]::ToString($sha.ComputeHash($body)).Replace('-','').ToLowerInvariant() -eq $expected
    }finally{$sha.Dispose()}
}
function Restore-LamdaTransaction($Journal,[string]$JournalFile) {
    $Journal.status='rolling_back';Write-LamdaJson $JournalFile $Journal
    foreach($item in $Journal.engines) {if($item.requiresClosed -and (Test-LamdaGameOpen $item.client (Get-LamdaMember $Journal.games $item.client))){throw 'Close WoW, then run the installer again to finish restoring the previous version.'}}
    $null=Stop-LamdaPanel $Journal.root
    foreach($item in $Journal.engines) {if($item.arguments){$null=Stop-LamdaCompanion $item.exe}}
    foreach($change in $Journal.changes) {
        $now=Get-LamdaHash $change.target
        if($now -eq $change.before){continue}
        if($now -ne $change.after) {
            if(Test-LamdaGeneratedAfterSeed $change){Write-LamdaAtomic (Join-Path (Split-Path -Parent $JournalFile) ('generated\'+[IO.Path]::GetFileName($change.target))) ([IO.File]::ReadAllBytes($change.target))}
            else{throw 'A file changed outside the updater. Your backup is preserved for recovery.'}
        }
        if($null -eq $change.before) {if([IO.File]::Exists($change.target)){[IO.File]::Delete($change.target)}}
        else {
            if((Get-LamdaHash $change.backup) -ne $change.before){throw 'A backup failed its integrity check. Installation remains stopped.'}
            Write-LamdaAtomic $change.target ([IO.File]::ReadAllBytes($change.backup))
        }
    }
    foreach($item in $Journal.engines) {if($item.arguments){Start-LamdaCompanion $item.exe $item.arguments $item.client ''}}
    if($Journal.panelRunning -and [IO.File]::Exists((Join-Path $Journal.root 'Desktop\LamdaUI.exe'))){Start-Process (Join-Path $Journal.root 'Desktop\LamdaUI.exe') -ArgumentList '--hidden' | Out-Null}
    $Journal.status='rolled_back';Write-LamdaJson $JournalFile $Journal
}
function Install-LamdaRelease([string]$Release,[string]$Root,[hashtable]$Games,[string]$StatusFile,[string]$DesktopPath) {
    $bundles=Test-LamdaRelease $Release
    Assert-LamdaOrdinaryPath $Root
    $mutex=New-Object Threading.Mutex($false,'Local\LamdaUI.EngineControl');$owns=$false;$planRoot=$null
    try {
        try {$owns=$mutex.WaitOne(0)}catch [Threading.AbandonedMutexException] {$owns=$true}
        if(-not $owns){throw 'LamdaUI is already updating or starting. Please try again when it finishes.'}
        $planRoot=Join-Path ([IO.Path]::GetTempPath()) ('LamdaUI-module-plan-'+[guid]::NewGuid().ToString('N'))
        [IO.Directory]::CreateDirectory($planRoot) | Out-Null
        $transactions=Join-Path $Root 'Updater\data\transactions'
        if(Test-Path -LiteralPath $transactions) {
            foreach($file in Get-ChildItem -LiteralPath $transactions -Filter 'transaction.json' -Recurse -File) {
                $journal=Read-LamdaJson $file.FullName
                if($journal.status -in @('applying','rolling_back')) {
                    Assert-LamdaRecovery $journal $file.FullName $Root $Games
                    foreach($property in $journal.games.PSObject.Properties){if(Test-LamdaGameOpen $property.Name $property.Value){throw 'An earlier update was interrupted. Close WoW, then run Update to restore it.'}}
                    Restore-LamdaTransaction $journal $file.FullName
                }
            }
        }
        $changes=New-Object Collections.Generic.List[object]
        $enginePlans=New-Object Collections.Generic.List[object]
        $clientConfig=@{}
        $configPath=Join-Path $Root 'Updater\updater.json'
        $fresh=-not (Test-Path -LiteralPath $configPath)
        if($fresh -and (Test-Path -LiteralPath $Root)) {
            # A failed first installation leaves its backups here. Allow a retry
            # after recovery, while preserving an unrelated nonempty folder.
            $recoveryPrefix=$transactions.TrimEnd('\')+'\'
            foreach($file in Get-ChildItem -Force -LiteralPath $Root -Recurse -File) {
                if(-not $file.FullName.StartsWith($recoveryPrefix,[StringComparison]::OrdinalIgnoreCase)){throw 'The installation folder already contains files from another setup. They have been preserved.'}
                $stageName=$file.FullName.Substring($recoveryPrefix.Length).Split('\')[0]
                $recordPath=Join-Path (Join-Path $transactions $stageName) 'transaction.json'
                if(-not [IO.File]::Exists($recordPath)){throw 'An unfinished setup backup needs recovery. Its files have been preserved.'}
                $record=Read-LamdaJson $recordPath
                Assert-LamdaRecovery $record $recordPath $Root $Games
                if($record.status -notin @('rolled_back','prepared')){throw 'An unfinished installation needs recovery before setup can continue.'}
            }
        }
        if(Test-Path -LiteralPath $configPath) {
            $old=Read-LamdaJson $configPath
            if($old.product -ne 'LamdaUI'){throw 'That installation folder belongs to another application.'}
            if((Get-LamdaMember $old 'schema') -eq 2){foreach($p in $old.clients.PSObject.Properties){$clientConfig[$p.Name]=$p.Value}}
            elseif((Get-LamdaMember $old 'schema') -eq 1) {
                # The earlier developer launcher used these fixed game paths.
                # Preserve its other managed client when adopting the new UI.
                foreach($key in @('retail','forever')) {
                    $legacyGame=if($key -eq 'retail'){'D:\World of Warcraft\_retail_'}elseif([IO.File]::Exists('D:\World of Warcraft\_forever_\Wow.exe')){'D:\World of Warcraft\_forever_'}else{'D:\World of Warcraft\_classic_beta_'}
                    $legacyManifest=Join-Path $Root ((Get-Culture).TextInfo.ToTitleCase($key)+'\manifest.json')
                    if([IO.File]::Exists($legacyManifest) -and [IO.File]::Exists((Join-Path $legacyGame (Get-LamdaGameName $key $legacyGame)))) {
                        $legacy=Read-LamdaJson $legacyManifest
                        if($legacy.product -eq 'LamdaUI' -and $legacy.client -eq $key){$clientConfig[$key]=$legacyGame}
                    }
                }
            }
        }
        foreach($client in $Games.Keys) {
            $game=[IO.Path]::GetFullPath($Games[$client]);Assert-LamdaOrdinaryPath $game
            if(-not [IO.File]::Exists((Join-Path $game (Get-LamdaGameName $client $game)))){throw 'Choose the World of Warcraft folder for the selected game.'}
            $clientConfig[$client]=$game;$componentDir=Join-Path $Root (Get-Culture).TextInfo.ToTitleCase($client)
            $addons=Join-Path $game 'Interface\AddOns';$layoutChange=$false
            $addonFiles=Get-LamdaModulePlan $Release $client $componentDir $addons $bundles[$client].addon $planRoot
            foreach($kind in @('addon','engine')) {
                $bundle=$bundles[$client][$kind];$base=if($kind -eq 'addon'){$addons}else{$componentDir}
                $oldVersion=$null
                if($kind -eq 'engine' -and (Test-Path -LiteralPath (Join-Path $componentDir 'manifest.json'))) {
                    $installed=Read-LamdaJson (Join-Path $componentDir 'manifest.json')
                    if($installed.product -ne 'LamdaUI' -or $installed.client -ne $client){throw 'An existing companion belongs to a different game.'}
                    $oldVersion=$installed.version
                }
                if($kind -eq 'addon') {
                    foreach($addon in $script:LamdaAddons) {
                        $folder=Join-Path $addons $addon;Assert-LamdaOrdinaryPath $folder
                        if(Test-Path -LiteralPath $folder) {
                            if(-not @(Get-ChildItem -LiteralPath $folder -Recurse -File -Force).Count){$layoutChange=$true;continue}
                            $tocPath=Join-Path $folder ($addon+'.toc')
                            if(-not [IO.File]::Exists($tocPath)){throw 'An existing addon folder is not recognized. Its files have been preserved.'}
                            $toc=[IO.File]::ReadAllText($tocPath)
                            if(-not $toc.Contains('## X-LamdaFrames-Managed: 1')){throw 'An older or customized addon is already installed. Its files have been preserved; contact LamdaUI support to migrate it.'}
                            if($addon -eq 'lamdaUI' -and $toc -match '(?m)^## Version:\s*(\S+)'){$oldVersion=$Matches[1]}
                            foreach($file in Get-ChildItem -LiteralPath $folder -Recurse -File) {
                                if($addon -eq 'lamdaUI' -and $file.DirectoryName -eq $folder -and $file.Name -match '^\.(sim-results|kbb-data|overlays)-[0-9]+$'){continue}
                                $relative=$addon+'/'+$file.FullName.Substring($folder.Length+1).Replace('\','/')
                                if($null -eq (Get-LamdaMember $addonFiles $relative)) {
                                    $layoutChange=$true
                                    $changes.Add(@{target=$file.FullName;source=$null;before=(Get-LamdaHash $file.FullName);after=$null;backup=$null})
                                }
                            }
                        } else {$layoutChange=$true}
                    }
                }
                if($oldVersion -and (Get-LamdaVersion $oldVersion $client) -gt (Get-LamdaVersion $bundle.version $client)){throw 'A newer LamdaUI version is already installed. It will not be replaced by this release.'}
                $plannedFiles=New-Object Collections.Generic.List[object]
                if($kind -eq 'addon') {
                    foreach($property in $addonFiles.GetEnumerator()){$plannedFiles.Add(@{Name=$property.Key;Value=$property.Value.hash;Source=$property.Value.source})}
                } else {
                    foreach($property in $bundle.files.PSObject.Properties){$plannedFiles.Add(@{Name=$property.Name;Value=$property.Value;Source=(Join-Path $Release ($client+'/'+$kind+'/package/'+$property.Name))})}
                }
                foreach($property in $plannedFiles) {
                    $target=Join-Path $base $property.Name;Assert-LamdaOrdinaryPath $target
                    $before=Get-LamdaHash $target
                    if($kind -eq 'addon' -and $before -and $property.Name -in @('lamdaUI/KBBData.lua','lamdaUI/SimResults.lua','lamdaUI/OverlaysData.lua')){continue}
                    if($before -ne $property.Value) {
                        if($kind -eq 'addon' -and -not $before){$layoutChange=$true}
                        $changes.Add(@{target=$target;source=$property.Source;before=$before;after=$property.Value;backup=$null})
                    }
                }
                if($kind -eq 'engine') {
                    $source=Join-Path $Release ($client+'/engine/manifest.json');$target=Join-Path $componentDir 'manifest.json'
                    if((Get-LamdaHash $source) -ne (Get-LamdaHash $target)){$changes.Add(@{target=$target;source=$source;before=(Get-LamdaHash $target);after=(Get-LamdaHash $source);backup=$null})}
                }
            }
            if($layoutChange -and (Test-LamdaGameOpen $client $game)){throw 'This installation needs WoW closed once. You can keep playing and run Install again after you close it.'}
            $enginePlans.Add(@{exe=(Join-Path $componentDir 'lamda-engine.exe');client=$client;arguments=$null;version=$bundles[$client].engine.version;requiresClosed=$layoutChange})
        }
        foreach($kind in @('desktop','updater')) {
            $componentDir=Join-Path $Root (Get-Culture).TextInfo.ToTitleCase($kind);$bundle=$bundles[$kind]
            if(Test-Path -LiteralPath (Join-Path $componentDir 'manifest.json')) {
                $old=Read-LamdaJson (Join-Path $componentDir 'manifest.json')
                if($old.product -ne 'LamdaUI' -or $old.component -ne $kind -or (Get-LamdaVersion $old.version) -gt (Get-LamdaVersion $bundle.version)){throw 'A newer or different LamdaUI component is already installed.'}
            }
            foreach($property in $bundle.files.PSObject.Properties) {
                $target=Join-Path $componentDir $property.Name;Assert-LamdaOrdinaryPath $target
                if((Get-LamdaHash $target) -ne $property.Value){$changes.Add(@{target=$target;source=(Join-Path $Release ($kind+'/package/'+$property.Name));before=(Get-LamdaHash $target);after=$property.Value;backup=$null})}
            }
            $source=Join-Path $Release ($kind+'/manifest.json');$target=Join-Path $componentDir 'manifest.json'
            if((Get-LamdaHash $target) -ne (Get-LamdaHash $source)){$changes.Add(@{target=$target;source=$source;before=(Get-LamdaHash $target);after=(Get-LamdaHash $source);backup=$null})}
        }
        $stage=Join-Path $transactions ([DateTime]::Now.ToString('yyyyMMdd-HHmmss-fff')+'-'+[guid]::NewGuid().ToString('N'))
        [IO.Directory]::CreateDirectory($stage) | Out-Null
        $newConfig=Join-Path $stage 'new-config.json'
        Write-LamdaJson $newConfig @{product='LamdaUI';schema=2;repository=$script:LamdaRepository;clients=$clientConfig}
        if((Get-LamdaHash $configPath) -ne (Get-LamdaHash $newConfig)){$changes.Add(@{target=$configPath;source=$newConfig;before=(Get-LamdaHash $configPath);after=(Get-LamdaHash $newConfig);backup=$null})}
        if($changes.Count -eq 0) {
            Set-LamdaStatus $StatusFile 'complete' 'LamdaUI is already up to date.'
            return @{root=$Root;backup=$stage;clients=@($Games.Keys);files=0;desktop=$DesktopPath;fresh=$false}
        }
        Set-LamdaStatus $StatusFile 'installing' 'Backing up your current LamdaUI files...'
        $index=0
        foreach($change in $changes) {
            if((Get-LamdaHash $change.target) -ne $change.before){throw 'A file changed while setup was preparing. Please try again.'}
            if($change.before) {
                $change.backup=Join-Path $stage ('files\'+$index);$index++
                Write-LamdaAtomic $change.backup ([IO.File]::ReadAllBytes($change.target))
                if((Get-LamdaHash $change.backup) -ne $change.before){throw 'The backup could not be verified. No files were installed.'}
            }
        }
        $journal=@{status='prepared';root=$Root;games=$Games;changes=@($changes.ToArray());engines=@($enginePlans.ToArray());panelRunning=$false}
        $journalFile=Join-Path $stage 'transaction.json';Write-LamdaJson $journalFile $journal
        foreach($item in $enginePlans){if($item.requiresClosed -and (Test-LamdaGameOpen $item.client $Games[$item.client])){throw 'WoW reopened. Close it before this first installation.'}}
        try {
            $journal.status='applying';Write-LamdaJson $journalFile $journal
            foreach($item in $enginePlans){$launch=Get-LamdaLaunch $item.exe;$item.arguments=if($launch){$launch.arguments}else{$null};Write-LamdaJson $journalFile $journal;$null=Stop-LamdaCompanion $item.exe}
            $journal.panelRunning=Stop-LamdaPanel $Root;Write-LamdaJson $journalFile $journal
            Set-LamdaStatus $StatusFile 'installing' 'Installing LamdaUI...'
            foreach($change in ($changes | Sort-Object {$_.target.EndsWith('.toc',[StringComparison]::OrdinalIgnoreCase)})) {
                if((Get-LamdaHash $change.target) -ne $change.before){throw 'A file changed during installation. Your previous installation will be restored.'}
                if($null -eq $change.after){[IO.File]::Delete($change.target)}
                else {
                    if((Get-LamdaHash $change.source) -ne $change.after){throw 'A setup file changed. Your previous installation will be restored.'}
                    Write-LamdaAtomic $change.target ([IO.File]::ReadAllBytes($change.source))
                }
                if((Get-LamdaHash $change.target) -ne $change.after){throw 'An installed file could not be verified.'}
            }
            foreach($item in $enginePlans) {
                if($item.requiresClosed -and (Test-LamdaGameOpen $item.client $Games[$item.client])){throw 'WoW opened before setup finished. Your previous installation will be restored.'}
                if($item.arguments){Start-LamdaCompanion $item.exe $item.arguments $item.client $item.version}
            }
            $journal.status='complete';Write-LamdaJson $journalFile $journal
        } catch {
            $failure=$_
            try{Restore-LamdaTransaction $journal $journalFile}catch{throw ('Setup stopped. Your backup is at '+$stage+'. '+$_.Exception.Message)}
            throw $failure
        }
        Set-LamdaStatus $StatusFile 'complete' 'LamdaUI is ready. Open WoW normally; use /reload if you kept it open.'
        return @{root=$Root;backup=$stage;clients=@($Games.Keys);files=$changes.Count;desktop=$DesktopPath;fresh=$fresh}
    } finally {
        if($planRoot -and (Test-Path -LiteralPath $planRoot)){Remove-Item -LiteralPath $planRoot -Recurse -Force}
        if($owns){$mutex.ReleaseMutex()};$mutex.Dispose()
    }
}
function Complete-LamdaSetup($Result) {
    $exe=Join-Path $Result.root 'Desktop\LamdaUI.exe'
    if($Result.desktop) {
        $shortcut=Join-Path $Result.desktop 'LamdaUI.lnk';$shell=New-Object -ComObject WScript.Shell
        if(Test-Path -LiteralPath $shortcut) {
            $old=$shell.CreateShortcut($shortcut)
            if($old.TargetPath -ne $exe){throw 'LamdaUI installed successfully. An existing desktop shortcut was kept; open LamdaUI from its installation folder.'}
            Copy-Item -LiteralPath $shortcut -Destination (Join-Path $Result.backup 'LamdaUI.lnk')
        }
        $link=$shell.CreateShortcut($shortcut);$link.TargetPath=$exe;$link.WorkingDirectory=Split-Path -Parent $exe
        $client=if('retail' -in $Result.clients){'retail'}else{'forever'}
        $link.Arguments='--start '+$client;$link.IconLocation=$exe+',0';$link.Description='Open LamdaUI';$link.Save()
    }
    $client=if('retail' -in $Result.clients){'retail'}else{'forever'}
    $arguments=if($Result.fresh){'--start '+$client+' --hidden'}else{'--hidden'}
    Start-Process -FilePath $exe -ArgumentList $arguments | Out-Null
}
