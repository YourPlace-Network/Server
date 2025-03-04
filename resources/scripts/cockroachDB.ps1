$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($isAdmin) {
    Write-Host "This script is running as administrator."
} else {
    Write-Host "This script is NOT running as administrator."
    Exit
}

cockroach start --insecure --store=node1 --listen-addr=127.0.0.1:26257 --http-addr=127.0.0.1:8085 --join=127.0.0.1:26257
# cockroach init --insecure --host=localhost:26257