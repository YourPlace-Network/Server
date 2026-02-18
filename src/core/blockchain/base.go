package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/services"
	"bytes"
	"context"
	"io"
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

type Base struct {
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
type loggingTransport struct {
	transport http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(body))
	}
	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		respbody, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(respbody))
	}
	return resp, nil
}

func (base *Base) Init(database *db.Database) {
	base.init(database, false)
}
func (base *Base) init(database *db.Database, gateway bool) {
	if base.RpcClient != nil {
		base.RpcClient.Close()
	}
	base.MainnetChainId = 8453
	base.Name = "Base"
	base.Currency = "ETH"
	base.ExplorerUrl = "https://etherscan.io"
	base.EnsResolverAddress = "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD"
	base.ReverseRegistrarAddress = "0x79EA96012eEa67A83431F1701B3dFf7e37F9E282"
	baseURL := os.Getenv("BASE_RPC_URL")
	baseThrottle := os.Getenv("BASE_RPC_THROTTLE")
	if gateway && baseURL != "" {
		base.RpcUrl = baseURL
		database.SettingsUpdateValue("baseURL", base.RpcUrl)
		if baseThrottle != "" {
			database.SettingsUpdateValue("baseThrottle", baseThrottle)
		}
	} else {
		base.RpcUrl = database.SettingsGetValue("baseURL")
		if base.RpcUrl == "" {
			if baseURL != "" {
				base.RpcUrl = baseURL
			} else {
				base.RpcUrl = DefaultBlockchainNodes["base"][0]
			}
			database.SettingsUpdateValue("baseURL", base.RpcUrl)
			database.SettingsUpdateValue("baseThrottle", DefaultBlockchainNodes["base"][1])
		}
	}
	rpcUrl := ResolveRPCUrl(base.RpcUrl)
	core.LogDebug("Base indexer initialized with URL: " + rpcUrl)
	httpClient := &http.Client{
		Transport: &loggingTransport{http.DefaultTransport},
	}
	rpcClient, err := rpc.DialOptions(context.Background(), rpcUrl, rpc.WithHTTPClient(httpClient))
	if err != nil {
		core.LogError("Could not connect to Base HTTPS RPC: " + err.Error())
		return
	}
	defer rpcClient.Close()
	base.RpcClient = rpcClient
	base.EarliestBlock = BaseGetEarliestBlock() // Earliest block where YourPlace existed on-chain
	ethClient := ethclient.NewClient(base.RpcClient)
	defer ethClient.Close()
	base.EthClient = ethClient
}
func (base *Base) GetBalance(address string) (big.Int, error) {
	core.LogDebug("base.GetBalance(): Getting Base balance for address: " + address)
	_addr := common.HexToAddress(address)
	var result hexutil.Big
	err := base.RpcClient.Call(&result, "eth_getBalance", _addr, "latest")
	if err != nil {
		return big.Int{}, core.LogWarningReturn(err.Error())
	}
	return big.Int(result), nil
}
func (base *Base) GetBlockNumber() (*big.Int, error) {
	var result hexutil.Big
	rpcError := 0
	if base.RpcClient == nil {
		return &big.Int{}, core.LogErrorReturn("Base RPC Client is nil")
	}
	for {
		err := base.RpcClient.CallContext(context.Background(), &result, "eth_blockNumber")
		if err != nil {
			core.LogDebug("GetBlockNumber(): Error getting Base block number: " + err.Error())
			rpcError++
			time.Sleep(1 * time.Second)
			if rpcError >= 20 {
				return &big.Int{}, core.LogWarningReturn("Could not get current Base block number. To many errors on RPC call")
			}
		}
		break
	}
	return result.ToInt(), nil
}
func (base *Base) GetPriceUSD() float64 {
	price, _ := services.CoinbaseGetPriceUSD("ETH")
	return price
}
func (base *Base) GetENSAddresses(name string) ([]string, error) {
	resolverAddr := common.HexToAddress(base.EnsResolverAddress)
	resolver, err := ens.NewResolverAt(base.EthClient, name, resolverAddr)
	if err != nil {
		return nil, core.LogErrorReturn("Could not get Base ENS resolver: " + err.Error())
	}
	address, err := resolver.Address()
	if err != nil {
		return nil, core.LogWarningReturn("Could not get Base ENS address: " + err.Error())
	}
	return []string{address.Hex()}, nil
}
func (base *Base) GetBlockTimestamp(blockNumber *big.Int) uint64 {
	blockNumberHex := "0x" + blockNumber.Text(16)
	var result map[string]interface{}
	err := base.RpcClient.Call(&result, "eth_getBlockByNumber", blockNumberHex, false)
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
func (base *Base) GetENSName(address string) (string, error) {
	core.LogDebug("Base.GetENSName(): Getting ENS name for address: " + address)
	if base.RpcClient == nil {
		return "", core.LogErrorReturn("Base.GetENSName(): RPC client is nil")
	}
	node := baseComputeReverseNode(base, address)
	if node == ([32]byte{}) {
		return "", nil
	}
	resolverAddr := baseGetDefaultResolver(base)
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
	err := base.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("Base.GetENSName(): Error calling name() for address " + address + ": " + err.Error())
		return "", err
	}
	name := baseDecodeABIString(result)
	if name != "" {
		core.LogDebug("Base.GetENSName(): Resolved " + address + " -> " + name)
	}
	return name, nil
}
func (base *Base) GetENSAvatar(address string) (string, error) {
	return base.GetENSText(address, "avatar")
}
func (base *Base) GetENSText(address string, key string) (string, error) {
	core.LogDebug("Base.GetENSText(): Getting text record '" + key + "' for address: " + address)
	if base.RpcClient == nil {
		return "", core.LogErrorReturn("Base.GetENSText(): RPC client is nil")
	}
	name, err := base.GetENSName(address)
	if err != nil || name == "" {
		core.LogDebug("Base.GetENSText(): Could not resolve ENS name for address " + address + ": " + err.Error())
		return "", err
	}
	node := baseNameHash(name)
	resolverAddr := baseGetDefaultResolver(base)
	if resolverAddr == (common.Address{}) {
		core.LogDebug("Base.GetENSText(): Could not get default resolver for address " + address)
		return "", nil
	}
	text := baseResolveText(base, resolverAddr, node, key)
	return text, nil
}

/* Public Functions */
func BaseGetEarliestBlock() big.Int {
	return *big.NewInt(int64(39000000)) // YourPlace did not exist on-chain before this block
}
func BaseResolveIdentities(base *Base, database *db.Database) {
	addresses := database.ProfileGetAddressesWithMissingEnsData("base")
	if len(addresses) == 0 {
		return
	}
	for _, address := range addresses {
		name, _ := base.GetENSName(address)
		if name == "" {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		avatar, _ := base.GetENSAvatar(address)
		database.ProfileUpdateEnsData(address, "base", name, avatar)
		time.Sleep(500 * time.Millisecond)
	}
}

/* Internal Functions */
func baseComputeReverseNode(base *Base, address string) [32]byte {
	methodID := crypto.Keccak256([]byte("node(address)"))[:4]
	addr := common.HexToAddress(address)
	callData := make([]byte, 36)
	copy(callData[:4], methodID)
	copy(callData[16:36], addr.Bytes())
	registrarAddr := common.HexToAddress(base.ReverseRegistrarAddress)
	msg := map[string]interface{}{
		"to":   registrarAddr.Hex(),
		"data": hexutil.Encode(callData),
	}
	var result hexutil.Bytes
	err := base.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("baseComputeReverseNode(): Error calling node() for " + address + ": " + err.Error())
		return [32]byte{}
	}
	if len(result) < 32 {
		core.LogDebug("baseComputeReverseNode(): Unexpected result length " + strconv.Itoa(len(result)) + " for " + address)
		return [32]byte{}
	}
	var node [32]byte
	copy(node[:], result[:32])
	return node
}
func baseDecodeABIString(data []byte) string {
	if len(data) < 64 {
		core.LogDebug("baseDecodeABIString(): Data length less than 64 bytes")
		return ""
	}
	offset := new(big.Int).SetBytes(data[:32]).Uint64()
	if offset+32 > uint64(len(data)) {
		core.LogDebug("baseDecodeABIString(): Offset out of bounds")
		return ""
	}
	length := new(big.Int).SetBytes(data[offset : offset+32]).Uint64()
	if offset+32+length > uint64(len(data)) {
		core.LogDebug("baseDecodeABIString(): Length out of bounds")
		return ""
	}
	return string(data[offset+32 : offset+32+length])
}
func baseGetDefaultResolver(base *Base) common.Address {
	methodID := crypto.Keccak256([]byte("defaultResolver()"))[:4]
	registrarAddr := common.HexToAddress(base.ReverseRegistrarAddress)
	msg := map[string]interface{}{
		"to":   registrarAddr.Hex(),
		"data": hexutil.Encode(methodID),
	}
	var result hexutil.Bytes
	err := base.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("baseGetDefaultResolver(): Error querying registrar: " + err.Error())
		return common.Address{}
	}
	if len(result) < 32 {
		core.LogDebug("baseGetDefaultResolver(): Result too short")
		return common.Address{}
	}
	resolverAddr := common.BytesToAddress(result[12:32])
	return resolverAddr
}
func baseNameHash(name string) [32]byte {
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
func baseResolveText(base *Base, resolverAddr common.Address, node [32]byte, key string) string {
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
	err := base.RpcClient.CallContext(context.Background(), &result, "eth_call", msg, "latest")
	if err != nil {
		core.LogDebug("baseResolveText(): Error calling text() for key " + key + ": " + err.Error())
		return ""
	}
	return baseDecodeABIString(result)
}
