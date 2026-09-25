package blockchain

import (
	"YourPlace/src/core/db"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/encoding/msgpack"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"
)

type algoNFT struct{ client *algod.Client }
type nftAcceptance struct {
	Transactions   []types.Transaction `json:"transactions"`
	RecipientIndex int                 `json:"recipientIndex"`
}

func (chain *algoNFT) Build(ctx context.Context, grant db.NFTGrant, key []byte) (*db.NFTTransaction, error) {
	if err := chain.Validate(ctx, grant.Config); err != nil {
		return nil, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("credential_unavailable")
	}
	sender, _ := types.DecodeAddress(grant.Config.Sender)
	if !bytes.Equal(key[32:], sender[:]) {
		return nil, errors.New("sender_mismatch")
	}
	var txns []types.Transaction
	recipientIndex := -1
	operation := "mint"
	if grant.TokenID == "" {
		params, err := chain.params(ctx, grant.Config)
		if err != nil {
			return nil, err
		}
		hash, err := hex.DecodeString(grant.Config.MetadataHash)
		if err != nil || len(hash) != 32 {
			return nil, errors.New("invalid_metadata_hash")
		}
		tx, err := transaction.MakeAssetCreateTxn(grant.Config.Sender, []byte(grant.ID), params, 1, 0, false, "", "", "", "", grant.Config.Unit, grant.Config.Name, grant.Config.MetadataURI+"#arc3", string(hash))
		if err != nil {
			return nil, errors.New("invalid_asset")
		}
		tx.Lease = sha256.Sum256([]byte(grant.ID + ":mint"))
		txns = []types.Transaction{tx}
	} else {
		operation = "transfer"
		if len(grant.Acceptance) == 0 {
			acceptance, err := chain.prepare(ctx, grant)
			if err != nil {
				return nil, err
			}
			if acceptance.RecipientIndex >= 0 {
				return nil, errors.New("awaiting_recipient")
			}
			txns = acceptance.Transactions
		} else {
			var acceptance nftAcceptance
			if err := msgpack.Decode(grant.Acceptance, &acceptance); err != nil {
				return nil, errors.New("invalid_acceptance")
			}
			txns, recipientIndex = acceptance.Transactions, acceptance.RecipientIndex
			if recipientIndex >= 0 {
				if len(grant.RecipientSignature) == 0 {
					return nil, errors.New("awaiting_recipient")
				}
				if err := chain.VerifyAcceptance(ctx, grant, grant.RecipientSignature); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(txns) == 0 || len(txns) > 3 {
		return nil, errors.New("invalid_acceptance")
	}
	status, err := chain.client.Status().Do(ctx)
	if err != nil {
		return nil, errors.New("rpc_unavailable")
	}
	if uint64(txns[0].LastValid) <= status.LastRound+10 {
		return nil, errors.New("approval_expired")
	}
	var cost uint64
	for index, tx := range txns {
		if index == recipientIndex {
			continue
		}
		if tx.Sender != sender {
			return nil, errors.New("invalid_acceptance")
		}
		if uint64(tx.Amount) > grant.Config.MaxTopUp {
			return nil, errors.New("top_up_limit")
		}
		cost += uint64(tx.Fee) + uint64(tx.Amount)
	}
	maximum, err := strconv.ParseUint(grant.Config.MaxCost, 10, 64)
	if err != nil || cost > maximum {
		return nil, errors.New("cost_limit")
	}
	account, err := chain.client.AccountInformation(grant.Config.Sender).Do(ctx)
	reserve := uint64(0)
	if operation == "mint" {
		reserve = 200000
	}
	if err != nil || account.Amount < account.MinBalance || account.Amount-account.MinBalance < cost+reserve {
		return nil, errors.New("insufficient_balance")
	}
	var raw []byte
	for index, tx := range txns {
		if index == recipientIndex {
			raw = append(raw, grant.RecipientSignature...)
			continue
		}
		_, signed, signErr := crypto.SignTransaction(ed25519.PrivateKey(key), tx)
		if signErr != nil {
			return nil, errors.New("signing_failed")
		}
		raw = append(raw, signed...)
	}
	last := txns[len(txns)-1]
	return &db.NFTTransaction{Hash: crypto.GetTxID(last), Raw: raw, Operation: operation, LastRound: uint64(last.LastValid)}, nil
}
func (chain *algoNFT) Broadcast(ctx context.Context, config db.NFTChainConfig, tx *db.NFTTransaction) error {
	if _, err := chain.params(ctx, config); err != nil {
		return err
	}
	_, err := chain.client.SendRawTransaction(tx.Raw).Do(ctx)
	return err
}
func (chain *algoNFT) Credential(secret string) ([]byte, string, error) {
	key, err := mnemonic.ToPrivateKey(strings.TrimSpace(secret))
	if err != nil {
		return nil, "", errors.New("invalid_recovery_phrase")
	}
	var address types.Address
	copy(address[:], key[32:])
	return key, address.String(), nil
}
func (chain *algoNFT) Prepare(ctx context.Context, grant db.NFTGrant) (NFTAcceptanceResponse, []byte, error) {
	response := NFTAcceptanceResponse{RecipientIndex: -1}
	acceptance, err := chain.prepare(ctx, grant)
	if err != nil {
		return response, nil, err
	}
	response.RecipientIndex = acceptance.RecipientIndex
	for _, tx := range acceptance.Transactions {
		response.Transactions = append(response.Transactions, msgpack.Encode(tx))
	}
	return response, msgpack.Encode(acceptance), nil
}
func (chain *algoNFT) prepare(ctx context.Context, grant db.NFTGrant) (nftAcceptance, error) {
	result := nftAcceptance{RecipientIndex: -1}
	asset, err := strconv.ParseUint(grant.TokenID, 10, 64)
	if err != nil || asset == 0 {
		return result, errors.New("invalid_asset")
	}
	params, err := chain.params(ctx, grant.Config)
	if err != nil {
		return result, err
	}
	account, err := chain.client.AccountInformation(grant.Recipient).Do(ctx)
	if err != nil {
		return result, errors.New("recipient_unavailable")
	}
	opted := false
	for _, holding := range account.Assets {
		if holding.AssetId == asset {
			opted = true
			break
		}
	}
	fee := params.Fee
	if !opted {
		minimum := account.MinBalance
		if minimum < 100000 {
			minimum = 100000
		}
		topUp := uint64(0)
		if account.Amount < minimum+100000 {
			topUp = minimum + 100000 - account.Amount
		}
		if topUp > grant.Config.MaxTopUp {
			return result, errors.New("top_up_limit")
		}
		if topUp > 0 {
			fund, fundErr := transaction.MakePaymentTxn(grant.Config.Sender, grant.Recipient, topUp, []byte(grant.ID), "", params)
			if fundErr != nil {
				return result, fundErr
			}
			result.Transactions = append(result.Transactions, fund)
		}
		params.Fee = 0
		optIn, optErr := transaction.MakeAssetAcceptanceTxn(grant.Recipient, []byte(grant.ID), params, asset)
		if optErr != nil {
			return result, optErr
		}
		result.RecipientIndex = len(result.Transactions)
		result.Transactions = append(result.Transactions, optIn)
		params.Fee = fee * 2
	}
	transfer, err := transaction.MakeAssetTransferTxn(grant.Config.Sender, grant.Recipient, 1, []byte(grant.ID), params, "", asset)
	if err != nil {
		return result, err
	}
	transfer.Lease = sha256.Sum256([]byte(grant.ID + ":transfer"))
	result.Transactions = append(result.Transactions, transfer)
	result.Transactions, err = transaction.AssignGroupID(result.Transactions, "")
	return result, err
}
func (chain *algoNFT) Reconcile(ctx context.Context, grant db.NFTGrant) (string, string, error) {
	if _, err := chain.params(ctx, grant.Config); err != nil {
		return "", "", err
	}
	pending, _, err := chain.client.PendingTransactionInformation(grant.Transaction.Hash).Do(ctx)
	if err == nil && pending.ConfirmedRound > 0 {
		if grant.Transaction.Operation == "mint" {
			if pending.AssetIndex == 0 {
				return "attention", "", nil
			}
			asset, assetErr := chain.client.GetAssetByID(pending.AssetIndex).Do(ctx)
			if assetErr != nil {
				return "", "", errors.New("rpc_unavailable")
			}
			params := asset.Params
			hash, hashErr := hex.DecodeString(grant.Config.MetadataHash)
			zero := (types.Address{}).String()
			empty := func(value string) bool { return value == "" || value == zero }
			if hashErr != nil || !bytes.Equal(params.MetadataHash, hash) || params.Url != grant.Config.MetadataURI+"#arc3" || params.Creator != grant.Config.Sender || params.Total != 1 || params.Decimals != 0 || params.DefaultFrozen || !empty(params.Manager) || !empty(params.Reserve) || !empty(params.Freeze) || !empty(params.Clawback) {
				return "attention", "", nil
			}
			return "confirmed", strconv.FormatUint(pending.AssetIndex, 10), nil
		}
		asset, _ := strconv.ParseUint(grant.TokenID, 10, 64)
		tx := pending.Transaction.Txn
		if tx.Type != types.AssetTransferTx || uint64(tx.XferAsset) != asset || tx.AssetAmount != 1 || tx.AssetReceiver.String() != grant.Recipient || tx.Sender.String() != grant.Config.Sender {
			return "attention", "", nil
		}
		return "confirmed", grant.TokenID, nil
	}
	status, statusErr := chain.client.Status().Do(ctx)
	if statusErr != nil {
		return "", "", errors.New("rpc_unavailable")
	}
	// An expired, missing transaction may have confirmed before node history was pruned.
	if status.LastRound > grant.Transaction.LastRound {
		return "attention", "", nil
	}
	return "pending", "", nil
}
func (chain *algoNFT) Source(ctx context.Context, source db.NFTSource, owner string) (string, error) {
	assetID, err := strconv.ParseUint(source.TokenID, 10, 64)
	if err != nil || assetID == 0 || source.Contract != source.TokenID {
		return "", errors.New("invalid_source_nft")
	}
	account, err := chain.client.AccountInformation(owner).Do(ctx)
	if err != nil {
		return "", errors.New("source_owner_unavailable")
	}
	owned := false
	for _, holding := range account.Assets {
		if holding.AssetId == assetID && holding.Amount == 1 {
			owned = true
			break
		}
	}
	if !owned {
		return "", errors.New("source_nft_not_owned")
	}
	asset, err := chain.client.GetAssetByID(assetID).Do(ctx)
	if err != nil || asset.Params.Total != 1 || asset.Params.Decimals != 0 || !strings.HasSuffix(asset.Params.Url, "#arc3") {
		return "", errors.New("source_requires_arc3_metadata")
	}
	return strings.TrimSuffix(asset.Params.Url, "#arc3"), nil
}
func (chain *algoNFT) Validate(ctx context.Context, config db.NFTChainConfig) error {
	if len(config.Name) == 0 || len(config.Name) > types.AssetNameMaxLen || len(config.Unit) > types.AssetUnitNameMaxLen || len(config.MetadataURI)+5 > types.AssetURLMaxLen {
		return errors.New("invalid_asset_metadata")
	}
	if _, err := chain.params(ctx, config); err != nil {
		return err
	}
	account, err := chain.client.AccountInformation(config.Sender).Do(ctx)
	if err != nil {
		return errors.New("issuer_unavailable")
	}
	if account.AuthAddr != "" && account.AuthAddr != config.Sender {
		return errors.New("rekeyed_issuer_not_supported")
	}
	return nil
}
func (chain *algoNFT) VerifyAcceptance(ctx context.Context, grant db.NFTGrant, raw []byte) error {
	var acceptance nftAcceptance
	var signed types.SignedTxn
	if len(raw) > 8192 || msgpack.Decode(grant.Acceptance, &acceptance) != nil || msgpack.Decode(raw, &signed) != nil {
		return errors.New("invalid_approval")
	}
	index := acceptance.RecipientIndex
	if index < 0 || index >= len(acceptance.Transactions) {
		return errors.New("invalid_approval")
	}
	expected := acceptance.Transactions[index]
	if !bytes.Equal(msgpack.Encode(expected), msgpack.Encode(signed.Txn)) || !bytes.Equal(raw, msgpack.Encode(types.SignedTxn{Txn: expected, Sig: signed.Sig, AuthAddr: signed.AuthAddr})) {
		return errors.New("approval_mismatch")
	}
	account, err := chain.client.AccountInformation(grant.Recipient).Do(ctx)
	if err != nil {
		return errors.New("recipient_unavailable")
	}
	authorizer := grant.Recipient
	if account.AuthAddr != "" {
		authorizer = account.AuthAddr
	}
	address, err := types.DecodeAddress(authorizer)
	if err != nil {
		return errors.New("invalid_authorizer")
	}
	if authorizer != grant.Recipient && signed.AuthAddr != address {
		return errors.New("invalid_authorizer")
	}
	if authorizer == grant.Recipient && signed.AuthAddr != (types.Address{}) {
		return errors.New("invalid_authorizer")
	}
	message := append([]byte("TX"), msgpack.Encode(expected)...)
	if !ed25519.Verify(address[:], message, signed.Sig[:]) {
		return errors.New("invalid_signature")
	}
	return nil
}
func (chain *algoNFT) params(ctx context.Context, config db.NFTChainConfig) (types.SuggestedParams, error) {
	params, err := chain.client.SuggestedParams().Do(ctx)
	if err != nil {
		return params, errors.New("rpc_unavailable")
	}
	if base64.StdEncoding.EncodeToString(params.GenesisHash) != config.ChainID {
		return params, errors.New("network_mismatch")
	}
	params.FlatFee = true
	params.Fee = types.MicroAlgos(params.MinFee)
	if params.Fee < transaction.MinTxnFee {
		params.Fee = transaction.MinTxnFee
	}
	if params.Fee > 1000000 {
		return params, errors.New("fee_limit")
	}
	return params, nil
}
