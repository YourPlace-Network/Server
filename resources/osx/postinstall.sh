#!/bin/bash

# Get the current console user
CONSOLE_USER=$(stat -f "%Su" /dev/console)
CONSOLE_UID=$(id -u "$CONSOLE_USER")

# Create persistent location for the uninstall script
mkdir -p /Library/Application\ Support/YourPlace
cp /Applications/YourPlace.app/Contents/Resources/scripts/uninstall.sh /Library/Application\ Support/YourPlace/
chmod 0755 /Library/Application\ Support/YourPlace/uninstall.sh

# Copy and load the LaunchDaemon for uninstaller
cp /Applications/YourPlace.app/Contents/Resources/scripts/com.yourplace.uninstall.plist /Library/LaunchDaemons/
chown root:wheel /Library/LaunchDaemons/com.yourplace.uninstall.plist
chmod 0644 /Library/LaunchDaemons/com.yourplace.uninstall.plist
launchctl bootout system/com.yourplace.uninstall 2>/dev/null || true
launchctl bootstrap system /Library/LaunchDaemons/com.yourplace.uninstall.plist

# Copy and load the LaunchDaemon for helper
cp /Applications/YourPlace.app/Contents/Resources/scripts/com.yourplace.helper.plist /Library/LaunchDaemons/
chown root:wheel /Library/LaunchDaemons/com.yourplace.helper.plist
chmod 0644 /Library/LaunchDaemons/com.yourplace.helper.plist
launchctl bootout system/com.yourplace.helper 2>/dev/null || true
launchctl bootstrap system /Library/LaunchDaemons/com.yourplace.helper.plist

# Copy and load the LaunchAgent for the server
cp /Applications/YourPlace.app/Contents/Resources/scripts/com.yourplace.server.plist "/Users/$CONSOLE_USER/Library/LaunchAgents/"
chown "$CONSOLE_USER:staff" "/Users/$CONSOLE_USER/Library/LaunchAgents/com.yourplace.server.plist"
chmod 0644 "/Users/$CONSOLE_USER/Library/LaunchAgents/com.yourplace.server.plist"
launchctl asuser "$CONSOLE_UID" launchctl bootout "gui/$CONSOLE_UID/com.yourplace.server" 2>/dev/null || true
launchctl asuser "$CONSOLE_UID" launchctl bootstrap "gui/$CONSOLE_UID" "/Users/$CONSOLE_USER/Library/LaunchAgents/com.yourplace.server.plist"
