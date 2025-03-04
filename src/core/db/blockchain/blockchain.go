package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"math/big"
)

type Blockchain struct {
	Base     *Base
	Algorand *Algorand
}

var DefaultBlockchainNodes = map[string][]string{
	// blockchain: {rpcURL, rateLimit}
	"base": {"https://mainnet.base.org", "10"},
}

func (blockchain *Blockchain) Init(database *db.Database) {
	// --- Algorand --- //
	algo := new(Algorand) // Create Algod & Indexer instance
	algoURL := database.SettingsGetValue("algodURL")
	algoToken := database.SettingsGetValue("algodToken")
	algoIndexerURL := database.SettingsGetValue("algoIndexerURL")
	algoIndexerToken := database.SettingsGetValue("algoIndexerToken")
	algo.Init(algoURL, algoToken, 443, "mainnet", algoIndexerURL, algoIndexerToken, 443)
	blockchain.Algorand = algo

	// --- Base --- //
	base := new(Base)
	base.Init(database)
	blockchain.Base = base
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
	case "base":
		return blockchain.Base.GetBlockNumber()
	case "algorand":
		return blockchain.Algorand.GetBlockNumber()
	default:
		return big.NewInt(0), core.LogErrorReturn("Invalid chain: " + chain)
	}
}
func (blockchain *Blockchain) GetEarliestBlock(chain string) *big.Int {
	switch chain {
	case "base":
		return &blockchain.Base.EarliestBlock
	default:
		return big.NewInt(0)
	}
}
