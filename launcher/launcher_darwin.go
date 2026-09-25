//go:build darwin

package launcher

import (
	"YourPlace/src/core"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func HandleCommand(protocol, domain string, port int) bool {
	if len(os.Args) != 2 || os.Args[1] != "-open-ui" {
		return false
	}
	if err := OpenLauncherUI(protocol, domain, port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return true
}
func Start(gateway bool) {
	if gateway {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	suffix := filepath.FromSlash("/Contents/Helpers/YourPlaceServer.app/Contents/MacOS/YourPlace")
	if !strings.HasSuffix(executable, suffix) {
		return
	}
	launcher := filepath.Join(strings.TrimSuffix(executable, suffix), "Contents", "MacOS", "YourPlaceLauncher")
	go func() {
		if err := exec.Command(launcher, "--background").Run(); err != nil {
			core.LogDebug("Could not start Dock launcher: " + err.Error())
		}
	}()
}
