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

var rpcDedup = core.NewDedupeQueue()

func WalletGetAddress(blockchain string, name string, _blockchain *Blockchain) (string, error) {
	key := blockchain + ":address:" + name
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
		if blockchain == "base" {
			addresses, err := _blockchain.Base.GetENSAddresses(name)
			if err != nil {
				core.LogDebug("WalletGetAddress: " + err.Error())
				return "", nil
			}
			if len(addresses) > 0 {
				return addresses[0], nil
			}
		}
		return "", nil
	})
	if val == nil {
		return "", nil
	}
	return val.(string), nil
}
func WalletGetAvatar(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	key := blockchain + ":avatar:" + address
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
		if blockchain == "algorand" {
			_, avatar := AlgorandResolveNFD(address)
			return avatar, nil
		}
		if blockchain == "base" {
			avatar, err := _blockchain.Base.GetENSAvatar(address)
			if err != nil {
				core.LogDebug("WalletGetAvatar: " + err.Error())
				return "", nil
			}
			return avatar, nil
		}
		return "", nil
	})
	if val == nil {
		return "", nil
	}
	return val.(string), nil
}
func WalletGetBalance(blockchain string, address string, _blockchain *Blockchain) (float64, error) {
	key := blockchain + ":balance:" + address
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
		if blockchain == "algorand" {
			balance := _blockchain.Algorand.GetBalance(address)
			return float64(balance), nil
		}
		if blockchain == "base" {
			balance, err := _blockchain.Base.GetBalance(address)
			if err != nil {
				core.LogDebug("WalletGetBalance: " + err.Error())
				return float64(0), nil
			}
			return float64(balance.Uint64()), nil
		}
		core.LogDebug("WalletGetBalance: unsupported blockchain: " + blockchain)
		return float64(0), nil
	})
	if val == nil {
		return 0, nil
	}
	return val.(float64), nil
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	key := blockchain + ":name:" + address
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
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
	})
	if val == nil {
		return "", nil
	}
	return val.(string), nil
}
func WalletGetPriceUSD(blockchain string, _blockchain *Blockchain) (float64, error) {
	key := blockchain + ":priceUSD"
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
		if blockchain == "algorand" {
			return _blockchain.Algorand.GetPriceUSD(), nil
		}
		if blockchain == "base" {
			return _blockchain.Base.GetPriceUSD(), nil
		}
		core.LogDebug("WalletGetPriceUSD: unsupported blockchain: " + blockchain)
		return float64(0), nil
	})
	if val == nil {
		return 0, nil
	}
	return val.(float64), nil
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
