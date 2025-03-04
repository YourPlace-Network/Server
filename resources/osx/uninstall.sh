#!/bin/bash

KEEP_DATA=0
if [ "$1" == "data" ]; then
    KEEP_DATA=1
fi

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
rm -rf "/Users/$CONSOLE_USER/Library/Caches/YourPlace/"*
if [ $KEEP_DATA -eq 1 ]; then # delete all user data except the .db files
  find "/Users/$CONSOLE_USER/YourPlace/" -type f ! -name "*.db" -delete
  find "/Users/$CONSOLE_USER/YourPlace/" -type d -empty -delete
else # delete all user data
  rm -rf "/Users/$CONSOLE_USER/YourPlace/"*
fi