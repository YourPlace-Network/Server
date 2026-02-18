package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"context"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
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
func AlgorandResolveIdentities(database *db.Database) {
	addresses := database.ProfileGetAddressesWithMissingEnsData("algorand")
	if len(addresses) == 0 {
		return
	}
	core.LogDebug("Resolving NFD names for " + strconv.Itoa(len(addresses)) + " Algorand addresses")
	for _, address := range addresses {
		name, avatar := AlgorandResolveNFD(address)
		if name != "" || avatar != "" {
			database.ProfileUpdateEnsData(address, "algorand", name, avatar)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
func AlgorandResolveNFD(address string) (string, string) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", "https://api.nf.domains/nfd/lookup?address="+address+"&view=brief", nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}
	var result map[string]struct {
		Name       string `json:"name"`
		Properties struct {
			Verified struct {
				Avatar string `json:"avatar"`
			} `json:"verified"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", ""
	}
	for _, nfd := range result {
		return nfd.Name, nfd.Properties.Verified.Avatar
	}
	return "", ""
}
