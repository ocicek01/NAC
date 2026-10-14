#Requires -RunAsAdministrator
[CmdletBinding()]
param([switch]$Rollback)

$ErrorActionPreference = 'Stop'
function Invoke-NetshChecked {
    param([string[]]$NetshArguments)
    $output = @(& netsh.exe @NetshArguments 2>&1)
    $exitCode = $LASTEXITCODE
    $output | Write-Output
    # netsh can report an import error while returning exit code zero.
    if ($exitCode -ne 0 -or ($output -join "`n") -match '(?im)^\s*(Error\b|Hata\b)') {
        throw 'netsh failed. The wired profile operation was not confirmed.'
    }
}
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
        Invoke-NetshChecked -NetshArguments @('lan', 'add', 'profile', "filename=$($profile.FullName)", "interface=$interface")
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
& (Join-Path $PSScriptRoot 'Test-EapProfile.ps1') -ProfilePath $profilePath
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
Invoke-NetshChecked -NetshArguments @('lan', 'add', 'profile', "filename=$profilePath", "interface=$interface")
$verification = Join-Path $env:TEMP ("nac-profile-check-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $verification | Out-Null
try {
    Invoke-NetshChecked -NetshArguments @('lan', 'export', 'profile', "folder=$verification", "interface=$interface")
    $exports = @(Get-ChildItem -LiteralPath $verification -Filter '*.xml')
    if ($exports.Count -ne 1) { throw 'Installed wired profile could not be read back.' }
    [xml]$installed = Get-Content -LiteralPath $exports[0].FullName -Raw
    $ns = New-Object System.Xml.XmlNamespaceManager($installed.NameTable)
    $ns.AddNamespace('ttls', 'http://www.microsoft.com/provisioning/EapTtlsConnectionPropertiesV1')
    $server = $installed.SelectSingleNode('//ttls:ServerNames', $ns)
    $hash = $installed.SelectSingleNode('//ttls:TrustedRootCAHashes', $ns)
    $prompt = $installed.SelectSingleNode('//ttls:DisablePrompt', $ns)
    $pap = $installed.SelectSingleNode('//ttls:PAPAuthentication', $ns)
    if (!$server -or $server.InnerText -ne 'radius.hakkari.edu.tr' -or !$hash -or
        ($hash.InnerText -replace '\s', '') -ne 'CABD2A79A1076A31F21D253635CB039D4329A5E8' -or
        !$prompt -or $prompt.InnerText -ne 'true' -or !$pap) {
        throw 'Installed profile does not match the required TTLS/PAP and server trust settings.'
    }
} finally {
    Get-ChildItem -LiteralPath $verification -File | Remove-Item
    Remove-Item -LiteralPath $verification
}
Write-Output "Profile loaded on $interface. Enter the temporary LDAP username/password when Windows requests network credentials."
& netsh lan show interfaces
