//go:build gateway

package main

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
)

func runSystray(database *db.Database) {
	core.LogDebug("Running in gateway mode - systray disabled")
	select {} // Block forever
}
