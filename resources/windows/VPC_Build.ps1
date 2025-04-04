# Create deployment user
$Password = ConvertTo-SecureString "<password>" -AsPlainText -Force
New-LocalUser -name "ypdeploy" -Password $Password -FullName "YP Deployer" -Description "YourPlace Server deployer user" -AccountNeverExpires -PasswordNeverExpires
Add-LocalGroupMember -Group "Users" -Member "ypdeploy"

# Create SSH directory structure
$UserProfile = "C:\Users\ypdeploy"
New-Item -Path "$UserProfile\.ssh" -ItemType Directory -Force

# Generate SSH Keys
ssh-keygen -t ed25519 -f "$UserProfile\.ssh\id_ed25519" -N '""'
Copy-Item "$UserProfile\.ssh\id_ed25519.pub" -Destination "$UserProfile\.ssh\authorized_keys"

# Set permissions for SSH keys
icacls "$UserProfile\.ssh" /inheritance:r
icacls "$UserProfile\.ssh" /grant "sshuser:(OI)(CI)F"
icacls "$UserProfile\.ssh\authorized_keys" /inheritance:r
icacls "$UserProfile\.ssh\authorized_keys" /grant "sshuser:F"

# Enable user-specific authorized_keys
$ConfigFile = "C:\ProgramData\ssh\sshd_config"
$Content = Get-Content -Path $ConfigFile

# Remove comment from PubkeyAuthentication line (if commented)
$Content = $Content -replace '#PubkeyAuthentication yes', 'PubkeyAuthentication yes'

# Ensure proper authorization file configuration
$Content = $Content -replace '#AuthorizedKeysFile(.*)\.ssh/authorized_keys', 'AuthorizedKeysFile$1.ssh/authorized_keys'

# Ensure password authentication is enabled
$Content = $Content -replace '#PasswordAuthentication yes', 'PasswordAuthentication yes'
$Content = $Content -replace 'PasswordAuthentication no', 'PasswordAuthentication yes'

Set-Content -Path $ConfigFile -Value $Content

Restart-Service sshd