$content = Get-Content -Path helper\helper_win.go -Raw
$versionMatch = [regex]::Match($content, 'version\s+=\s+"(.+?)"')
if ($versionMatch.Success) {
    $VERSION = $versionMatch.Groups[1].Value
    Write-Output "Helper Version: $VERSION"
    Write-Output $VERSION | Out-File -FilePath src\core\host\bin\helper\win\helper.version -NoNewline -Force
} else {
    Write-Host "Could not extract version from helper_win.go"
}


# $VERSION = (Select-String -Path helper\helper_win.go -Pattern 'version.*=.*"' | ForEach-Object { $_.Line -match 'version.*=.*"(.+?)"'; $matches[1] })
# Write-Output "Version: $VERSION"
# Write-Output $VERSION | Out-File -FilePath src\core\host\bin\helper\win\helper.version -NoNewline -Force