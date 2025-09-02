param(
    [Parameter(Mandatory=$true)]
    [string]$Repository,

    [Parameter(Mandatory=$true)]
    [string]$AssetPattern
)

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

try {
    # Get latest release info from GitHub API
    $apiUrl = "https://api.github.com/repos/$Repository/releases/latest"
    $release = Invoke-RestMethod -Uri $apiUrl -Method Get

    # Filter assets by regex pattern
    $matchingAssets = $release.assets | Where-Object { $_.name -match $AssetPattern }

    if ($matchingAssets.Count -eq 0) {
        Write-Error "No assets found matching pattern: $AssetPattern"
        exit 1
    }

    # Download each matching asset
    foreach ($asset in $matchingAssets) {
        $outputPath = Join-Path $scriptDir $asset.name
        Write-Host "Downloading $($asset.name)..."

        Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $outputPath
        Write-Host "Downloaded to: $outputPath"
    }

    Write-Host "Download completed successfully."
}
catch {
    Write-Error "Failed to download: $($_.Exception.Message)"
    exit 1
}