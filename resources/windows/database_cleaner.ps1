# Get command line arguments
$DatabasePath = $args[0]
$SqlCommand = $args[1]
# Check if arguments were provided
if (-not $DatabasePath -or -not $SqlCommand) {
    Write-Error "Usage: .\database_cleaner.ps1 <DatabasePath> <SqlCommand>"
    exit 1
}
# Check if the database file exists
if (-not (Test-Path $DatabasePath)) {
    Write-Error "Database file not found: $DatabasePath"
    exit 1
}
try {
    # Load the SQLite assembly
    Add-Type -AssemblyName "System.Data.SQLite"
    # Create a connection to the SQLite database
    $connectionString = "Data Source=$DatabasePath;Version=3;"
    $connection = New-Object System.Data.SQLite.SQLiteConnection($connectionString)
    $connection.Open()
    # Create a command to execute the SQL statement
    $command = $connection.CreateCommand()
    $command.CommandText = $SqlCommand
}