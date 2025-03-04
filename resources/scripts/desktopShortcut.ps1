$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($isAdmin) {
    Write-Host "This script is running as administrator."
} else {
    Write-Host "This script is NOT running as administrator."
    Exit
}
# Set the variables
$AppDataDir = $env:LOCALAPPDATA
$TempDir = $env:TEMP
$TargetPath = "$AppDataDir\YourPlace\YourPlace.exe"
$ShortcutName = "YourPlace"
$IconLocation = "$TempDir\yp_temp_image.png"

# Get the Desktop path
$DesktopPath = [Environment]::GetFolderPath("Desktop")

# Create the shortcut object
$Shell = New-Object -ComObject ("WScript.Shell")
$Shortcut = $Shell.CreateShortcut("$DesktopPath\$ShortcutName.lnk")

# Set the properties for the shortcut
$Shortcut.TargetPath = $TargetPath
$Shortcut.IconLocation = $IconLocation
$Shortcut.WorkingDirectory = (Split-Path $TargetPath)

$Shortcut.Save()
