package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"math/big"
	"os"
	"strings"
	"time"
)

type Blockchain struct {
	Algorand *Algorand
	Base     *Base
	Ethereum *Ethereum
}

const blockchainRPCTimeout = 30 * time.Second

var DefaultBlockchainNodes = map[string][]string{
	"algorand": {"https://mainnet-api.algonode.cloud", "60"},
	"base":     {"/rpc/base", "5"},
	"ethereum": {"/rpc/ethereum", "5"},
}

func (blockchain *Blockchain) Init(database *db.Database) {
	blockchain.init(database, false)
}
func (blockchain *Blockchain) InitGateway(database *db.Database) {
	blockchain.init(database, true)
}
func (blockchain *Blockchain) init(database *db.Database, gateway bool) {
	algo := new(Algorand)
	algoURLEnv := os.Getenv("ALGO_RPC_URL")
	algoThrottleEnv := os.Getenv("ALGO_RPC_THROTTLE")
	var algoURL string
	if gateway && algoURLEnv != "" {
		algoURL = algoURLEnv
		database.SettingsUpdateValue("algoURL", algoURL)
		if algoThrottleEnv != "" {
			database.SettingsUpdateValue("algoThrottle", algoThrottleEnv)
		}
	} else {
		algoURL = database.SettingsGetValue("algoURL")
		if algoURL == "" {
			if algoURLEnv != "" {
				algoURL = algoURLEnv
			} else {
				algoURL = DefaultBlockchainNodes["algorand"][0]
			}
			database.SettingsUpdateValue("algoURL", algoURL)
			database.SettingsUpdateValue("algoThrottle", DefaultBlockchainNodes["algorand"][1])
		}
	}
	algoToken := database.SettingsGetValue("algodToken")
	algo.Init(algoURL, algoToken, 443, "mainnet")
	blockchain.Algorand = algo
	base := new(Base)
	base.init(database, gateway)
	blockchain.Base = base
	ethereum := new(Ethereum)
	ethereum.init(database, gateway)
	blockchain.Ethereum = ethereum
	//blockchain.StartupIndexerCleanup(database)                      // Reset any indexer backfill jobs that were left hanging on startup
	if database.SettingsGetValue("baseFullNode") == "true" { // Install Geth + Base if configured to
		/*gethInstalled := host.InstallGethNode()
		if gethInstalled {
			baseDataDirectory := database.GetKeyValue("baseDataDirectory", "settings")
			host.RunGethNode(port+3, baseDataDirectory)
		}*/
		go host.InstallRunBaseNode()
	}
}
func (blockchain *Blockchain) GetLatestBlock(chain string) (*big.Int, error) {
	switch chain {
	case "algorand":
		return blockchain.Algorand.GetBlockNumber()
	case "base":
		return blockchain.Base.GetBlockNumber()
	case "ethereum":
		return blockchain.Ethereum.GetBlockNumber()
	default:
		return big.NewInt(0), core.LogErrorReturn("Invalid chain: " + chain)
	}
}
func (blockchain *Blockchain) GetEarliestBlock(chain string) *big.Int {
	switch chain {
	case "algorand":
		earliestBlock := AlgoGetEarliestBlock()
		return &earliestBlock
	case "base":
		return &blockchain.Base.EarliestBlock
	case "ethereum":
		return &blockchain.Ethereum.EarliestBlock
	default:
		return big.NewInt(0)
	}
}
func ResolveRPCUrl(rpcUrl string) string {
	if !strings.HasPrefix(rpcUrl, "/") {
		return rpcUrl
	}
	domain := os.Getenv("YourPlaceDomain")
	port := os.Getenv("YourPlacePort")
	gateway := os.Getenv("YourPlaceGateway") == "true"
	if domain == "" {
		domain = "localhost"
	}
	if port == "" {
		port = "42424"
	}
	protocol := "http"
	if gateway {
		protocol = "https"
	}
	if gateway {
		return protocol + "://" + domain + rpcUrl
	}
	return protocol + "://" + domain + ":" + port + rpcUrl
}
