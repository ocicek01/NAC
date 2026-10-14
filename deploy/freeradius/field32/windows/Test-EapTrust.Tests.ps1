$ErrorActionPreference = 'Stop'
$profilePath = Join-Path $PSScriptRoot 'nac-field32.xml'
& (Join-Path $PSScriptRoot 'Test-EapProfile.ps1') -ProfilePath $profilePath
$source = Get-Content -LiteralPath $profilePath -Raw
$checker = Join-Path $PSScriptRoot 'Test-EapTrust.ps1'
$cases = @(
    @{ Name = 'padded fingerprint'; Xml = $source; Accept = $true },
    @{ Name = 'Windows unpadded fingerprint'; Xml = $source.Replace('07', '7').Replace('03', '3'); Accept = $true },
    @{ Name = 'missing fingerprint'; Xml = ($source -replace '<TrustedRootCAHash>.*?</TrustedRootCAHash>', ''); Accept = $false },
    @{ Name = 'ignored plural element'; Xml = $source.Replace('TrustedRootCAHash', 'TrustedRootCAHashes'); Accept = $false },
    @{ Name = 'wrong fingerprint'; Xml = $source.Replace('CA BD', 'CB BD'); Accept = $false },
    @{ Name = 'wrong server'; Xml = $source.Replace('radius.hakkari.edu.tr', 'other.example'); Accept = $false },
    @{ Name = 'certificate prompting enabled'; Xml = $source.Replace('<DisablePrompt>true', '<DisablePrompt>false'); Accept = $false },
    @{ Name = 'PAP missing'; Xml = $source.Replace('<PAPAuthentication />', ''); Accept = $false }
)
foreach ($case in $cases) {
    $accepted = $true
    try { & $checker -Configuration ([xml]$case.Xml) } catch { $accepted = $false }
    if ($accepted -ne $case.Accept) { throw "Unexpected result: $($case.Name)" }
    Write-Output "PASS: $($case.Name)"
}
