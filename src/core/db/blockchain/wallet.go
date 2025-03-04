package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
)

type Post struct {
	To         string
	From       string
	Blockchain string
	Status     string // Caching Status - backfilling, complete
}

func GetBalance(blockchain string, address string, database *db.Database) (float64, error) {
	if blockchain == "base" {
		balance, _ := BaseGetBalance(address, database)
		return float64(balance.Uint64()), nil
	}
	return 0, core.LogErrorReturn("Could not get balance of address")
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	if blockchain == "base" {
		names, _ := _blockchain.Base.GetENSNames(address)
		if len(names) > 0 {
			return names[0], nil
		}
	}
	return "", nil
}
func WalletGetAddress(blockchain string, name string, _blockchain *Blockchain) (string, error) {
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
}
