#Requires -RunAsAdministrator

# Enable nested virtualization if on a VM
Set-VMProcessor -VMName YourPlace -ExposeVirtualizationExtensions $true

# Set-ItemProperty -Path 'HKLM:\Software\Microsoft\Windows\CurrentVersion\RunOnce' -Name '' -Value `powershell.exe -File "$PSCommandPath"`
Enable-WindowsOptionalFeature -Online -FeatureName "Containers-DisposableClientVM" -All

Start-Process -FilePath ""