package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/network"
)

func getConfiguredIPFSGateway(database *db.Database) string {
	ipfsGateway := database.SettingsGetValue("ipfsGateway")
	if ipfsGateway == "" {
		return network.GetDefaultIPFSGateway()
	}
	return ipfsGateway
}
