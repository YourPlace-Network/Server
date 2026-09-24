package routes

import (
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
)

func getConfiguredIPFSGateway(database *db.Database) string {
	if host.IsGatewayMode() {
		if gateway := security.SanitizeHostname(host.GetEnvVar("YOURPLACE_IPFS_GATEWAY")); gateway != "" {
			return gateway
		}
	}
	ipfsGateway := database.SettingsGetValue("ipfsGateway")
	if ipfsGateway == "" {
		return network.GetDefaultIPFSGateway()
	}
	return ipfsGateway
}
