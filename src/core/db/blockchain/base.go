package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/services"
	"bytes"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	ens "github.com/wealdtech/go-ens/v3"
	"io"
	"math/big"
	"net/http"
	"time"
)

type Base struct {
	MainnetChainId     uint
	Name               string
	Currency           string
	ExplorerUrl        string
	RpcUrl             string
	RpcClient          *rpc.Client
	EarliestBlock      big.Int
	EthClient          *ethclient.Client
	EnsResolverAddress string
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
		core.LogError("Base RPC Response Error: " + err.Error())
		return nil, err
	}
	respbody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(respbody))
	//core.LogDebug("Base RPC Response: " + resp.Status + " - " + string(respbody))
	return resp, nil
}

func (base *Base) Init(database *db.Database) {
	if base.RpcClient != nil { // close the previous connection if it exists
		base.RpcClient.Close()
	}
	base.MainnetChainId = 8453
	base.Name = "Base"
	base.Currency = "ETH"
	base.ExplorerUrl = "https://etherscan.io"
	base.EnsResolverAddress = "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD"
	base.RpcUrl = database.SettingsGetValue("baseURL")
	if base.RpcUrl == "" {
		base.RpcUrl = DefaultBlockchainNodes["base"][0] // fallback to Coinbase public nodes
		database.SettingsUpdateValue("baseURL", base.RpcUrl)
		database.SettingsUpdateValue("baseThrottle", DefaultBlockchainNodes["base"][1])
	}
	httpClient := &http.Client{
		Transport: &loggingTransport{http.DefaultTransport},
	}
	rpcClient, err := rpc.DialOptions(context.Background(), base.RpcUrl, rpc.WithHTTPClient(httpClient))
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
			rpcError++
			time.Sleep(1 * time.Second)
			if rpcError >= 20 {
				return &big.Int{}, core.LogWarningReturn("Could not get current Base block number: " + err.Error())
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
func (base *Base) GetENSNames(address string) ([]string, error) {
	commonAddress := common.HexToAddress(address)
	reverse, err := ens.ReverseResolve(base.EthClient, commonAddress)
	if err != nil {
		return nil, core.LogWarningReturn("Could not get Base ENS names: " + err.Error())
	}
	return []string{reverse}, nil
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
	return *big.NewInt(int64(23000000)) // YourPlace did not exist on-chain before this block
}
func BaseGetBytecode(database *db.Database, address string) ([]byte, error) {
	base := new(Base)
	base.Init(database)
	var bytecode hexutil.Bytes
	err := base.RpcClient.Call(&bytecode, "eth_getCode", address, "latest")
	if err != nil {
		return nil, core.LogWarningReturn("Could not get bytecode of Base address: " + err.Error())
	}
	return bytecode, nil
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
func WeiToEther(wei big.Int) float64 {
	weiAmount := big.NewInt(1000000000000000000) // 1 Ether = 10^18 Wei
	weiBigFloat := new(big.Float).SetInt(weiAmount)
	etherPerWei := big.NewFloat(1e-18) // 1 Ether = 10^18 Wei
	etherBigFloat := new(big.Float).Mul(weiBigFloat, etherPerWei)
	etherAmount, _ := etherBigFloat.Float64()
	return etherAmount
}
func WeiToEtherString(wei big.Int) string {
	ethValue := new(big.Float).SetInt(&wei)
	ethValue.Quo(ethValue, big.NewFloat(1e18))
	ethString := fmt.Sprintf("%.18f", ethValue)
	return ethString
}
