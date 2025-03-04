#!/bin/bash

# Specify the path to your binary
BINARY_PATH="/path/to/your/binary"

# Define the sandbox profile
SANDBOX_PROFILE='(version 1)
(allow default)
(deny network*)
(allow file-read-metadata file-read-data
    (literal "/path/to/your/binary")
    (literal "/usr/lib")
    (literal "/System/Library")
)
'

# Create a temporary sandbox profile file
SANDBOX_PROFILE_FILE="$(mktemp)"

# Write the sandbox profile to the file
echo "$SANDBOX_PROFILE" > "$SANDBOX_PROFILE_FILE"

# Run the binary in the sandboxed environment
sandbox-exec -f "$SANDBOX_PROFILE_FILE" "$BINARY_PATH"

# Remove the temporary sandbox profile file
rm "$SANDBOX_PROFILE_FILE"
