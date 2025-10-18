#!/bin/bash
set -e

# Ensure the YourPlace data directory exists
mkdir -p /root/YourPlace

# Start the gateway service
# The YourPlace binary will handle IPFS installation and initialization
exec /app/YourPlaceGateway -g -d -du
