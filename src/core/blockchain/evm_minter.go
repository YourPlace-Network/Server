package blockchain

import (
	"YourPlace/src/core/db"
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type evmNFT struct {
	client   *ethclient.Client
	extraFee func(context.Context, []byte) (*big.Int, error)
}

var nftABI, _ = abi.JSON(strings.NewReader(`[
{"type":"function","name":"mint","inputs":[{"name":"uri","type":"string"}],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"mintFee","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"ownerOf","inputs":[{"type":"uint256"}],"outputs":[{"type":"address"}]},
{"type":"function","name":"tokenURI","inputs":[{"type":"uint256"}],"outputs":[{"type":"string"}]},
{"type":"function","name":"safeTransferFrom","inputs":[{"type":"address"},{"type":"address"},{"type":"uint256"}],"outputs":[]}
]`))
var nftTransferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

func (chain *evmNFT) Build(ctx context.Context, grant db.NFTGrant, key []byte) (*db.NFTTransaction, error) {
	config := grant.Config
	if err := chain.Validate(ctx, config); err != nil {
		return nil, err
	}
	privateKey, err := crypto.ToECDSA(key)
	if err != nil {
		return nil, errors.New("credential_unavailable")
	}
	defer privateKey.D.SetInt64(0)
	sender, contract := common.HexToAddress(config.Sender), common.HexToAddress(config.Contract)
	if crypto.PubkeyToAddress(privateKey.PublicKey) != sender {
		return nil, errors.New("sender_mismatch")
	}
	chainID, _ := new(big.Int).SetString(config.ChainID, 10)
	value, operation := new(big.Int), "mint"
	var data []byte
	if grant.TokenID == "" {
		feeData, _ := nftABI.Pack("mintFee")
		fee, feeErr := chain.client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: feeData}, nil)
		if feeErr != nil || len(fee) != 32 {
			return nil, errors.New("mint_fee_unavailable")
		}
		value.SetBytes(fee)
		data, err = nftABI.Pack("mint", config.MetadataURI)
	} else {
		operation = "transfer"
		token, valid := new(big.Int).SetString(grant.TokenID, 10)
		if !valid || token.Sign() < 0 {
			return nil, errors.New("invalid_token")
		}
		ownerData, _ := nftABI.Pack("ownerOf", token)
		owner, ownerErr := chain.client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: ownerData}, nil)
		if ownerErr != nil || len(owner) != 32 || common.BytesToAddress(owner) != sender {
			return nil, errors.New("issuer_no_longer_owns_token")
		}
		data, err = nftABI.Pack("safeTransferFrom", sender, common.HexToAddress(grant.Recipient), token)
	}
	if err != nil {
		return nil, err
	}
	nonce, err := chain.client.PendingNonceAt(ctx, sender)
	if err != nil {
		return nil, errors.New("rpc_unavailable")
	}
	confirmedNonce, err := chain.client.NonceAt(ctx, sender, nil)
	if err != nil || nonce != confirmedNonce {
		return nil, errors.New("signer_has_pending_transactions")
	}
	gasPrice, err := chain.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, errors.New("fee_unavailable")
	}
	gas, err := chain.client.EstimateGas(ctx, ethereum.CallMsg{From: sender, To: &contract, Value: value, GasPrice: gasPrice, Data: data})
	if err != nil {
		return nil, errors.New("simulation_failed")
	}
	if gas > 10000000 {
		return nil, errors.New("gas_limit_exceeded")
	}
	gas = gas + gas/5
	tx := types.NewTransaction(nonce, contract, value, gas, gasPrice, data)
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	cost := new(big.Int).Add(value, new(big.Int).Mul(new(big.Int).SetUint64(gas), gasPrice))
	if chain.extraFee != nil {
		fee, feeErr := chain.extraFee(ctx, raw)
		if feeErr != nil {
			return nil, feeErr
		}
		cost.Add(cost, fee)
	}
	maximum, valid := new(big.Int).SetString(config.MaxCost, 10)
	if !valid || maximum.Sign() <= 0 || cost.Cmp(maximum) > 0 {
		return nil, errors.New("cost_limit")
	}
	balance, err := chain.client.BalanceAt(ctx, sender, nil)
	if err != nil || balance.Cmp(cost) < 0 {
		return nil, errors.New("insufficient_balance")
	}
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		return nil, errors.New("signing_failed")
	}
	raw, err = signed.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &db.NFTTransaction{Hash: signed.Hash().Hex(), Raw: raw, Operation: operation, Nonce: nonce}, nil
}
func (chain *evmNFT) Broadcast(ctx context.Context, config db.NFTChainConfig, transaction *db.NFTTransaction) error {
	if err := chain.Validate(ctx, config); err != nil {
		return err
	}
	var tx types.Transaction
	if err := tx.UnmarshalBinary(transaction.Raw); err != nil {
		return err
	}
	if tx.Hash().Hex() != transaction.Hash {
		return errors.New("journal_mismatch")
	}
	return chain.client.SendTransaction(ctx, &tx)
}
func (chain *evmNFT) Credential(secret string) ([]byte, string, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(secret), "0x"))
	if err != nil {
		return nil, "", errors.New("invalid_private_key")
	}
	defer key.D.SetInt64(0)
	return crypto.FromECDSA(key), crypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}
func (chain *evmNFT) Reconcile(ctx context.Context, grant db.NFTGrant) (string, string, error) {
	chainID, err := chain.client.ChainID(ctx)
	if err != nil || chainID.String() != grant.Config.ChainID {
		return "", "", errors.New("network_mismatch")
	}
	receipt, err := chain.client.TransactionReceipt(ctx, common.HexToHash(grant.Transaction.Hash))
	if errors.Is(err, ethereum.NotFound) {
		nonce, nonceErr := chain.client.NonceAt(ctx, common.HexToAddress(grant.Config.Sender), nil)
		if nonceErr == nil && nonce > grant.Transaction.Nonce {
			return "attention", "", nil
		}
		return "pending", "", nonceErr
	}
	if err != nil {
		return "", "", errors.New("rpc_unavailable")
	}
	finalized, err := chain.client.HeaderByNumber(ctx, big.NewInt(-3))
	if err != nil {
		return "", "", errors.New("finality_unavailable")
	}
	if receipt.BlockNumber.Cmp(finalized.Number) > 0 {
		return "confirming", "", nil
	}
	canonical, err := chain.client.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil || canonical.Hash() != receipt.BlockHash {
		return "confirming", "", nil
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return "reverted", "", nil
	}
	from, to := common.Address{}, common.HexToAddress(grant.Config.Sender)
	if grant.Transaction.Operation == "transfer" {
		from, to = to, common.HexToAddress(grant.Recipient)
	}
	token := ""
	for _, event := range receipt.Logs {
		if event.Address != common.HexToAddress(grant.Config.Contract) || len(event.Topics) != 4 || event.Topics[0] != nftTransferTopic {
			continue
		}
		if common.BytesToAddress(event.Topics[1].Bytes()) != from || common.BytesToAddress(event.Topics[2].Bytes()) != to {
			continue
		}
		id := new(big.Int).SetBytes(event.Topics[3].Bytes()).String()
		if grant.TokenID != "" && id != grant.TokenID {
			continue
		}
		if token != "" {
			return "attention", "", nil
		}
		token = id
	}
	if token == "" {
		return "attention", "", nil
	}
	return "confirmed", token, nil
}
func (chain *evmNFT) Source(ctx context.Context, source db.NFTSource, owner string) (string, error) {
	token, valid := new(big.Int).SetString(source.TokenID, 10)
	if !valid || token.Sign() < 0 || token.BitLen() > 256 || !common.IsHexAddress(source.Contract) || common.HexToAddress(source.Contract) == (common.Address{}) {
		return "", errors.New("invalid_source_nft")
	}
	contract := common.HexToAddress(source.Contract)
	ownerData, _ := nftABI.Pack("ownerOf", token)
	result, err := chain.client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: ownerData}, nil)
	if err != nil || len(result) != 32 || common.BytesToAddress(result) != common.HexToAddress(owner) {
		return "", errors.New("source_nft_not_owned")
	}
	uriData, _ := nftABI.Pack("tokenURI", token)
	result, err = chain.client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: uriData}, nil)
	if err != nil || len(result) > 4096 {
		return "", errors.New("source_metadata_unavailable")
	}
	values, err := nftABI.Unpack("tokenURI", result)
	if err != nil || len(values) != 1 {
		return "", errors.New("source_metadata_unavailable")
	}
	uri, ok := values[0].(string)
	if !ok {
		return "", errors.New("source_metadata_unavailable")
	}
	return uri, nil
}
func (chain *evmNFT) Validate(ctx context.Context, config db.NFTChainConfig) error {
	if !common.IsHexAddress(config.Contract) || common.HexToAddress(config.Contract) == (common.Address{}) {
		return errors.New("invalid_contract")
	}
	chainID, err := chain.client.ChainID(ctx)
	if err != nil || chainID.String() != config.ChainID {
		return errors.New("network_mismatch")
	}
	approved := ""
	for _, chain := range NFTRegistered().Chains {
		if chain.Network == config.Network {
			approved = chain.CodeHash
		}
	}
	if len(approved) != 66 || config.CodeHash != approved {
		return errors.New("contract_not_reviewed")
	}
	code, err := chain.client.CodeAt(ctx, common.HexToAddress(config.Contract), nil)
	if err != nil || len(code) == 0 || crypto.Keccak256Hash(code).Hex() != approved {
		return errors.New("contract_code_mismatch")
	}
	return nil
}
