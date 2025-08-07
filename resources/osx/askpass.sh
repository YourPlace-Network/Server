#!/usr/bin/env bash

SCRIPT_NAME=$(basename "$0")
PARENT_CMD=$(ps -o comm= -p $PPID | xargs basename 2>/dev/null || echo "unknown")
REASON="${1:-Administrative access requred}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

osascript -e "Tell application \"System Events\" to display dialog \"Sudo Password Required\n\nScript: $SCRIPT_NAME\nCalled by: $PARENT_CMD\nDirectory: $(pwd)\nReason: $REASON\" default answer \"\" with hidden answer with icon POSIX file \"$SCRIPT_DIR/AppIcon.icns\"" -e 'text returned of result'
