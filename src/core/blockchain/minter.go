package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/network"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	ipfscid "github.com/ipfs/go-cid"
)

type Minter struct {
	blockchain *Blockchain
	database   *db.Database
	ipfs       *network.IPFS
	mu         sync.Mutex
	keysMu     sync.RWMutex
	keys       map[string][]byte
}
type NFTRegistration struct {
	OperatorNetwork string              `json:"operatorNetwork"`
	OperatorAddress string              `json:"operatorAddress"`
	Chains          []db.NFTChainConfig `json:"chains"`
}
type NFTAcceptanceResponse struct {
	Transactions   [][]byte `json:"transactions"`
	RecipientIndex int      `json:"recipientIndex"`
}

func NewMinter(database *db.Database, blockchain *Blockchain, ipfs *network.IPFS) *Minter {
	return &Minter{database: database, blockchain: blockchain, ipfs: ipfs, keys: map[string][]byte{}}
}
func MinterRecordLogin(ctx context.Context, database *db.Database, network, address string) error {
	identity, err := WalletNFTIdentity(network, address)
	if err != nil {
		return err
	}
	return database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
		first, err := store.FirstLogin(ctx, identity, network, address)
		if err != nil || !first {
			return err
		}
		config, err := store.Config(ctx)
		if err != nil || !config.Enabled {
			return err
		}
		for _, chain := range config.Chains {
			if chain.Network != network {
				continue
			}
			grant := db.NFTGrant{ID: uuid.NewString(), Identity: identity, Recipient: address, Config: chain, Stage: "queued", CreatedAt: time.Now().Unix(), DueAt: time.Now().Unix()}
			return store.InsertGrant(ctx, grant)
		}
		return nil
	})
}
func (minter *Minter) Accept(ctx context.Context, identity string, signed []byte) error {
	minter.mu.Lock()
	defer minter.mu.Unlock()
	grant, err := minter.ownGrant(ctx, identity)
	if err != nil {
		return err
	}
	backend, err := minter.blockchain.nftChain(grant.Config.Network)
	if err != nil {
		return err
	}
	chain, ok := backend.(nftRecipientChain)
	if !ok || grant.Stage != "awaiting_recipient" || grant.Transaction != nil {
		return errors.New("acceptance_unavailable")
	}
	if err = chain.VerifyAcceptance(ctx, grant, signed); err != nil {
		return err
	}
	grant.RecipientSignature, grant.Reason, grant.DueAt = signed, "", time.Now().Unix()
	return minter.database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
		current, err := store.Grant(ctx, grant.ID)
		if err != nil {
			return err
		}
		if current.Transaction != nil || string(current.Acceptance) != string(grant.Acceptance) {
			return errors.New("acceptance_changed")
		}
		return store.SaveGrant(ctx, grant)
	})
}
func (minter *Minter) IsOperator(network, address string) bool {
	registration := NFTRegistered()
	expected, err := WalletNFTIdentity(registration.OperatorNetwork, registration.OperatorAddress)
	actual, actualErr := WalletNFTIdentity(network, address)
	return err == nil && actualErr == nil && actual == expected
}
func (minter *Minter) Prepare(ctx context.Context, identity string) (NFTAcceptanceResponse, error) {
	minter.mu.Lock()
	defer minter.mu.Unlock()
	response := NFTAcceptanceResponse{RecipientIndex: -1}
	config, err := minter.database.NFTConfig(ctx)
	if err != nil || !config.Enabled {
		return response, errors.New("auto_minting_paused")
	}
	grant, err := minter.ownGrant(ctx, identity)
	if err != nil {
		return response, err
	}
	policy, active := minter.policy(config, grant)
	if !active {
		return response, errors.New("auto_minting_paused")
	}
	grant.Config.MaxCost, grant.Config.MaxTopUp = policy.MaxCost, policy.MaxTopUp
	backend, err := minter.blockchain.nftChain(grant.Config.Network)
	if err != nil {
		return response, err
	}
	chain, ok := backend.(nftRecipientChain)
	if !ok || grant.Stage != "awaiting_recipient" || grant.Transaction != nil {
		return response, errors.New("acceptance_unavailable")
	}
	prepared, acceptance, err := chain.Prepare(ctx, grant)
	if err != nil {
		return response, err
	}
	grant.Acceptance, grant.RecipientSignature, grant.Reason, grant.DueAt = acceptance, nil, "", time.Now().Unix()
	err = minter.database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
		current, err := store.Grant(ctx, grant.ID)
		if err != nil {
			return err
		}
		if current.Transaction != nil || current.Stage != "awaiting_recipient" {
			return errors.New("acceptance_changed")
		}
		return store.SaveGrant(ctx, grant)
	})
	if err != nil {
		return response, err
	}
	return prepared, nil
}
func (minter *Minter) Readiness() map[string]bool {
	minter.keysMu.RLock()
	defer minter.keysMu.RUnlock()
	ready := map[string]bool{}
	for name, key := range minter.keys {
		ready[name] = len(key) > 0
	}
	return ready
}
func (minter *Minter) Retry(ctx context.Context, id string) error {
	minter.mu.Lock()
	defer minter.mu.Unlock()
	return minter.database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
		grant, err := store.Grant(ctx, id)
		if err != nil {
			return err
		}
		if grant.Stage == "delivered" {
			return errors.New("already_delivered")
		}
		grant.Reason, grant.DueAt = "", time.Now().Unix()
		grant.Stage = "queued"
		if grant.TokenID != "" {
			grant.Stage = "minted"
		}
		if grant.Transaction != nil {
			grant.Stage = "confirming"
		}
		return store.SaveGrant(ctx, grant)
	})
}
func (minter *Minter) SaveConfig(ctx context.Context, config db.NFTConfig) error {
	minter.mu.Lock()
	defer minter.mu.Unlock()
	if !config.Enabled {
		return minter.database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
			current, err := store.Config(ctx)
			if err != nil {
				return err
			}
			current.Enabled = false
			return store.SaveConfig(ctx, current)
		})
	}
	if config.DailyLimit < 0 || config.DailyLimit > 1000000 || len(config.Chains) > 3 {
		return errors.New("invalid_limits")
	}
	if config.Enabled && len(config.Chains) == 0 {
		return errors.New("no_chains_configured")
	}
	seen := map[string]bool{}
	registration := NFTRegistered()
	if _, err := WalletNFTIdentity(registration.OperatorNetwork, registration.OperatorAddress); err != nil {
		return errors.New("operator_not_registered")
	}
	current, err := minter.database.NFTConfig(ctx)
	if err != nil {
		return err
	}
	config.Template = current.Template
	if config.Template == nil {
		return errors.New("select_welcome_nft_first")
	}
	for index := range config.Chains {
		chain := &config.Chains[index]
		applyNFTTemplate(chain, config.Template)
		if seen[chain.Network] {
			return errors.New("duplicate_chain")
		}
		seen[chain.Network] = true
		registered := false
		for _, expected := range registration.Chains {
			if chain.Network == expected.Network && chain.Sender == expected.Sender && chain.ChainID == expected.ChainID && chain.CodeHash == expected.CodeHash && chain.Contract == expected.Contract {
				registered = true
			}
		}
		if !registered {
			return errors.New("sender_or_collection_not_registered")
		}
		identity, err := WalletNFTIdentity(chain.Network, chain.Sender)
		if err != nil {
			return err
		}
		chain.SignerIdentity = identity
		maximum, valid := new(big.Int).SetString(chain.MaxCost, 10)
		if !valid || maximum.Sign() <= 0 || maximum.BitLen() > 128 || chain.MaxTopUp > 10000000 {
			return errors.New("invalid_cost_limit")
		}
		if !strings.HasPrefix(chain.MetadataURI, "ipfs://") {
			return errors.New("metadata_must_use_ipfs")
		}
		cid := strings.TrimPrefix(chain.MetadataURI, "ipfs://")
		if _, err := ipfscid.Decode(cid); err != nil {
			return errors.New("invalid_metadata_cid")
		}
		metadata, err := minter.ipfs.IPFSReadFile(ctx, cid, 1<<20)
		if err != nil {
			return errors.New("metadata_unavailable")
		}
		var content struct {
			Name  string          `json:"name"`
			Image string          `json:"image"`
			Extra json.RawMessage `json:"extra_metadata"`
		}
		if json.Unmarshal(metadata, &content) != nil || content.Name == "" || content.Image == "" || len(content.Extra) != 0 {
			return errors.New("invalid_metadata")
		}
		hash := sha256.Sum256(metadata)
		chain.MetadataHash = hex.EncodeToString(hash[:])
		backend, err := minter.blockchain.nftChain(chain.Network)
		if err != nil {
			return err
		}
		if err = backend.Validate(ctx, *chain); err != nil {
			return err
		}
		if config.Enabled && len(minter.keys[chain.Network]) == 0 {
			return errors.New("credentials_not_provisioned")
		}
	}
	return minter.database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
		latest, err := store.Config(ctx)
		if err != nil {
			return err
		}
		if (latest.Template == nil) != (config.Template == nil) || (latest.Template != nil && *latest.Template != *config.Template) {
			return errors.New("template_changed_reload_settings")
		}
		return store.SaveConfig(ctx, config)
	})
}
func (minter *Minter) SelectTemplate(ctx context.Context, source db.NFTSource) error {
	registration := NFTRegistered()
	if source.Network != registration.OperatorNetwork {
		return errors.New("source_wallet_mismatch")
	}
	if _, err := WalletNFTIdentity(source.Network, registration.OperatorAddress); err != nil {
		return errors.New("operator_not_registered")
	}
	backend, err := minter.blockchain.nftChain(source.Network)
	if err != nil {
		return err
	}
	uri, err := backend.Source(ctx, source, registration.OperatorAddress)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(uri, "ipfs://") {
		return errors.New("source_requires_ipfs_metadata")
	}
	cid := strings.TrimPrefix(uri, "ipfs://")
	if _, err = ipfscid.Decode(cid); err != nil || len(uri)+5 > 96 {
		return errors.New("source_requires_file_cid")
	}
	metadata, err := minter.ipfs.IPFSImportFile(ctx, cid, 1<<20)
	if err != nil {
		return errors.New("source_metadata_could_not_be_pinned")
	}
	var content struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Image       string          `json:"image"`
		MimeType    string          `json:"image_mimetype"`
		Extra       json.RawMessage `json:"extra_metadata"`
	}
	if json.Unmarshal(metadata, &content) != nil || strings.TrimSpace(content.Name) == "" || len(content.Name) > 1024 || len(content.Description) > 16384 || len(content.Extra) != 0 {
		return errors.New("invalid_source_metadata")
	}
	if !strings.HasPrefix(content.Image, "ipfs://") {
		return errors.New("source_requires_ipfs_image")
	}
	imageCID := strings.TrimPrefix(content.Image, "ipfs://")
	if _, err = ipfscid.Decode(imageCID); err != nil {
		return errors.New("source_requires_image_file_cid")
	}
	if _, err = minter.ipfs.IPFSImportFile(ctx, imageCID, 64<<20); err != nil {
		return errors.New("source_media_unavailable_or_over_64_mib")
	}
	hash := sha256.Sum256(metadata)
	assetName, unit := WalletNFTAssetLabels(content.Name)
	template := &db.NFTTemplate{Source: source, MetadataURI: uri, MetadataHash: hex.EncodeToString(hash[:]), Name: content.Name, Description: content.Description, Image: content.Image, MimeType: content.MimeType, AssetName: assetName, Unit: unit}
	minter.mu.Lock()
	defer minter.mu.Unlock()
	return minter.database.NFTUpdate(ctx, "", func(store *db.NFTStore) error {
		config, err := store.Config(ctx)
		if err != nil {
			return err
		}
		config.Template = template
		for index := range config.Chains {
			applyNFTTemplate(&config.Chains[index], template)
		}
		return store.SaveConfig(ctx, config)
	})
}
func (minter *Minter) StartBootstrap() error {
	address := os.Getenv("YOURPLACE_MINTER_BOOTSTRAP_ADDR")
	if address == "" {
		return nil
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet && r.URL.Path == "/health" && r.Header.Get("Origin") == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/credentials" || r.Header.Get("Origin") != "" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "not allowed", http.StatusForbidden)
			return
		}
		var payload struct {
			Credentials map[string]string `json:"credentials"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || len(payload.Credentials) == 0 || len(payload.Credentials) > 3 {
			http.Error(w, "invalid credentials", http.StatusBadRequest)
			return
		}
		keys := map[string][]byte{}
		defer func() {
			for _, key := range keys {
				clear(key)
			}
			for name := range payload.Credentials {
				delete(payload.Credentials, name)
			}
		}()
		registration := NFTRegistered()
		for name, secret := range payload.Credentials {
			backend, err := minter.blockchain.nftChain(name)
			if err != nil {
				http.Error(w, "invalid credentials", http.StatusBadRequest)
				return
			}
			key, sender, err := backend.Credential(secret)
			if err != nil {
				http.Error(w, "invalid credentials", http.StatusBadRequest)
				return
			}
			keys[name] = key
			matched := false
			actual, _ := WalletNFTIdentity(name, sender)
			for _, expected := range registration.Chains {
				identity, identityErr := WalletNFTIdentity(expected.Network, expected.Sender)
				if identityErr == nil && name == expected.Network && identity == actual {
					matched = true
				}
			}
			if !matched {
				http.Error(w, "sender mismatch", http.StatusBadRequest)
				return
			}
		}
		minter.mu.Lock()
		minter.keysMu.Lock()
		for name, key := range keys {
			clear(minter.keys[name])
			minter.keys[name] = append([]byte(nil), key...)
		}
		minter.keysMu.Unlock()
		minter.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 4096}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			core.LogDebug("Minter credential listener stopped")
		}
	}()
	return nil
}
func (minter *Minter) Tick() {
	if !minter.mu.TryLock() {
		return
	}
	defer minter.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	owner := uuid.NewString()
	if err := minter.database.NFTAcquire(ctx, owner); err != nil {
		return
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		_ = minter.database.NFTRelease(releaseCtx, owner)
	}()
	grants, err := minter.database.NFTGrants(ctx, "", 0, 10, true)
	if err != nil {
		core.LogDebug("Minter queue unavailable")
		return
	}
	for _, grant := range grants {
		if ctx.Err() != nil {
			return
		}
		stepCtx, stepCancel := context.WithTimeout(ctx, 8*time.Second)
		minter.advance(stepCtx, owner, grant)
		stepCancel()
	}
}
func NFTRegistered() NFTRegistration {
	var registration NFTRegistration
	if json.Unmarshal([]byte(os.Getenv("YOURPLACE_MINTER_REGISTRATION")), &registration) != nil {
		return NFTRegistration{}
	}
	return registration
}
func applyNFTTemplate(config *db.NFTChainConfig, template *db.NFTTemplate) {
	if template != nil {
		config.MetadataURI, config.MetadataHash = template.MetadataURI, template.MetadataHash
		config.Name, config.Unit = template.AssetName, template.Unit
	}
}
func (minter *Minter) advance(ctx context.Context, owner string, grant db.NFTGrant) {
	backend, err := minter.blockchain.nftChain(grant.Config.Network)
	if err != nil {
		minter.wait(ctx, owner, grant, "chain_unavailable")
		return
	}
	config, err := minter.database.NFTConfig(ctx)
	if err != nil {
		return
	}
	policy, enabled := minter.policy(config, grant)
	if grant.Transaction != nil {
		outcome, token, err := backend.Reconcile(ctx, grant)
		if err != nil {
			minter.wait(ctx, owner, grant, "confirmation_unavailable")
			return
		}
		switch outcome {
		case "confirmed":
			grant.LastTransactionHash = grant.Transaction.Hash
			if grant.Transaction.Operation == "mint" {
				grant.TokenID, grant.Stage = token, "minted"
			} else {
				grant.Stage, grant.DeliveredAt = "delivered", time.Now().Unix()
			}
			grant.Transaction, grant.Acceptance, grant.RecipientSignature = nil, nil, nil
			grant.Reason, grant.DueAt = "", time.Now().Unix()
			_ = minter.database.NFTUpdate(ctx, owner, func(store *db.NFTStore) error { return store.SaveGrant(ctx, grant) })
		case "reverted":
			grant.Transaction = nil
			grant.Stage = "queued"
			if grant.TokenID != "" {
				grant.Stage = "minted"
			}
			minter.wait(ctx, owner, grant, "confirmed_revert")
		case "attention":
			minter.wait(ctx, owner, grant, "transaction_needs_attention")
		default:
			if enabled && outcome == "pending" {
				if minter.database.NFTUpdate(ctx, owner, func(store *db.NFTStore) error {
					current, err := store.Config(ctx)
					if err != nil {
						return err
					}
					if _, active := minter.policy(current, grant); !active {
						return errors.New("paused")
					}
					return nil
				}) == nil {
					_ = backend.Broadcast(ctx, grant.Config, grant.Transaction)
				}
			}
			minter.wait(ctx, owner, grant, outcome)
		}
		return
	}
	if !enabled {
		minter.wait(ctx, owner, grant, "paused")
		return
	}
	if grant.Reason == "confirmed_revert" || grant.Reason == "transaction_needs_attention" {
		return
	}
	key := minter.keys[grant.Config.Network]
	if len(key) == 0 {
		minter.wait(ctx, owner, grant, "credentials_not_provisioned")
		return
	}
	grant.Config.MaxCost, grant.Config.MaxTopUp = policy.MaxCost, policy.MaxTopUp
	err = minter.database.NFTUpdate(ctx, owner, func(store *db.NFTStore) error {
		pending, err := store.PendingSigner(ctx, grant)
		if err != nil {
			return err
		}
		if pending {
			return errors.New("signer_busy")
		}
		return nil
	})
	if err != nil {
		minter.wait(ctx, owner, grant, "signer_busy")
		return
	}
	tx, err := backend.Build(ctx, grant, key)
	if err != nil {
		reason := err.Error()
		if reason == "awaiting_recipient" || reason == "approval_expired" {
			grant.Stage = "awaiting_recipient"
			if reason == "approval_expired" {
				grant.Acceptance, grant.RecipientSignature = nil, nil
			}
		}
		minter.wait(ctx, owner, grant, reason)
		return
	}
	grant.Transaction, grant.Stage, grant.Reason, grant.DueAt = tx, "confirming", "", time.Now().Unix()
	err = minter.database.NFTUpdate(ctx, owner, func(store *db.NFTStore) error {
		current, err := store.Config(ctx)
		if err != nil {
			return err
		}
		currentPolicy, active := minter.policy(current, grant)
		if !active || currentPolicy.MaxCost != grant.Config.MaxCost || currentPolicy.MaxTopUp != grant.Config.MaxTopUp {
			return errors.New("paused")
		}
		pending, err := store.PendingSigner(ctx, grant)
		if err != nil {
			return err
		}
		if pending {
			return errors.New("signer_busy")
		}
		if tx.Operation == "mint" {
			if err = store.Reserve(ctx, &grant, current.DailyLimit); err != nil {
				return err
			}
		}
		if err = store.Journal(ctx, grant); err != nil {
			return err
		}
		return store.SaveGrant(ctx, grant)
	})
	if err != nil {
		grant.Transaction = nil
		grant.Stage = "queued"
		if grant.TokenID != "" {
			grant.Stage = "minted"
		}
		if err.Error() == "daily_limit" {
			minter.wait(ctx, owner, grant, "daily_limit")
		}
		return
	}
	_ = backend.Broadcast(ctx, grant.Config, tx)
}
func (minter *Minter) ownGrant(ctx context.Context, identity string) (db.NFTGrant, error) {
	grants, err := minter.database.NFTGrants(ctx, identity, 0, 1, false)
	if err != nil {
		return db.NFTGrant{}, err
	}
	if len(grants) != 1 {
		return db.NFTGrant{}, errors.New("grant_unavailable")
	}
	return grants[0], nil
}
func (minter *Minter) policy(config db.NFTConfig, grant db.NFTGrant) (db.NFTChainConfig, bool) {
	for _, chain := range config.Chains {
		if chain.Network == grant.Config.Network && chain.ChainID == grant.Config.ChainID && chain.SignerIdentity == grant.Config.SignerIdentity {
			return chain, config.Enabled
		}
	}
	return db.NFTChainConfig{}, false
}
func (minter *Minter) wait(ctx context.Context, owner string, grant db.NFTGrant, reason string) {
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	grant.Reason, grant.DueAt = reason, time.Now().Add(time.Minute).Unix()
	if reason == "confirmed_revert" || reason == "transaction_needs_attention" {
		grant.Stage = "attention"
		grant.DueAt = time.Now().Add(24 * time.Hour).Unix()
	}
	_ = minter.database.NFTUpdate(saveCtx, owner, func(store *db.NFTStore) error { return store.SaveGrant(saveCtx, grant) })
}
