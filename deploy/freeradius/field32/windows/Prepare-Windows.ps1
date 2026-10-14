#Requires -RunAsAdministrator
[CmdletBinding()]
param([switch]$Rollback)

$ErrorActionPreference = 'Stop'
$expectedMac = '309C239B97AA'
$adapter = @(Get-NetAdapter | Where-Object { ($_.MacAddress -replace '[:-]', '') -eq $expectedMac })
if ($adapter.Count -ne 1) { throw 'Expected test NIC 30:9C:23:9B:97:AA was not found.' }
$interface = $adapter[0].Name
$backup = Join-Path $env:ProgramData 'NAC-Field32-Backup'
$stateFile = Join-Path $backup 'state.json'

if ($Rollback) {
    if (!(Test-Path -LiteralPath $stateFile)) { throw 'No saved test state.' }
    $state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
    Set-Service dot3svc -StartupType Manual
    Start-Service dot3svc
    & netsh lan delete profile "interface=$interface"
    foreach ($profile in @(Get-ChildItem -LiteralPath $backup -Filter '*.xml')) {
        & netsh lan add profile "filename=$($profile.FullName)" "interface=$interface"
        if ($LASTEXITCODE -ne 0) { throw 'Original profile restore failed.' }
    }
    Set-Service dot3svc -StartupType $state.StartType
    if ($state.Status -eq 'Stopped') { Stop-Service dot3svc }
    Write-Output 'Original wired profile and service state restored.'
    exit
}

$root = Get-ChildItem Cert:\LocalMachine\Root | Where-Object { $_.Thumbprint -eq 'CABD2A79A1076A31F21D253635CB039D4329A5E8' }
if (!$root) { throw 'Trusted ISRG Root X1 is missing. Update the Windows trusted root store first.' }
$profilePath = Join-Path $PSScriptRoot 'nac-field32.xml'
if (!(Test-Path -LiteralPath $profilePath)) { throw 'nac-field32.xml is missing.' }
if (!(Test-Path -LiteralPath $stateFile)) {
    New-Item -Path $backup -ItemType Directory -Force | Out-Null
    $service = Get-Service dot3svc
    @{ StartType = $service.StartType.ToString(); Status = $service.Status.ToString(); Interface = $interface } |
        ConvertTo-Json | Set-Content -LiteralPath $stateFile -Encoding UTF8
    if ($service.StartType -eq 'Disabled') { Set-Service dot3svc -StartupType Manual }
    Start-Service dot3svc
    & netsh lan export profile "folder=$backup" "interface=$interface"
}
Set-Service dot3svc -StartupType Automatic
Start-Service dot3svc
& netsh lan add profile "filename=$profilePath" "interface=$interface"
if ($LASTEXITCODE -ne 0) { throw 'Wired EAP-TTLS profile import failed.' }
Write-Output "Profile loaded on $interface. Enter the temporary LDAP username/password when Windows requests network credentials."
& netsh lan show interfaces
