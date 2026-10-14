[CmdletBinding()]
param([string]$ProfilePath)

$ErrorActionPreference = 'Stop'
if (!$ProfilePath) { $ProfilePath = Join-Path $PSScriptRoot 'nac-field32.xml' }
if (!('NacField32.EapConfig' -as [type])) {
    Add-Type @'
using System;
using System.Runtime.InteropServices;
namespace NacField32 {
 public class EapConfig {
  [StructLayout(LayoutKind.Sequential)]
  public struct Method { public byte Type; public uint Vendor; public uint VendorType; public uint Author; }
  [DllImport("Eappcfg.dll")]
  public static extern uint EapHostPeerConfigXml2Blob(uint flags, IntPtr doc, out uint size, out IntPtr blob, out Method method, out IntPtr error);
  [DllImport("Eappcfg.dll")]
  public static extern uint EapHostPeerConfigBlob2Xml(uint flags, Method method, uint size, IntPtr blob, [MarshalAs(UnmanagedType.Interface)] out object doc, out IntPtr error);
  [DllImport("Eappcfg.dll")] public static extern void EapHostPeerFreeMemory(IntPtr p);
  [DllImport("Eappcfg.dll")] public static extern void EapHostPeerFreeErrorMemory(IntPtr p);
 }
}
'@
}
[xml]$profile = Get-Content -LiteralPath $ProfilePath -Raw
$config = $profile.SelectSingleNode("//*[local-name()='EapHostConfig']")
if (!$config) { throw 'EapHostConfig is missing.' }
$doc = New-Object -ComObject Msxml2.DOMDocument.6.0
if (!$doc.loadXML($config.OuterXml)) { throw 'EAP XML cannot be parsed.' }
$unknown = [Runtime.InteropServices.Marshal]::GetIUnknownForObject($doc.documentElement)
$node = [IntPtr]::Zero
$blob = [IntPtr]::Zero
$errorPtr = [IntPtr]::Zero
$roundtripError = [IntPtr]::Zero
$roundtrip = $null
try {
    $iid = [Guid]'2933BF80-7B36-11d2-B20E-00C04F983E60'
    $hr = [Runtime.InteropServices.Marshal]::QueryInterface($unknown, [ref]$iid, [ref]$node)
    if ($hr -ne 0) { throw "Cannot obtain XML DOM node: $hr" }
    $size = [uint32]0
    $method = New-Object NacField32.EapConfig+Method
    $result = [NacField32.EapConfig]::EapHostPeerConfigXml2Blob(0, $node, [ref]$size, [ref]$blob, [ref]$method, [ref]$errorPtr)
    if ($result -ne 0) { throw "Windows rejected EAP configuration: error $result. No profile was installed." }
    if ($method.Type -ne 21 -or $size -eq 0) { throw 'Expected a non-empty EAP-TTLS configuration.' }
    $result = [NacField32.EapConfig]::EapHostPeerConfigBlob2Xml(0, $method, $size, $blob, [ref]$roundtrip, [ref]$roundtripError)
    if ($result -ne 0) { throw "Windows EAP configuration readback failed: $result" }
    & (Join-Path $PSScriptRoot 'Test-EapTrust.ps1') -Configuration ([xml]$roundtrip.xml)
    Write-Output 'Windows EAP-TTLS configuration and root CA readback: PASS'
} finally {
    if ($roundtrip) { [void][Runtime.InteropServices.Marshal]::ReleaseComObject($roundtrip) }
    if ($roundtripError -ne [IntPtr]::Zero) { [NacField32.EapConfig]::EapHostPeerFreeErrorMemory($roundtripError) }
    if ($blob -ne [IntPtr]::Zero) { [NacField32.EapConfig]::EapHostPeerFreeMemory($blob) }
    if ($errorPtr -ne [IntPtr]::Zero) { [NacField32.EapConfig]::EapHostPeerFreeErrorMemory($errorPtr) }
    if ($node -ne [IntPtr]::Zero) { [void][Runtime.InteropServices.Marshal]::Release($node) }
    [void][Runtime.InteropServices.Marshal]::Release($unknown)
}
