#!/bin/bash

# Uninstall script for YourPlace on macOS. Lives in "/Library/Application\ Support/YourPlace/uninstall.sh"

KEEP_UPLOAD=0
KEEP_BLOCKCHAIN=0

# Parse command line arguments
for arg in "$@"; do
    case $arg in
        keepUpload)
            KEEP_UPLOAD=1
            ;;
        keepBlockchain)
            KEEP_BLOCKCHAIN=1
            ;;
        data)
            # Legacy support for keeping all data
            KEEP_UPLOAD=1
            KEEP_BLOCKCHAIN=1
            ;;
    esac
done

CONSOLE_USER=$(stat -f "%Su" /dev/console)
CONSOLE_UID=$(id -u "$CONSOLE_USER")
USER_ID=$(id -u)

# Remove the LaunchAgents
launchctl bootout "gui/$USER_ID/com.yourplace.server" 2>/dev/null || true
rm -f "/Users/$CONSOLE_USER/Library/LaunchAgents/com.yourplace.server.plist"

# Remove the Helper LaunchDaemon (requires root)
sudo launchctl bootout "system/com.yourplace.helper" 2>/dev/null || true
sudo rm -f /Library/LaunchDaemons/com.yourplace.helper.plist

# Remove the uninstall LaunchDaemon (requires root)
sudo launchctl bootout "system/com.yourplace.uninstall" 2>/dev/null || true
sudo rm -f /Library/LaunchDaemons/com.yourplace.uninstall.plist

# Kill any running processes
sudo pkill -f YourPlaceHelper 2>/dev/null || true
pkill -f YourPlace 2>/dev/null || true
pkill -f YourPlaceIpfs 2>/dev/null || true
pkill -f YourPlaceFfmpeg 2>/dev/null || true
sleep 2

# Remove the application & artifacts
sudo rm -rf "/Applications/YourPlace.app"
sudo rm -rf "/Library/Logs/YourPlace"
sudo rm -rf "/Users/$CONSOLE_USER/Library/Caches/YourPlace"
sudo rm -rf "/Users/$CONSOLE_USER/Library/Logs/YourPlace"
sudo rm -rf "/tmp/YourPlaceHelper.sock"

# Handle user data based on flags
if [ $KEEP_UPLOAD -eq 1 ] || [ $KEEP_BLOCKCHAIN -eq 1 ]; then
    cd "/Users/$CONSOLE_USER/YourPlace/" 2>/dev/null
    if [ $? -eq 0 ]; then
        find . -mindepth 1 -maxdepth 1 | while read -r item; do
            base=$(basename "$item")
            if [ $KEEP_UPLOAD -eq 1 ] && [ "$base" == "uploads" ]; then
                continue
            elif [ $KEEP_BLOCKCHAIN -eq 1 ] && [ "$base" == "yourplace.sqlite.db" ]; then
                continue
            else
                rm -rf "$item"
            fi
        done
        rm -rf .ipfs
    fi
else
    rm -rf "/Users/$CONSOLE_USER/YourPlace/"
fi