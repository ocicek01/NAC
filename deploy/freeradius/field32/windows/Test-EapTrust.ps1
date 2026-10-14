[CmdletBinding()]
param([Parameter(Mandatory = $true)][xml]$Configuration)

$ErrorActionPreference = 'Stop'
$ns = New-Object System.Xml.XmlNamespaceManager($Configuration.NameTable)
$ns.AddNamespace('ttls', 'http://www.microsoft.com/provisioning/EapTtlsConnectionPropertiesV1')
$server = $Configuration.SelectSingleNode('//ttls:ServerNames', $ns)
$hashes = @($Configuration.SelectNodes('//ttls:TrustedRootCAHash', $ns))
$prompt = $Configuration.SelectSingleNode('//ttls:DisablePrompt', $ns)
$pap = $Configuration.SelectSingleNode('//ttls:PAPAuthentication', $ns)
if (!$server -or $server.InnerText -ne 'radius.hakkari.edu.tr' -or $hashes.Count -ne 1 -or
    !$prompt -or $prompt.InnerText -notin @('true', '1') -or !$pap) {
    throw 'EAP profile is missing the required TTLS/PAP or server trust settings.'
}
# Windows exports hexadecimal bytes without leading zeros (07 becomes 7).
$raw = $hashes[0].InnerText.Trim()
if ($raw -match '^[0-9a-fA-F]{40}$') {
    $hash = $raw.ToUpperInvariant()
} else {
    $bytes = @($raw -split '\s+')
    if ($bytes.Count -ne 20) { throw 'Expected exactly 20 root CA fingerprint bytes.' }
    $hash = ''
    foreach ($byte in $bytes) {
        if ($byte -notmatch '^[0-9a-fA-F]{1,2}$') { throw 'Invalid root CA fingerprint byte.' }
        $hash += $byte.PadLeft(2, '0').ToUpperInvariant()
    }
}
if ($hash -ne 'CABD2A79A1076A31F21D253635CB039D4329A5E8') {
    throw 'EAP profile does not pin the required ISRG Root X1 certificate.'
}
