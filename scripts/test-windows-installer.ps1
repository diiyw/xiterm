param([Parameter(Mandatory=$true)][string]$Version)
$ErrorActionPreference = 'Stop'
$package = (Resolve-Path "dist/xiterm-$Version-windows-amd64.msi").Path
$installRoot = Join-Path $env:ProgramFiles 'XiTerm'
function Run-Msi([string]$Arguments) {
    $process = Start-Process msiexec.exe -ArgumentList $Arguments -PassThru -Wait
    if ($process.ExitCode -notin @(0, 3010)) { throw "msiexec failed with exit code $($process.ExitCode)" }
}
$installed = $false
try {
    Run-Msi "/i `"$package`" /qn /norestart /l*v `"$env:RUNNER_TEMP/xiterm-install.log`""
    $installed = $true
    $exe = Join-Path $installRoot 'xiterm.exe'
    if (!(Test-Path $exe)) { throw 'Installed executable is missing' }
    if (!(Test-Path (Join-Path $installRoot 'Notices/README.md'))) { throw 'Installed notices are missing' }
    $versionFile = Join-Path $env:RUNNER_TEMP 'xiterm-version.txt'
    $process = Start-Process $exe -ArgumentList '-version' -RedirectStandardOutput $versionFile -PassThru -Wait
    if ($process.ExitCode -ne 0) { throw 'Installed executable failed to start' }
    if ((Get-Content $versionFile -Raw).Trim() -ne "XiTerm $Version") { throw 'Executable version does not match tag' }
    Add-Type -AssemblyName System.Drawing
    $icon = [System.Drawing.Icon]::ExtractAssociatedIcon($exe)
    if ($null -eq $icon) { throw 'Executable application icon is missing' }
    $icon.Dispose()

} finally {
    if ($installed) { Run-Msi "/x `"$package`" /qn /norestart /l*v `"$env:RUNNER_TEMP/xiterm-uninstall.log`"" }
}
if (Test-Path (Join-Path $installRoot 'xiterm.exe')) { throw 'Uninstall left the executable behind' }
Write-Output 'PASS: MSI install, executable version, application icon and uninstall'
