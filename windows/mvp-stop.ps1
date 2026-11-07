$ErrorActionPreference='Stop'
$engine=Join-Path $PSScriptRoot 'lamda-engine.exe'
foreach($name in @('state.json','demo-state.json')){
 $path=Join-Path $PSScriptRoot ('data\'+$name)
 if(-not(Test-Path -LiteralPath $path)){continue}
 try{$state=Get-Content -LiteralPath $path -Raw -Encoding UTF8 | ConvertFrom-Json}catch{continue}
 if($state.pid -isnot [int] -or $state.pid -le 0){continue}
 $process=Get-Process -Id $state.pid -ErrorAction SilentlyContinue
 if($process -and $process.Path -eq $engine){Stop-Process -Id $process.Id}
}
Write-Host 'MVP stopped. Combat logging is controlled separately in WoW.'
