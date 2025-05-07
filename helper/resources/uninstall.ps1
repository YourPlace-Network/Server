Install-Module -Name SQLite -Force
Import-Module SQLite

$CurrentUserIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
$CurrentUserName = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name.Split('\')[1]

function Test-Administrator {
    $windowsPrincipal = New-Object Security.Principal.WindowsPrincipal($CurrentUserIdentity)
    $adminRole = [Security.Principal.WindowsBuiltInRole]::Administrator
    return $windowsPrincipal.IsInRole($adminRole)
}
function Reset-Database($databasePath) {
    # Wipes the YourPlace database file of user data
    Write-Output "Cleaning database: $databasePath" | Out-File -FilePath 'uninstall.log' -Append
    $connection = New-Object System.Data.SQLite.SQLiteConnection('Data Source='+$databasePath+';')
    $connection.Open()
    $command = $connection.CreateCommand()
    $commands = @(
        "DELETE FROM authExpired;",
        "DELETE FROM authNonce;"
    )
    foreach ($cmd in $commands) {
        $command.CommandText = $cmd
        $reader = $command.ExecuteReader()
    }
    $connection.Close()
}
function Main() {
    if (-not (Test-Administrator)) {
        Write-Output "This script must be run as an administrator" | Out-File -FilePath 'uninstall.log' -Append
        exit 1
    }

    $keepUpload = $true
    $keepBlockchain = $false
    foreach ($arg in $args) {
        if ($arg -eq "-keepUpload") {
            $keepUpload = $true
        } elseif ($arg -eq "-keepBlockchain") {
            $keepBlockchain = $true
        }
    }

    Write-Output "Removing scheduled tasks..." | Out-File -FilePath 'uninstall.log' -Append
    schtasks /Delete /TN "YourPlaceHelper" /F 2>$null

    Write-Output "Stopping running processes..." | Out-File -FilePath 'uninstall.log' -Append
    Stop-Process -Name "YourPlace*" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "YourPlace-*" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "YourPlaceHelper" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "YourPlaceIpfs" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "YourPlaceFfmpeg" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2

    Write-Output "Removing application files..." | Out-File -FilePath 'uninstall.log' -Append
    Remove-Item -Path "C:\Users\$CurrentUserName\AppData\Local\YourPlace" -Recurse -Force -ErrorAction SilentlyContinue

    Write-Output "Removing shortcuts..." | Out-File -FilePath 'uninstall.log' -Append
    Remove-Item -Path "C:\Users\$CurrentUserName\Desktop\YourPlace.lnk" -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "C:\Users\$CurrentUserName\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\YourPlace.lnk" -Force -ErrorAction SilentlyContinue

    Write-Output "Removing data files..." | Out-File -FilePath 'uninstall.log' -Append
    $dataFolder = "C:\Users\$CurrentUserName\YourPlace"
    if ($KeepUpload) {
        Write-Output "Keeping uploaded files..." | Out-File -FilePath 'uninstall.log' -Append
        if (Test-Path -Path $dataFolder) {
            $foldersToKeep = @(".ipfs", "upload")
            $allItems = Get-ChildItem -Path $dataFolder -Force
            foreach ($item in $allItems) {
                # Skip the user content folders
                if ($item.PSIsContainer -and $foldersToKeep -contains $item.Name) {
                    Write-Output "Keeping Data Folder: $($item.FullName)" | Out-File -FilePath 'uninstall.log' -Append
                    continue
                }
                if ($keepBlockchain) {
                    # Skip the database file
                    if (-not $item.PSIsContainer -and $item.Extension -eq ".db") {
                        Write-Output "Keeping Database File: $( $item.FullName )" | Out-File -FilePath 'uninstall.log' -Append
                        Reset-Database($item.Path)
                        continue
                    }
                }
                # Delete everything else
                Write-Output "Deleting: $($item.FullName)" | Out-File -FilePath 'uninstall.log' -Append
                Remove-Item -Path $item.FullName -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    } else {
        Write-Output "Removing all user data..." | Out-File -FilePath 'uninstall.log' -Append
        Remove-Item -Path "C:\Users\$CurrentUserName\YourPlace" -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Output "YourPlace has been uninstalled." | Out-File -FilePath 'uninstall.log' -Append
}

Main $args # Call the main function to execute the script