package blockchain

import (
	"YourPlace/src/core"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type WalletData struct {
	PrivateKey string
	PublicKey  string
	Address    string
	Blockchain string
}

func CreateEthereumWallet() (*WalletData, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, core.LogErrorReturn("Could not generate Eth private key: " + err.Error())
	}
	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex := hexutil.Encode(privateKeyBytes)
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, core.LogErrorReturn("error casting public key to ECDSA")
	}
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	publicKeyHex := hexutil.Encode(publicKeyBytes)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	fmt.Println("Wallet created successfully!")
	fmt.Println("Address:", address)
	fmt.Println("Private Key:", privateKeyHex)
	fmt.Println("Public Key:", publicKeyHex)
	return &WalletData{
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Address:    address,
		Blockchain: "Ethereum",
	}, nil
}
