#!/usr/bin/env bash

osascript -e 'Tell application "System Events" to display dialog "Sudo Password:" default answer "" with hidden answer' -e 'text returned of result'
