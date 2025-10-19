//go:build !gateway

package main

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"runtime"
	"strconv"

	"github.com/getlantern/systray"
)

func runSystray(database *db.Database) {
	systray.Run(func() { SystrayOnReady(database) }, func() { host.Shutdown(0) })
}

func SystrayOnReady(database *db.Database) {
	systray.SetTemplateIcon(favicon, favicon)
	if runtime.GOOS == "windows" {
		systray.SetIcon(favicon)
		systray.SetTitle("YourPlace")
	}
	systray.SetTooltip("YourPlace Server")
	mUI := systray.AddMenuItem("Open YourPlace", "Open YourPlace in your browser")
	mUI.SetIcon(favicon)
	mSettings := systray.AddMenuItem("Settings", "YourPlace Settings")
	mQuit := systray.AddMenuItem("Quit", "Quit YourPlace Server")
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				host.Shutdown(0)
			case <-mUI.ClickedCh:
				host.OpenBrowser(protocol + "://" + domain + ":" + strconv.Itoa(port))
			case <-mSettings.ClickedCh:
				host.OpenBrowser(protocol + "://" + domain + ":" + strconv.Itoa(port) + "/settings")
			}
		}
	}()
}
