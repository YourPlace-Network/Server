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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewBuffer(body))
	//core.LogDebug("Base RPC Request: " + req.URL.String() + " - " + string(body))
	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		//core.LogDebug("Base RPC Response Error: " + err.Error())
		return nil, err
	}
	respbody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(respbody))
	//core.LogDebug("Base RPC Response: " + resp.Status + " - " + string(respbody))
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
	core.LogDebug("Base RPC URL: " + rpcUrl)
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
func BaseGetEarliestBlock() big.Int {
	return *big.NewInt(int64(39000000)) // YourPlace did not exist on-chain before this block
}
func BaseGetBalance(address string, database *db.Database) (big.Int, error) {
	core.LogDebug("BaseGetBalance(): Getting Base balance for address: " + address)
	base := new(Base)
	base.Init(database)
	_addr := common.HexToAddress(address)
	var result hexutil.Big
	err := base.RpcClient.Call(&result, "eth_getBalance", _addr, "latest")
	if err != nil {
		return *big.NewInt(0), core.LogWarningReturn(err.Error())
	}
	//etherBalance := WeiToEther(*result.ToInt())
	return *result.ToInt(), nil
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
	commonAddress := common.HexToAddress(address)
	name, err := ens.ReverseResolve(base.EthClient, commonAddress)
	if err != nil {
		core.LogDebug("Base.GetENSName(): No ENS name found for address: " + address + " - " + err.Error())
		return "", err
	}
	core.LogDebug("Base ENS name for address " + address + " is " + name)
	return name, nil
}
func (base *Base) GetENSAvatar(address string) (string, error) {
	name, err := base.GetENSName(address)
	if err != nil || name == "" {
		return "", err
	}
	resolverAddr := common.HexToAddress(base.EnsResolverAddress)
	resolver, err := ens.NewResolverAt(base.EthClient, name, resolverAddr)
	if err != nil {
		return "", err
	}
	avatar, err := resolver.Text("avatar")
	if err != nil {
		return "", err
	}
	return avatar, nil
}
