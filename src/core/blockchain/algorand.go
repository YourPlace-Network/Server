package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/encoding/msgpack"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"
)

type Algorand struct {
	algodClient   *algod.Client
	mnemonic      string
	network       string
	algodHost     *url.URL
	algodToken    string
	algodPort     int
	walletAddress types.Address
}

/*var DefaultMainnetServers = struct {
	Algod   map[int]string
	Indexer map[int]string
	Header  map[int]string
}{
	Algod: map[int]string{
		//0: "https://algod.yourplace.network",
		0: "https://mainnet-algorand.api.purestake.io/ps2",
	},
	Indexer: map[int]string{
		//0: "https://indexer.yourplace.network",
		0: "https://mainnet-algorand.api.purestake.io/idx2",
	},
}*/

func (algo *Algorand) Init(algodHost string, algodToken string, algodPort int, algodNetwork string) {
	if !security.IsValidAlgoNetwork(algodNetwork) {
		log.Panicln("Invalid Algorand network configuration")
	}
	algo.network = algodNetwork
	algodURL, err := url.Parse(algodHost)
	if err != nil {
		log.Panicln("Invalid Algod host URL")
	}
	algo.algodHost = algodURL
	core.LogDebug("Algorand indexer initialized with URL: " + algodHost)
	if !security.IsValidPort(algodPort) {
		log.Panicln("Invalid Algod Port")
	}
	algo.algodPort = algodPort
	if !security.IsValidAlgodToken(algodToken) {
		if algodToken != "" {
			log.Panicln("Invalid Algod Token format")
		}
	}
	algo.algodToken = algodToken
	var algodClient *algod.Client
	if strings.Contains(algodURL.Host, "purestake.io") {
		var commonClient, err = common.MakeClient(algodHost, "X-API-Key", algodToken)
		if err != nil {
			log.Panicf("Can't create algod client: %s\n", err)
		}
		algodClient = (*algod.Client)(commonClient)
	} else {
		algodClient, err = algod.MakeClient(algodHost, algodToken)
		if err != nil {
			log.Panicf("Can't create algod client: %s\n", err)
		}
	}
	algo.algodClient = algodClient
}
func (algo *Algorand) CreateTransaction(toAddr string, fromAddr string, amount uint64, message string) types.Transaction {
	txParams, err := algo.algodClient.SuggestedParams().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	txn, err := transaction.MakePaymentTxn(fromAddr, toAddr, transaction.MinTxnFee, []byte(message), "", txParams)
	return txn
}
func (algo *Algorand) SubmitTransaction(transaction string) bool {
	response, err := algo.algodClient.SendRawTransaction([]byte(transaction)).Do(context.Background())
	fmt.Println(response)
	if err != nil {
		return false
	}
	return true
}
func (algo *Algorand) CreateProtectedWallet(password string) {
	// https://developer.algorand.org/docs/get-details/accounts/create/#wallet-derived-kmd
	if algo.doesWalletExist() {
		//LogError("Wallet file exists already") debug
		return
	}
	account := crypto.GenerateAccount()
	seedPhrase, err := mnemonic.FromPrivateKey(account.PrivateKey)
	if err != nil {
		//core.LogFail("Could not create wallet seed phrase: " + err.Error()) debug
	}
	encryptedMnemonic, err := security.EncryptString(password, seedPhrase)
	if err != nil {
		//core.LogFail("Could not encrypt wallet seed phrase: " + err.Error()) debug
	}
	err = ioutil.WriteFile(host.GetDataDir()+"wallet.yp", []byte(encryptedMnemonic), 0440)
	if err != nil {
		//core.LogFail("Could not create wallet file: " + err.Error()) debug
	}
}
func (algo *Algorand) CreateWallet() {
	if algo.doesWalletExist() {
		//core.LogError("Wallet file exists already")
		return
	}
	account := crypto.GenerateAccount()
	seedPhrase, err := mnemonic.FromPrivateKey(account.PrivateKey)
	if err != nil {
		//core.LogFail("Could not create wallet seed phrase: " + err.Error())
		return
	}
	err = ioutil.WriteFile(host.GetDataDir()+"wallet.yp", []byte(seedPhrase), 0440)
	if err != nil {
		//core.LogFail("Could not create wallet file: " + err.Error())
	}
}
func (algo *Algorand) doesWalletExist() bool {
	walletLocation := host.GetInstallDir() + "wallet.yp"
	if _, err := os.Stat(walletLocation); os.IsNotExist(err) {
		return false
	}
	return true
}
func (algo *Algorand) RawVerifyTransaction(pubkey ed25519.PublicKey, transaction types.Transaction, sig []byte) bool {
	// https://github.com/algorand/go-algorand-sdk/blob/develop/crypto/account.go#L120
	core.LogInfo("Transaction note: " + string(transaction.Note))
	core.LogInfo("Pubkey Length: " + strconv.Itoa(len(pubkey)))
	if (ed25519.PublicKey{}.Equal(pubkey)) { // pubkey should not be empty
		core.LogError("Invalid public key")
		return false
	}
	core.LogInfo("Pubkey: " + string(pubkey))
	core.LogInfo("Signature Length: " + strconv.Itoa(len(sig)))
	core.LogInfo("Signature: " + string(sig))
	encodedTxn := msgpack.Encode(transaction)
	core.LogInfo("Encoded transaction: " + string(encodedTxn))
	var transaction2 types.Transaction
	err := msgpack.Decode(encodedTxn, &transaction2)
	if err != nil {
		core.LogError("Could not decode transaction: " + err.Error())
		return false
	}
	kp := crypto.GenerateAccount()
	pk := ed25519.PublicKey(kp.Address[:])
	message := []byte("Hello World")
	sig2 := ed25519.Sign(kp.PrivateKey, message)
	ret2 := ed25519.Verify(pk, message, sig2)
	if ret2 {
		core.LogInfo("Test Transaction Verified")
	}

	ret := ed25519.Verify(pubkey, encodedTxn, sig)
	if ret {
		core.LogInfo("Transaction verified")
		return true
	}
	return false
}

// ----- Profile ----- //
func (algo *Algorand) ProfileGetName(address string) string {

	return ""
}

// ----- Getters ----- //
func (algo *Algorand) GetAlgodURL() *url.URL {
	return algo.algodHost
}
func (algo *Algorand) GetAlgodToken() string {
	return algo.algodToken
}
func (algo *Algorand) GetAlgodPort() int {
	return algo.algodPort
}
func (algo *Algorand) GetBlockNumber() (*big.Int, error) {
	info, err := algo.algodClient.Status().Do(context.Background())
	if err != nil {
		return big.NewInt(0), err
	}
	return big.NewInt(int64(info.LastRound)), nil
}
func (algo *Algorand) GetSeedPhrase(password string) string {
	cipherText, err := os.ReadFile(host.GetInstallDir() + "wallet.yp")
	if err != nil {
		//core.LogFail("Could not read the wallet file: " + err.Error())
	}
	seedPhrase, err := security.DecryptString(password, string(cipherText))
	if err != nil {
		//core.LogFail("Could not decrypt the wallet file: " + err.Error())
	}
	return seedPhrase
}
func (algo *Algorand) GetBalance(address string) uint64 {
	result, err := algo.algodClient.AccountInformation(address).Do(context.Background())
	if err != nil {
		return 0
	}
	return result.Amount
}
func (algo *Algorand) GetNetwork() string {
	return algo.network
}
func (algo *Algorand) GetPubKey(address string) (ed25519.PublicKey, error) {
	checksumLenBytes := 4
	if !security.IsValidAlgoAddress(address) {
		return nil, errors.New("not a valid algo address")
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(address)
	if err != nil {
		return nil, errors.New("could not decode algo address")
	}
	if len(decoded) != len(types.Address{})+checksumLenBytes {
		return nil, errors.New("decoded algo address wrong length")
	}
	addressBytes := decoded[:len(types.Address{})]
	return addressBytes, nil
}
func (algo *Algorand) getPrivKey(password string) (ed25519.PrivateKey, error) {
	privkey, err := mnemonic.ToPrivateKey(algo.GetSeedPhrase(password))
	return privkey, err
}
func (algo *Algorand) GetWalletAddress() types.Address {
	return algo.walletAddress
}
func (algo *Algorand) GetTransactionNonce(host string) string {
	return ""
}
func (algo *Algorand) GetProfileMetaTxn(avatarURL string, name string) types.Transaction {
	txParams, err := algo.algodClient.SuggestedParams().Do(context.Background())
	if err != nil {
		log.Fatalln("Can't get algod suggested params: " + err.Error())
	}
	address := algo.GetWalletAddress()
	note := []byte("yp/1/m:j{\"mn\":\"" + name + "\",\"ma\":\"" + avatarURL + "\"}")
	txn, err := transaction.MakePaymentTxn(address.String(), address.String(), uint64(0), note, "", txParams)
	return txn
}
func (algo *Algorand) GetPriceUSD() float64 {
	price, _ := services.CoinbaseGetPriceUSD("ALGO")
	return price
}

// ----- Setters ----- //
func (algo *Algorand) SetWalletAddress(address types.Address) {
	algo.walletAddress = address
}

// ----- Identity Resolution ----- //
func AlgorandResolveIdentities(algo *Algorand, database *db.Database) {
	addresses := database.ProfileGetAddressesWithMissingEnsData("algorand")
	if len(addresses) == 0 {
		return
	}
	core.LogDebug("Resolving NFD names for " + strconv.Itoa(len(addresses)) + " Algorand addresses")
	for _, address := range addresses {
		name, avatar := algo.ResolveNFD(address)
		database.ProfileUpdateEnsData(address, "algorand", name, avatar)
		time.Sleep(500 * time.Millisecond)
	}
}
func (algo *Algorand) ResolveNFD(address string) (string, string) {
	if !security.IsValidAlgoAddress(address) {
		return "", ""
	}
	ctx := context.Background()
	registryAppID := nfdRegistryAppID(algo.network)
	addr, err := types.DecodeAddress(address)
	if err != nil {
		return "", ""
	}
	boxKey := nfdGetRegistryBoxNameForAddress(addr)
	boxValue, err := algo.algodClient.GetApplicationBoxByName(registryAppID, boxKey).Do(ctx)
	if err == nil && len(boxValue.Value) >= 8 {
		for offset := 0; offset+8 <= len(boxValue.Value); offset += 8 {
			nfdAppID := binary.BigEndian.Uint64(boxValue.Value[offset : offset+8])
			if nfdAppID == 0 {
				continue
			}
			name, _, avatar := nfdReadAppState(algo.algodClient, nfdAppID)
			if name != "" {
				return name, avatar
			}
		}
	}
	lsig, err := getNFDSigRevAddressLSIG(addr, registryAppID)
	if err != nil {
		return "", ""
	}
	lsigAddr, err := lsig.Address()
	if err != nil {
		return "", ""
	}
	acctInfo, err := algo.algodClient.AccountApplicationInformation(lsigAddr.String(), registryAppID).Do(ctx)
	if err != nil {
		return "", ""
	}
	for idx := 0; idx < 16; idx++ {
		targetKey := "i.apps" + strconv.Itoa(idx)
		found := false
		for _, kv := range acctInfo.AppLocalState.KeyValue {
			keyBytes, err := base64.StdEncoding.DecodeString(kv.Key)
			if err != nil || string(keyBytes) != targetKey {
				continue
			}
			found = true
			if kv.Value.Type != 1 {
				break
			}
			valBytes, err := base64.StdEncoding.DecodeString(kv.Value.Bytes)
			if err != nil || len(valBytes) < 8 {
				break
			}
			for off := 0; off+8 <= len(valBytes); off += 8 {
				nfdAppID := binary.BigEndian.Uint64(valBytes[off : off+8])
				if nfdAppID == 0 {
					continue
				}
				name, _, avatar := nfdReadAppState(algo.algodClient, nfdAppID)
				if name != "" {
					return name, avatar
				}
			}
			break
		}
		if !found {
			break
		}
	}
	return "", ""
}
func (algo *Algorand) ResolveNFDName(nfdName string) string {
	if !security.IsValidNFDomain(nfdName) {
		return ""
	}
	ctx := context.Background()
	registryAppID := nfdRegistryAppID(algo.network)
	boxKey := nfdGetRegistryBoxNameForNFD(nfdName)
	boxValue, err := algo.algodClient.GetApplicationBoxByName(registryAppID, boxKey).Do(ctx)
	if err == nil && len(boxValue.Value) >= 16 {
		nfdAppID := binary.BigEndian.Uint64(boxValue.Value[8:16])
		if nfdAppID != 0 {
			_, owner, _ := nfdReadAppState(algo.algodClient, nfdAppID)
			if owner != "" {
				return owner
			}
		}
	}
	lsig, err := getNFDSigNameLSIG(nfdName, registryAppID)
	if err != nil {
		return ""
	}
	lsigAddr, err := lsig.Address()
	if err != nil {
		return ""
	}
	acctInfo, err := algo.algodClient.AccountApplicationInformation(lsigAddr.String(), registryAppID).Do(ctx)
	if err != nil {
		return ""
	}
	for _, kv := range acctInfo.AppLocalState.KeyValue {
		keyBytes, err := base64.StdEncoding.DecodeString(kv.Key)
		if err != nil || string(keyBytes) != "i.appid" {
			continue
		}
		var nfdAppID uint64
		if kv.Value.Type == 1 {
			valBytes, err := base64.StdEncoding.DecodeString(kv.Value.Bytes)
			if err == nil && len(valBytes) == 8 {
				nfdAppID = binary.BigEndian.Uint64(valBytes)
			}
		} else if kv.Value.Type == 2 {
			nfdAppID = kv.Value.Uint
		}
		if nfdAppID != 0 {
			_, owner, _ := nfdReadAppState(algo.algodClient, nfdAppID)
			if owner != "" {
				return owner
			}
		}
		break
	}
	return ""
}
func getNFDLookupLSIG(prefixBytes string, lookupBytes string, registryAppID uint64) (crypto.LogicSigAccount, error) {
	sigLookupByteCode := []byte{
		0x05, 0x20, 0x01, 0x01, 0x80, 0x08, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06,
		0x07, 0x08, 0x17, 0x35, 0x00, 0x31, 0x18, 0x34, 0x00, 0x12, 0x31, 0x10,
		0x81, 0x06, 0x12, 0x10, 0x31, 0x19, 0x22, 0x12, 0x31, 0x19, 0x81, 0x00,
		0x12, 0x11, 0x10, 0x40, 0x00, 0x01, 0x00, 0x22, 0x43, 0x26, 0x01,
	}
	binary.BigEndian.PutUint64(sigLookupByteCode[6:14], registryAppID)
	bytesToAppend := bytes.Join([][]byte{[]byte(prefixBytes), []byte(lookupBytes)}, nil)
	uvarIntBytes := make([]byte, binary.MaxVarintLen64)
	nBytes := binary.PutUvarint(uvarIntBytes, uint64(len(bytesToAppend)))
	composedBytecode := bytes.Join([][]byte{sigLookupByteCode, uvarIntBytes[:nBytes], bytesToAppend}, nil)
	return crypto.MakeLogicSigAccountEscrowChecked(composedBytecode, [][]byte{})
}
func getNFDSigNameLSIG(nfdName string, registryAppID uint64) (crypto.LogicSigAccount, error) {
	return getNFDLookupLSIG("name/", nfdName, registryAppID)
}
func getNFDSigRevAddressLSIG(address types.Address, registryAppID uint64) (crypto.LogicSigAccount, error) {
	return getNFDLookupLSIG("address/", address.String(), registryAppID)
}
func nfdGetRegistryBoxNameForAddress(address types.Address) []byte {
	hash := sha256.Sum256(bytes.Join([][]byte{[]byte("addr/algo/"), address[:]}, nil))
	return hash[:]
}
func nfdGetRegistryBoxNameForNFD(name string) []byte {
	hash := sha256.Sum256([]byte("name/" + name))
	return hash[:]
}
func nfdReadAppState(algodClient *algod.Client, appID uint64) (string, string, string) {
	ctx := context.Background()
	app, err := algodClient.GetApplicationByID(appID).Do(ctx)
	if err != nil {
		core.LogDebug("nfdReadAppState: failed to get app " + strconv.FormatUint(appID, 10) + ": " + err.Error())
		return "", "", ""
	}
	var name, owner, verifiedOwner, avatar string
	for _, kv := range app.Params.GlobalState {
		keyBytes, err := base64.StdEncoding.DecodeString(kv.Key)
		if err != nil {
			continue
		}
		key := string(keyBytes)
		switch {
		case key == "i.name" && kv.Value.Type == 1:
			valBytes, err := base64.StdEncoding.DecodeString(kv.Value.Bytes)
			if err == nil {
				name = string(valBytes)
			}
		case key == "i.owner.a" && kv.Value.Type == 1:
			valBytes, err := base64.StdEncoding.DecodeString(kv.Value.Bytes)
			if err == nil && len(valBytes) == 32 {
				var addr types.Address
				copy(addr[:], valBytes)
				owner = addr.String()
			}
		case (key == "v.avatar" || key == "u.avatar") && kv.Value.Type == 1 && avatar == "":
			valBytes, err := base64.StdEncoding.DecodeString(kv.Value.Bytes)
			if err == nil && len(valBytes) > 0 {
				avatar = string(valBytes)
			}
		case key == "v.caAlgo.0.as" && kv.Value.Type == 1:
			valBytes, err := base64.StdEncoding.DecodeString(kv.Value.Bytes)
			if err == nil && len(valBytes) >= 32 {
				var addr types.Address
				copy(addr[:], valBytes[:32])
				verifiedOwner = addr.String()
			}
		}
	}
	if avatar == "" {
		for _, boxName := range []string{"v.avatar", "u.avatar"} {
			boxValue, err := algodClient.GetApplicationBoxByName(appID, []byte(boxName)).Do(ctx)
			if err == nil && len(boxValue.Value) > 0 {
				avatar = string(boxValue.Value)
				break
			}
		}
	}
	boxValue, err := algodClient.GetApplicationBoxByName(appID, []byte("v.caAlgo.0.as")).Do(ctx)
	if err == nil && len(boxValue.Value) >= 32 {
		var addr types.Address
		copy(addr[:], boxValue.Value[:32])
		verifiedOwner = addr.String()
	}
	if verifiedOwner != "" {
		owner = verifiedOwner
	}
	return name, owner, avatar
}
func nfdRegistryAppID(network string) uint64 {
	if network == "testnet" {
		return 84366825
	}
	return 760937186
}
