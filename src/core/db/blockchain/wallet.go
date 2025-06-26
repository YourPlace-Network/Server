package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"time"
)

type Post struct {
	To         string
	From       string
	Blockchain string
	Status     string // Caching Status - backfilling, complete
}

// Typed caches for better performance (optional alternative to generic wallet cache)
var (
	balanceCache = db.NewCache[float64]("balance", 1*time.Hour, 60*time.Second)
	nameCache    = db.NewCache[string]("names", 1*time.Hour, 60*time.Second)
	addressCache = db.NewCache[string]("addresses", 1*time.Hour, 60*time.Second)
)

// Wallet Interaction Functions
func GetBalance(blockchain string, address string, database *db.Database) (float64, error) {
	cacheKey := blockchain + ":" + address
	return balanceCache.ExecuteWithCache(cacheKey, func() (float64, error) {
		if blockchain == "base" {
			balance, err := BaseGetBalance(address, database)
			if err != nil {
				return 0, err
			}
			return float64(balance.Uint64()), nil
		}
		return 0, core.LogErrorReturn("Could not get balance of address")
	})
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	cacheKey := blockchain + ":" + address
	return nameCache.ExecuteWithCache(cacheKey, func() (string, error) {
		if blockchain == "base" {
			names, err := _blockchain.Base.GetENSNames(address)
			if err != nil {
				return "", err
			}
			if len(names) > 0 {
				return names[0], nil
			}
		}
		return "", nil
	})
}
func WalletGetAddress(blockchain string, name string, _blockchain *Blockchain) (string, error) {
	cacheKey := blockchain + ":" + name
	return addressCache.ExecuteWithCache(cacheKey, func() (string, error) {
		if blockchain == "base" {
			addresses, err := _blockchain.Base.GetENSAddresses(name)
			if err != nil {
				return "", core.LogErrorReturn("Could not get address from ENS name: " + err.Error())
			}
			if len(addresses) > 0 {
				return addresses[0], nil
			}
		}
		return "", nil
	})
}
