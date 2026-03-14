#!/bin/bash
set -e

# Start the gateway service
# The YourPlace binary will handle IPFS installation and initialization
exec /app/YourPlaceGateway -g -du
