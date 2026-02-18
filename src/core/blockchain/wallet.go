package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"sync"
)

type Post struct {
	To         string
	From       string
	Blockchain string
	Status     string // Caching Status - backfilling, complete
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
	if blockchain == "algorand" {
		_, avatar := AlgorandResolveNFD(address)
		return avatar, nil
	}
	if blockchain == "base" {
		avatar, err := _blockchain.Base.GetENSAvatar(address)
		if err != nil {
			return "", err
		}
		return avatar, nil
	}
	return "", nil
}
func WalletGetBalance(blockchain string, address string, _blockchain *Blockchain) (float64, error) {
	if blockchain == "algorand" {
		balance := _blockchain.Algorand.GetBalance(address)
		return float64(balance), nil
	}
	if blockchain == "base" {
		balance, err := _blockchain.Base.GetBalance(address)
		if err != nil {
			return 0, err
		}
		return float64(balance.Uint64()), nil
	}
	return 0, core.LogErrorReturn("Could not get balance of address")
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	core.LogDebug("WalletGetName(): Getting name for address: " + address + " on blockchain: " + blockchain)
	if blockchain == "algorand" {
		name, _ := AlgorandResolveNFD(address)
		return name, nil
	}
	if blockchain == "base" {
		name, err := _blockchain.Base.GetENSName(address)
		if err == nil || name != "" {
			return name, nil
		}
	}
	return "", nil
}
func WalletGetPriceUSD(blockchain string, _blockchain *Blockchain) (float64, error) {
	if blockchain == "algorand" {
		return _blockchain.Algorand.GetPriceUSD(), nil
	}
	if blockchain == "base" {
		return _blockchain.Base.GetPriceUSD(), nil
	}
	return 0, core.LogErrorReturn("Could not get price for blockchain: " + blockchain)
}
func WalletResolveIdentities(database *db.Database, _blockchain *Blockchain) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		BaseResolveIdentities(_blockchain.Base, database)
	}()
	go func() {
		defer wg.Done()
		AlgorandResolveIdentities(database)
	}()
	wg.Wait()
}
