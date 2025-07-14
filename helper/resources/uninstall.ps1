Set-Location "C:\Windows\Temp"
Write-Output "Changed working directory to: $(Get-Location)" | Out-File -FilePath $LogPath -Append

# Small delay to ensure the directory change takes effect
Start-Sleep -Milliseconds 1000
Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Force -Scope CurrentUser

Install-Module -Name SQLite -Force
Import-Module SQLite

$CurrentUserIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()


function Get-YourPlaceUser {
    $processes = Get-Process -Name "YourPlace*" -ErrorAction SilentlyContinue |
            Where-Object {
                $_.ProcessName -notin @("YourPlaceHelper", "YourPlaceIpfs", "YourPlaceFfmpeg")
            }
    foreach ($process in $processes) {
        $owner = (Get-WmiObject Win32_Process -Filter "ProcessId = $($process.Id)").GetOwner()
        if ($owner.ReturnValue -eq 0 -and $owner.User -and $owner.User -ne "SYSTEM") {
            return $owner.User
        }
    }
}
$YourPlaceUser = Get-YourPlaceUser
$LogPath = "C:\Users\$YourPlaceUser\AppData\Local\Temp\yourplaceuninstall.log"
Write-Output "Changed working directory to: $(Get-Location)" | Out-File -FilePath $LogPath -Append
function Test-Administrator {
    $windowsPrincipal = New-Object Security.Principal.WindowsPrincipal($CurrentUserIdentity)
    $adminRole = [Security.Principal.WindowsBuiltInRole]::Administrator
    return $windowsPrincipal.IsInRole($adminRole)
}
function Reset-Database($databasePath) {
    # Wipes the YourPlace database file of user data
    Write-Output "Cleaning database: $databasePath" | Out-File -FilePath $LogPath -Append
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
        Write-Output "This script must be run as an administrator" | Out-File -FilePath $LogPath -Append
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
    Write-Output "Removing scheduled tasks..." | Out-File -FilePath $LogPath -Append
    schtasks /Delete /TN "YourPlaceHelper" /F 2>$null
    Write-Output "Stopping running processes..." | Out-File -FilePath $LogPath -Append
    $ProcessPatterns = @("YourPlace*", "YourPlace-*", "YourPlaceHelper", "YourPlaceIpfs", "YourPlaceFfmpeg")
    foreach ($pattern in $ProcessPatterns) {
        $process = Get-Process -Name $pattern -ErrorAction SilentlyContinue
        if ($process) {
            $process | Stop-Process -Force -ErrorAction SilentlyContinue
            try {
                $process | Wait-Process -Timeout 20 -ErrorAction Stop
            }
            catch {
                Write-Output "Process $pattern did not stop within timeout"
            }
        }
    }
    Start-Sleep -Milliseconds 500
    Write-Output "Removing application files..." | Out-File -FilePath $LogPath -Append
    Remove-Item -Path "C:\Users\$YourPlaceUser\AppData\Local\YourPlace" -Recurse -Force -ErrorAction SilentlyContinue -ErrorVariable removeError
    #if ($removeError) {
    #    Write-Output "ERROR: Failed to remove AppData\Local\YourPlace: $removeError" | Out-File -FilePath $LogPath -Append
    #    $handlePath = "$env:TEMP\handle64.exe"
    #    if (-not (Test-Path $handlePath)) {
    #        Write-Output "Downloading Handle.exe to check file locks..." | Out-File -FilePath $LogPath -Append
    #        try {
    #            Invoke-WebRequest -Uri "https://live.sysinternals.com/handle64.exe" -OutFile $handlePath -ErrorAction Stop
    #        } catch {
    #            Write-Output "Could not download Handle.exe: $_" | Out-File -FilePath $LogPath -Append
    #        }
    #    }

    #    if (Test-Path $handlePath) {
    #        Write-Output "Running Handle.exe to find locks..." | Out-File -FilePath $LogPath -Append
    #        $handleOutput = & $handlePath -nobanner -accepteula "C:\Users\$YourPlaceUser\AppData\Local\YourPlace" 2>$null

    #        if ($handleOutput) {
    #            Write-Output "Handle.exe found these processes using the directory:" | Out-File -FilePath $LogPath -Append
    #            $handleOutput | Out-File -FilePath $LogPath -Append
    #            $handleOutput | ForEach-Object {
    #                if ($_ -match "^(\S+)\s+pid:\s+(\d+)") {
    #                   $procName = $Matches[1]
    #                   $procId = $Matches[2]
    #                    Write-Output "Process $procName (PID: $procId) has a handle on the directory" | Out-File -FilePath $LogPath -Append
    #                }
    #            }
    #        } else {
    #           Write-Output "Handle.exe found no locks on the directory" | Out-File -FilePath $LogPath -Append
    #        }
    # }
    #}

    Write-Output "Removing shortcuts..." | Out-File -FilePath $LogPath -Append
    Remove-Item -Path "C:\Users\$YourPlaceUser\Desktop\YourPlace.lnk" -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "C:\Users\$YourPlaceUser\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\YourPlace.lnk" -Force -ErrorAction SilentlyContinue

    Write-Output "Removing data files..." | Out-File -FilePath $LogPath -Append
    $dataFolder = "C:\Users\$YourPlaceUser\YourPlace"
    if ($KeepUpload) {
        Write-Output "Keeping uploaded files..." | Out-File -FilePath $LogPath -Append
        if (Test-Path -Path $dataFolder) {
            $foldersToKeep = @(".ipfs", "upload")
            $allItems = Get-ChildItem -Path $dataFolder -Force
            foreach ($item in $allItems) {
                # Skip the user content folders
                if ($item.PSIsContainer -and $foldersToKeep -contains $item.Name) {
                    Write-Output "Keeping Data Folder: $($item.FullName)" | Out-File -FilePath $LogPath -Append
                    continue
                }
                if ($keepBlockchain) {
                    # Skip the database file
                    if (-not $item.PSIsContainer -and $item.Extension -eq ".db") {
                        Write-Output "Keeping Database File: $( $item.FullName )" | Out-File -FilePath $LogPath -Append
                        Reset-Database($item.Path)
                        continue
                    }
                }
                # Delete everything else
                Write-Output "Deleting: $($item.FullName)" | Out-File -FilePath $LogPath -Append
                Remove-Item -Path $item.FullName -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    } else {
        Write-Output "Removing all user data..." | Out-File -FilePath $LogPath -Append
        Remove-Item -Path "C:\Users\$YourPlaceUser\YourPlace" -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Output "YourPlace has been uninstalled." | Out-File -FilePath $LogPath -Append
}

Main $args # Call the main function to execute the script