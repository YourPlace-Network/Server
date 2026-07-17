package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/services"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	ens "github.com/wealdtech/go-ens/v3"
)

type WalletData struct {
	Address    string
	Blockchain string
	PrivateKey string
	PublicKey  string
}
type Ethereum struct {
	MainnetChainId          uint
	Name                    string
	Currency                string
	ExplorerUrl             string
	RpcUrl                  string
	RpcClient               *rpc.Client
	EarliestBlock           big.Int
	EthClient               *ethclient.Client
	EnsResolverAddress      string
	ReverseRegistrarAddress string
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

func (ethereum *Ethereum) Init(database *db.Database) {
	ethereum.init(database, false)
}
func (ethereum *Ethereum) init(database *db.Database, gateway bool) {
	if ethereum.RpcClient != nil {
		ethereum.RpcClient.Close()
	}
	ethereum.MainnetChainId = 1
	ethereum.Name = "Ethereum"
	ethereum.Currency = "ETH"
	ethereum.ExplorerUrl = "https://etherscan.io"
	ethereum.EnsResolverAddress = "0x231b0Ee14048e9dCcD1d247744d114a4EB5E8E63"
	ethereum.ReverseRegistrarAddress = "0xa58E81fe9b61B5c3fE2AFD33CF304c454AbFc7Cb"
	ethereumURL := os.Getenv("ETHEREUM_RPC_URL")
	ethereumThrottle := os.Getenv("ETHEREUM_RPC_THROTTLE")
	if gateway && ethereumURL != "" {
		ethereum.RpcUrl = ethereumURL
		database.SettingsUpdateValue("ethereumURL", ethereum.RpcUrl)
		if ethereumThrottle != "" {
			database.SettingsUpdateValue("ethereumThrottle", ethereumThrottle)
		}
	} else {
		ethereum.RpcUrl = database.SettingsGetValue("ethereumURL")
		if ethereum.RpcUrl == "" {
			if ethereumURL != "" {
				ethereum.RpcUrl = ethereumURL
			} else {
				ethereum.RpcUrl = DefaultBlockchainNodes["ethereum"][0]
			}
			database.SettingsUpdateValue("ethereumURL", ethereum.RpcUrl)
			database.SettingsUpdateValue("ethereumThrottle", DefaultBlockchainNodes["ethereum"][1])
		}
	}
	rpcUrl := ResolveRPCUrl(ethereum.RpcUrl)
	core.LogDebug("Ethereum indexer initialized with URL: " + rpcUrl)
	httpClient := &http.Client{
		Transport: &loggingTransport{http.DefaultTransport},
		Timeout:   blockchainRPCTimeout,
	}
	rpcClient, err := rpc.DialOptions(context.Background(), rpcUrl, rpc.WithHTTPClient(httpClient))
	if err != nil {
		core.LogError("Could not connect to Ethereum HTTPS RPC: " + err.Error())
		return
	}
	ethereum.RpcClient = rpcClient
	ethereum.EarliestBlock = EthereumGetEarliestBlock()
	ethClient := ethclient.NewClient(ethereum.RpcClient)
	ethereum.EthClient = ethClient
}
func (ethereum *Ethereum) GetBalance(address string) (big.Int, error) {
	core.LogDebug("ethereum.GetBalance(): Getting Ethereum balance for address: " + address)
	_addr := common.HexToAddress(address)
	var result hexutil.Big
	err := ethereum.RpcClient.Call(&result, "eth_getBalance", _addr, "latest")
	if err != nil {
		return big.Int{}, core.LogWarningReturn(err.Error())
	}
	return big.Int(result), nil
}
func (ethereum *Ethereum) GetBlockNumber() (*big.Int, error) {
	var result hexutil.Big
	rpcError := 0
	if ethereum.RpcClient == nil {
		return &big.Int{}, core.LogErrorReturn("Ethereum RPC Client is nil")
	}
	for {
		rpcContext, cancel := context.WithTimeout(context.Background(), blockchainRPCTimeout)
		err := ethereum.RpcClient.CallContext(rpcContext, &result, "eth_blockNumber")
		cancel()
		if err != nil {
			core.LogDebug("GetBlockNumber(): Error getting Ethereum block number: " + err.Error())
			rpcError++
			time.Sleep(1 * time.Second)
			if rpcError >= 20 {
				ethereum.reconnectRPC()
				return &big.Int{}, core.LogWarningReturn("Could not get current Ethereum block number. Too many errors on RPC call")
			}
		} else {
			break
		}
	}
	return result.ToInt(), nil
}
func (ethereum *Ethereum) GetBlockTimestamp(blockNumber *big.Int) uint64 {
	blockNumberHex := "0x" + blockNumber.Text(16)
	var result map[string]interface{}
	err := ethereum.RpcClient.Call(&result, "eth_getBlockByNumber", blockNumberHex, false)
	if err != nil || result == nil {
		return 0
	}
	timestampHex, ok := result["timestamp"].(string)
	if !ok || len(timestampHex) < 3 {
		return 0
	}
	timestamp, _ := strconv.ParseUint(timestampHex[2:], 16, 64)
	return timestamp
}
func (ethereum *Ethereum) GetPriceUSD() float64 {
	price, _ := services.CoinbaseGetPriceUSD("ETH")
	return price
}
func (ethereum *Ethereum) GetENSAddresses(name string) ([]string, error) {
	core.LogDebug("Ethereum.GetENSAddresses(): Resolving ENS name: " + name)
	if ethereum.EthClient == nil {
		return nil, core.LogErrorReturn("Ethereum.GetENSAddresses(): EthClient is nil")
	}
	resolverAddr := common.HexToAddress(ethereum.EnsResolverAddress)
	core.LogDebug("Ethereum.GetENSAddresses(): Using resolver: " + resolverAddr.Hex())
	resolver, err := ens.NewResolverAt(ethereum.EthClient, name, resolverAddr)
	if err != nil {
		return nil, core.LogErrorReturn("Could not get Ethereum ENS resolver: " + err.Error())
	}
	address, err := resolver.Address()
	if err != nil {
		return nil, core.LogWarningReturn("Could not get Ethereum ENS address: " + err.Error())
	}
	if address != (common.Address{}) {
		core.LogDebug("Ethereum.GetENSAddresses(): Resolved " + name + " -> " + address.Hex())
		return []string{address.Hex()}, nil
	}
	core.LogDebug("Ethereum.GetENSAddresses(): " + name + " has no linked address, looking up owner")
	registry, err := ens.NewRegistry(ethereum.EthClient)
	if err != nil {
		return nil, core.LogErrorReturn("Ethereum.GetENSAddresses(): Could not get ENS registry: " + err.Error())
	}
	owner, err := registry.Owner(name)
	if err != nil {
		return nil, core.LogWarningReturn("Ethereum.GetENSAddresses(): Could not get owner for " + name + ": " + err.Error())
	}
	if owner == (common.Address{}) {
		core.LogDebug("Ethereum.GetENSAddresses(): " + name + " has no owner, treating as not found")
		return nil, nil
	}
	core.LogDebug("Ethereum.GetENSAddresses(): Resolved " + name + " -> " + owner.Hex() + " (owner)")
	return []string{owner.Hex()}, nil
}
func (ethereum *Ethereum) GetENSName(address string) (string, error) {
	core.LogDebug("Ethereum.GetENSName(): Getting ENS name for address: " + address)
	if ethereum.RpcClient == nil {
		return "", core.LogErrorReturn("Ethereum.GetENSName(): RPC client is nil")
	}
	node := ethereumComputeReverseNode(ethereum, address)
	if node == ([32]byte{}) {
		return "", nil
	}
	resolverAddr := ethereumGetDefaultResolver(ethereum)
	if resolverAddr == (common.Address{}) {
		return "", nil
	}
	methodID := crypto.Keccak256([]byte("name(bytes32)"))[:4]
	callData := make([]byte, 36)
	copy(callData[:4], methodID)
	copy(callData[4:], node[:])
	msg := map[string]interface{}{
		"to":   resolverAddr.Hex(),
		"data": hexutil.Encode(callData),
	}
	var result hexutil.Bytes
	err := ethereum.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("Ethereum.GetENSName(): Error calling name() for address " + address + ": " + err.Error())
		return "", err
	}
	name := ethereumDecodeABIString(result)
	if name != "" {
		core.LogDebug("Ethereum.GetENSName(): Resolved " + address + " -> " + name)
	}
	return name, nil
}
func (ethereum *Ethereum) GetENSAvatar(address string) (string, error) {
	return ethereum.GetENSText(address, "avatar")
}
func (ethereum *Ethereum) GetENSText(address string, key string) (string, error) {
	core.LogDebug("Ethereum.GetENSText(): Getting text record '" + key + "' for address: " + address)
	if ethereum.RpcClient == nil {
		return "", core.LogErrorReturn("Ethereum.GetENSText(): RPC client is nil")
	}
	name, err := ethereum.GetENSName(address)
	if err != nil {
		core.LogDebug("Ethereum.GetENSText(): Could not resolve ENS name for address " + address + ": " + err.Error())
		return "", err
	}
	if name == "" {
		return "", nil
	}
	node := ethereumNameHash(name)
	resolverAddr := ethereumGetDefaultResolver(ethereum)
	if resolverAddr == (common.Address{}) {
		core.LogDebug("Ethereum.GetENSText(): Could not get default resolver for address " + address)
		return "", nil
	}
	text := ethereumResolveText(ethereum, resolverAddr, node, key)
	return text, nil
}
func (ethereum *Ethereum) reconnectRPC() {
	core.LogDebug("[Ethereum] Reconnecting RPC client")
	if ethereum.RpcClient != nil {
		ethereum.RpcClient.Close()
	}
	rpcUrl := ResolveRPCUrl(ethereum.RpcUrl)
	httpClient := &http.Client{
		Transport: &loggingTransport{http.DefaultTransport},
		Timeout:   blockchainRPCTimeout,
	}
	rpcClient, err := rpc.DialOptions(context.Background(), rpcUrl, rpc.WithHTTPClient(httpClient))
	if err != nil {
		core.LogDebug("[Ethereum] reconnectRPC(): Could not reconnect: " + err.Error())
		return
	}
	ethereum.RpcClient = rpcClient
	ethereum.EthClient = ethclient.NewClient(ethereum.RpcClient)
}

/* Public Functions */
func EthereumGetEarliestBlock() big.Int {
	return *big.NewInt(int64(24527000))
}
func EthereumResolveIdentities(ethereum *Ethereum, database *db.Database) {
	addresses := database.ProfileGetAddressesWithMissingEnsData("ethereum")
	if len(addresses) == 0 {
		return
	}
	for _, address := range addresses {
		name, _ := ethereum.GetENSName(address)
		avatar, _ := ethereum.GetENSAvatar(address)
		database.ProfileUpdateEnsData(address, "ethereum", name, avatar)
		time.Sleep(500 * time.Millisecond)
	}
}

/* Internal Functions */
func ethereumComputeReverseNode(ethereum *Ethereum, address string) [32]byte {
	methodID := crypto.Keccak256([]byte("node(address)"))[:4]
	addr := common.HexToAddress(address)
	callData := make([]byte, 36)
	copy(callData[:4], methodID)
	copy(callData[16:36], addr.Bytes())
	registrarAddr := common.HexToAddress(ethereum.ReverseRegistrarAddress)
	msg := map[string]interface{}{
		"to":   registrarAddr.Hex(),
		"data": hexutil.Encode(callData),
	}
	var result hexutil.Bytes
	err := ethereum.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("ethereumComputeReverseNode(): Error calling node() for " + address + ": " + err.Error())
		return [32]byte{}
	}
	if len(result) < 32 {
		core.LogDebug("ethereumComputeReverseNode(): Unexpected result length " + strconv.Itoa(len(result)) + " for " + address)
		return [32]byte{}
	}
	var node [32]byte
	copy(node[:], result[:32])
	return node
}
func ethereumDecodeABIString(data []byte) string {
	if len(data) < 64 {
		core.LogDebug("ethereumDecodeABIString(): Data length less than 64 bytes")
		return ""
	}
	offset := new(big.Int).SetBytes(data[:32]).Uint64()
	if offset+32 > uint64(len(data)) {
		core.LogDebug("ethereumDecodeABIString(): Offset out of bounds")
		return ""
	}
	length := new(big.Int).SetBytes(data[offset : offset+32]).Uint64()
	if offset+32+length > uint64(len(data)) {
		core.LogDebug("ethereumDecodeABIString(): Length out of bounds")
		return ""
	}
	return string(data[offset+32 : offset+32+length])
}
func ethereumGetDefaultResolver(ethereum *Ethereum) common.Address {
	methodID := crypto.Keccak256([]byte("defaultResolver()"))[:4]
	registrarAddr := common.HexToAddress(ethereum.ReverseRegistrarAddress)
	msg := map[string]interface{}{
		"to":   registrarAddr.Hex(),
		"data": hexutil.Encode(methodID),
	}
	var result hexutil.Bytes
	err := ethereum.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("ethereumGetDefaultResolver(): Error querying registrar: " + err.Error())
		return common.Address{}
	}
	if len(result) < 32 {
		core.LogDebug("ethereumGetDefaultResolver(): Result too short")
		return common.Address{}
	}
	resolverAddr := common.BytesToAddress(result[12:32])
	return resolverAddr
}
func ethereumNameHash(name string) [32]byte {
	var node [32]byte
	if name == "" {
		return node
	}
	labels := strings.Split(name, ".")
	for i := len(labels) - 1; i >= 0; i-- {
		labelHash := crypto.Keccak256([]byte(labels[i]))
		combined := make([]byte, 64)
		copy(combined[:32], node[:])
		copy(combined[32:], labelHash)
		copy(node[:], crypto.Keccak256(combined))
	}
	return node
}
func ethereumResolveText(ethereum *Ethereum, resolverAddr common.Address, node [32]byte, key string) string {
	methodID := crypto.Keccak256([]byte("text(bytes32,string)"))[:4]
	keyBytes := []byte(key)
	keyPadded := make([]byte, ((len(keyBytes)+31)/32)*32)
	copy(keyPadded, keyBytes)
	callData := make([]byte, 0, 4+32+32+32+len(keyPadded))
	callData = append(callData, methodID...)
	callData = append(callData, node[:]...)
	offset := make([]byte, 32)
	offset[31] = 0x40
	callData = append(callData, offset...)
	length := make([]byte, 32)
	length[31] = byte(len(keyBytes))
	callData = append(callData, length...)
	callData = append(callData, keyPadded...)
	msg := map[string]interface{}{
		"to":   resolverAddr.Hex(),
		"data": hexutil.Encode(callData),
	}
	var result hexutil.Bytes
	err := ethereum.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("ethereumResolveText(): Error calling text() for key " + key + ": " + err.Error())
		return ""
	}
	return ethereumDecodeABIString(result)
}
