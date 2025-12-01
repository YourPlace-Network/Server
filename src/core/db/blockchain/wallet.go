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

// Wallet Interaction Functions
func GetBalance(blockchain string, address string, database *db.Database) (float64, error) {
	if blockchain == "base" {
		balance, err := BaseGetBalance(address, database)
		if err != nil {
			return 0, err
		}
		return float64(balance.Uint64()), nil
	}
	return 0, core.LogErrorReturn("Could not get balance of address")
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	core.LogInfo("WalletGetName(): Getting ENS name for address: " + address + " on blockchain: " + blockchain)
	if blockchain == "base" {
		name, err := _blockchain.Base.GetENSName(address)
		if err == nil || name != "" {
			return name, nil
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
func WalletGetAvatar(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	if blockchain == "base" {
		avatar, err := _blockchain.Base.GetENSAvatar(address)
		if err != nil {
			return "", err
		}
		return avatar, nil
	}
	return "", nil
}
