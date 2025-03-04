# Extract version from main.go
$version = (Select-String -Path "main.go" -Pattern 'version.*=.*".*"').Matches[0].Value -replace '.*version.*=.*"(.*)".*','$1'
if (-not $version) {
    Write-Host "Could not extract version from main.go"
    exit 1
}
$version | Out-File -FilePath src\core\host\bin\helper\win\helper.version -NoNewline -Encoding UTF8

# Application Variables
$APP_NAME = "YourPlace"
$PRODUCT_NAME = "YourPlace"

# Path to rsrc executable
$RSRC = "resources\windows\rsrc_windows_amd64.exe"
if (-not (Test-Path $RSRC)) {
    Write-Error "rsrc executable not found"
    exit 1
}

# Create a version-aware manifest file
$manifestContent = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
    <assemblyIdentity version="$version" processorArchitecture="*" name="$APP_NAME" type="win32"/>
    <description>$PRODUCT_NAME</description>
    <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
        <security>
            <requestedPrivileges>
                <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
            </requestedPrivileges>
        </security>
    </trustInfo>
    <dependency>
        <dependentAssembly>
            <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
        </dependentAssembly>
    </dependency>
    <application xmlns="urn:schemas-microsoft-com:asm.v3">
        <windowsSettings>
            <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
            <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2, PerMonitor</dpiAwareness>
            <longPathAware xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">true</longPathAware>
            <heapType xmlns="http://schemas.microsoft.com/SMI/2020/WindowsSettings">SegmentHeap</heapType>
        </windowsSettings>
    </application>
    <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
        <application>
            <!-- Windows 10/11 -->
            <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
        </application>
    </compatibility>
</assembly>
"@

# Save the manifest
$manifestPath = "target\app.manifest"
New-Item -ItemType Directory -Force -Path "target" | Out-Null
$manifestContent | Out-File -FilePath $manifestPath -Encoding UTF8

# Generate the syso file using the specific rsrc path
& $RSRC -manifest $manifestPath -ico "resources\windows\AppIcon.ico" -arch amd64 -o rsrc_windows_amd64.syso

if ($LASTEXITCODE -ne 0) {
    Write-Error "Error: rsrc failed to generate resources"
    exit 1
}

# Optionally, if you want to sign the executable (requires a code signing certificate):
# $certificatePath = "path\to\your\certificate.pfx"
# $certificatePassword = "your-certificate-password"
# if (Test-Path $certificatePath) {
#     $signingCert = Get-PfxCertificate -FilePath $certificatePath
#     Set-AuthenticodeSignature -FilePath ".\target\$APP_NAME.exe" -Certificate $signingCert
# }