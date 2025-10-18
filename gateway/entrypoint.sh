#!/bin/bash
set -e

# Write SSH public key from environment variable to authorized_keys
if [ -n "$SSH_PUBLIC_KEY" ]; then
    echo "$SSH_PUBLIC_KEY" > /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
    echo "SSH public key configured"
else
    echo "Warning: SSH_PUBLIC_KEY environment variable not set"
fi

# Execute the command passed to the container
exec "$@"
