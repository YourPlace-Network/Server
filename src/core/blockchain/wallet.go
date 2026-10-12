package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"

	algotypes "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/ethereum/go-ethereum/common"
)

type Post struct {
	To         string
	From       string
	Blockchain string
	Status     string // Caching Status - backfilling, complete
}

var rpcDedup = core.NewDedupeQueue()

type nftChain interface {
	Build(context.Context, db.NFTGrant, []byte) (*db.NFTTransaction, error)
	Broadcast(context.Context, db.NFTChainConfig, *db.NFTTransaction) error
	Credential(string) ([]byte, string, error)
	EstimateMintCost(context.Context, db.NFTChainConfig) (float64, error)
	Reconcile(context.Context, db.NFTGrant) (string, string, error)
	Source(context.Context, db.NFTSource, string) (string, error)
	Validate(context.Context, db.NFTChainConfig) error
	ValidateRegistration(context.Context, db.NFTChainConfig) error
}
type nftRecipientChain interface {
	Prepare(context.Context, db.NFTGrant) (NFTAcceptanceResponse, []byte, error)
	VerifyAcceptance(context.Context, db.NFTGrant, []byte) error
}

func WalletNFTAssetLabels(name string) (string, string) {
	assetName, unit := "", ""
	for _, char := range name {
		if len(assetName)+len(string(char)) > algotypes.AssetNameMaxLen {
			break
		}
		assetName += string(char)
	}
	for _, char := range strings.ToUpper(name) {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			unit += string(char)
			if len(unit) == algotypes.AssetUnitNameMaxLen {
				break
			}
		}
	}
	return assetName, unit
}
func WalletNFTIdentity(network, address string) (string, error) {
	var identity []byte
	switch network {
	case "base", "ethereum":
		if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) {
			return "", errors.New("invalid wallet")
		}
		identity = common.HexToAddress(address).Bytes()
	case "algorand":
		decoded, err := algotypes.DecodeAddress(address)
		if err != nil || decoded == (algotypes.Address{}) {
			return "", errors.New("invalid wallet")
		}
		identity = decoded[:]
	default:
		return "", errors.New("unsupported wallet")
	}
	hash := sha256.Sum256(append([]byte(network+":"), identity...))
	return hex.EncodeToString(hash[:]), nil
}
func WalletValidateNFTRegistration(registration *db.NFTRegistration) error {
	if !walletValidNFTAddress(registration.OperatorNetwork, registration.OperatorAddress) {
		return errors.New("invalid_operator_wallet")
	}
	if len(registration.Chains) > 3 {
		return errors.New("too_many_chains")
	}
	seen := map[string]bool{}
	for index, config := range registration.Chains {
		identity, err := WalletNFTIdentity(config.Network, config.Sender)
		if err != nil || !walletValidNFTAddress(config.Network, config.Sender) || seen[config.Network] {
			return errors.New("invalid_or_duplicate_sender")
		}
		seen[config.Network] = true
		switch config.Network {
		case "base", "ethereum":
			chainID := nftMainnetChainID(config.Network)
			if config.ChainID != "" && config.ChainID != chainID {
				return errors.New("invalid_chain_id")
			}
			config.ChainID = chainID
			if !walletValidNFTAddress(config.Network, config.Contract) {
				return errors.New("invalid_collection")
			}
			hash, err := hex.DecodeString(strings.TrimPrefix(config.CodeHash, "0x"))
			if err != nil || len(config.CodeHash) != 66 || len(hash) != 32 || !strings.HasPrefix(config.CodeHash, "0x") || common.BytesToHash(hash) == (common.Hash{}) {
				return errors.New("invalid_reviewed_code_hash")
			}
			config.CodeHash = "0x" + hex.EncodeToString(hash)
		case "algorand":
			hash, err := base64.StdEncoding.Strict().DecodeString(config.ChainID)
			if err != nil || len(config.ChainID) != 44 || len(hash) != 32 || base64.StdEncoding.EncodeToString(hash) != config.ChainID || common.BytesToHash(hash) == (common.Hash{}) || config.Contract != "" || config.CodeHash != "" {
				return errors.New("invalid_algorand_registration")
			}
		}
		registration.Chains[index] = db.NFTChainConfig{Network: config.Network, ChainID: config.ChainID, Sender: config.Sender, SignerIdentity: identity, Contract: config.Contract, CodeHash: config.CodeHash}
	}
	return nil
}
func walletValidNFTAddress(network, address string) bool {
	if network != "base" && network != "ethereum" && network != "algorand" {
		return false
	}
	if !security.IsValidAddress(address, network) {
		return false
	}
	if _, err := WalletNFTIdentity(network, address); err != nil {
		return false
	}
	if network == "algorand" {
		decoded, err := algotypes.DecodeAddress(address)
		return err == nil && decoded.String() == address
	}
	hexAddress := address[2:]
	return hexAddress == strings.ToLower(hexAddress) || hexAddress == strings.ToUpper(hexAddress) || common.HexToAddress(address).Hex() == address
}
func (blockchain *Blockchain) ValidateSessionAuthority(ctx context.Context, network, address, authorizer string) (bool, error) {
	switch network {
	case "base", "ethereum":
		return true, nil
	case "algorand":
		if authorizer == "" {
			return false, nil
		}
		current, err := blockchain.Algorand.LoginAuthorizer(ctx, address)
		return err == nil && current == authorizer, err
	default:
		return false, nil
	}
}
func nftMainnetChainID(network string) string {
	switch network {
	case "base":
		return strconv.Itoa(baseMainnetChainID)
	case "ethereum":
		return strconv.Itoa(ethereumMainnetChainID)
	default:
		return ""
	}
}
func (blockchain *Blockchain) nftChain(network string) (nftChain, error) {
	switch network {
	case "base":
		if blockchain.Base != nil && blockchain.Base.EthClient != nil {
			return &evmNFT{client: blockchain.Base.EthClient, extraFee: blockchain.Base.nftExtraFee}, nil
		}
	case "ethereum":
		if blockchain.Ethereum != nil && blockchain.Ethereum.EthClient != nil {
			return &evmNFT{client: blockchain.Ethereum.EthClient}, nil
		}
	case "algorand":
		if blockchain.Algorand != nil && blockchain.Algorand.algodClient != nil {
			return &algoNFT{client: blockchain.Algorand.algodClient}, nil
		}
	}
	return nil, errors.New("chain unavailable")
}

func ShortWalletAddress(address string) string {
	if len(address) <= 12 {
		return address
	}
	return address[:6] + "..." + address[len(address)-4:]
}

func WalletGetAddress(blockchain string, name string, _blockchain *Blockchain) (string, error) {
	key := blockchain + ":address:" + name
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
		if blockchain == "algorand" {
			address := _blockchain.Algorand.ResolveNFDName(name)
			return address, nil
		}
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
		if blockchain == "ethereum" {
			addresses, err := _blockchain.Ethereum.GetENSAddresses(name)
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
			_, avatar := _blockchain.Algorand.ResolveNFD(address)
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
		if blockchain == "ethereum" {
			avatar, err := _blockchain.Ethereum.GetENSAvatar(address)
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
		if blockchain == "ethereum" {
			balance, err := _blockchain.Ethereum.GetBalance(address)
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
func WalletGetBalanceFormatted(blockchain string, address string, _blockchain *Blockchain) (float64, string) {
	balanceRaw, _ := WalletGetBalance(blockchain, address, _blockchain)
	if blockchain == "algorand" {
		return balanceRaw / 1e6, "ALGO"
	}
	if blockchain == "base" {
		return balanceRaw / 1e18, "ETH"
	}
	if blockchain == "ethereum" {
		return balanceRaw / 1e18, "ETH"
	}
	return balanceRaw, ""
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	key := blockchain + ":name:" + address
	val, _ := rpcDedup.Do(key, func() (interface{}, error) {
		core.LogDebug("WalletGetName(): Getting name for address: " + address + " on blockchain: " + blockchain)
		if blockchain == "algorand" {
			name, _ := _blockchain.Algorand.ResolveNFD(address)
			return name, nil
		}
		if blockchain == "base" {
			name, err := _blockchain.Base.GetENSName(address)
			if err == nil || name != "" {
				return name, nil
			}
		}
		if blockchain == "ethereum" {
			name, err := _blockchain.Ethereum.GetENSName(address)
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
		if blockchain == "ethereum" {
			return _blockchain.Ethereum.GetPriceUSD(), nil
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
	wg.Add(3)
	go func() {
		defer wg.Done()
		AlgorandResolveIdentities(_blockchain.Algorand, database)
	}()
	go func() {
		defer wg.Done()
		BaseResolveIdentities(_blockchain.Base, database)
	}()
	go func() {
		defer wg.Done()
		EthereumResolveIdentities(_blockchain.Ethereum, database)
	}()
	wg.Wait()
}
