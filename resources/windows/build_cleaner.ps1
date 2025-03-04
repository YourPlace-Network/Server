Get-Process -Name 'YourPlace' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process -Name 'YourPlaceHelper' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process -Name 'YourPlaceIpfs' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process -Name 'YourPlaceFfmpeg' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
if (Test-Path 'target') { Remove-Item -Recurse -Force 'target' }
if (Test-Path 'src\core\host\bin\helper\win\YourPlaceHelper.exe') { Remove-Item -Force 'src\core\host\bin\helper\win\YourPlaceHelper.exe' }
if (Test-Path 'rsrc_windows_amd64.syso') { Remove-Item -Force 'rsrc_windows_amd64.syso' }