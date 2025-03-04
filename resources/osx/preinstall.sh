#!/bin/bash

# Get the current console user
CONSOLE_USER=$(stat -f "%Su" /dev/console)
CONSOLE_UID=$(id -u "$CONSOLE_USER")

# Unload existing LaunchAgents and LaunchDaemons
launchctl bootout "gui/$CONSOLE_UID/com.yourplace.server" 2>/dev/null || true
launchctl bootout system/com.yourplace.helper 2>/dev/null || true
launchctl bootout system/com.yourplace.uninstall 2>/dev/null || true

# Stop running processes
pkill "YourPlace" 2>/dev/null || true
pkill "YourPlaceHelper" 2>/dev/null || true
pkill "YourPlaceIpfs" 2>/dev/null || true

# Give processes time to stop
sleep 5

# Delete old binaries
rm -rf /Applications/YourPlace.app 2>/dev/null || true
rm -rf /Users/$CONSOLE_USER/Library/Caches/YourPlace/* 2>/dev/null || true

# Remove temp files
rm /Users/$CONSOLE_USER/YourPlace/debug 2>/dev/null || true
rm /Users/$CONSOLE_USER/YourPlace/*db-wal 2>/dev/null || true
rm /Users/$CONSOLE_USER/YourPlace/*db-shm 2>/dev/null || true
