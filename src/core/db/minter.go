package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type NFTChainConfig struct {
	SignerIdentity string `json:"signerIdentity"`
	Network        string `json:"network"`
	ChainID        string `json:"chainId"`
	Sender         string `json:"sender"`
	Contract       string `json:"contract"`
	CodeHash       string `json:"codeHash"`
	MetadataURI    string `json:"metadataUri"`
	MetadataHash   string `json:"metadataHash"`
	Name           string `json:"name"`
	Unit           string `json:"unit"`
	MaxCost        string `json:"maxCost"`
	MaxTopUp       uint64 `json:"maxTopUp"`
}
type NFTConfig struct {
	Enabled    bool             `json:"enabled"`
	DailyLimit int              `json:"dailyLimit"`
	Chains     []NFTChainConfig `json:"chains"`
	Template   *NFTTemplate     `json:"template,omitempty"`
}
type NFTSource struct {
	Network  string `json:"network"`
	Contract string `json:"contract"`
	TokenID  string `json:"tokenId"`
}
type NFTTemplate struct {
	Source       NFTSource `json:"source"`
	MetadataURI  string    `json:"metadataUri"`
	MetadataHash string    `json:"metadataHash"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Image        string    `json:"image"`
	MimeType     string    `json:"mimeType"`
	AssetName    string    `json:"assetName"`
	Unit         string    `json:"unit"`
}
type NFTGrant struct {
	ID                  string          `json:"id"`
	Revision            int64           `json:"revision"`
	Identity            string          `json:"identity"`
	Recipient           string          `json:"recipient"`
	Config              NFTChainConfig  `json:"config"`
	Stage               string          `json:"stage"`
	TokenID             string          `json:"tokenId"`
	LastTransactionHash string          `json:"lastTransactionHash"`
	Reason              string          `json:"reason"`
	CreatedAt           int64           `json:"createdAt"`
	DeliveredAt         int64           `json:"deliveredAt"`
	DueAt               int64           `json:"dueAt"`
	IssuanceDay         string          `json:"issuanceDay"`
	Transaction         *NFTTransaction `json:"transaction,omitempty"`
	Acceptance          []byte          `json:"acceptance,omitempty"`
	ApprovalReceived    bool            `json:"approvalReceived"`
	RecipientSignature  []byte          `json:"recipientSignature,omitempty"`
}
type NFTProfile struct {
	FirstLoginAt          int64 `json:"firstLoginAt"`
	WelcomeNFTDeliveredAt int64 `json:"welcomeNFTDeliveredAt"`
	WelcomeNFTDelivered   bool  `json:"welcomeNFTDelivered"`
}
type NFTTransaction struct {
	Hash      string `json:"hash"`
	Raw       []byte `json:"raw,omitempty"`
	Operation string `json:"operation"`
	LastRound uint64 `json:"lastRound"`
	Nonce     uint64 `json:"nonce"`
}
type NFTStore struct {
	tx  *sql.Tx
	now int64
}

var ErrNFTLease = errors.New("minter lease unavailable")

func (db *Database) NFTAcquire(ctx context.Context, owner string) error {
	return db.NFTUpdate(ctx, "", func(store *NFTStore) error {
		var current string
		var expires int64
		if err := store.tx.QueryRowContext(ctx, "SELECT leaseOwner, leaseUntil FROM auto_nft_control WHERE id = 1").Scan(&current, &expires); err != nil {
			return err
		}
		if current != "" && expires > store.now {
			return ErrNFTLease
		}
		_, err := store.tx.ExecContext(ctx, "UPDATE auto_nft_control SET leaseOwner = ?, leaseUntil = ? WHERE id = 1", owner, store.now+120)
		return err
	})
}
func (db *Database) NFTConfig(ctx context.Context) (NFTConfig, error) {
	var config NFTConfig
	var raw string
	err := db.nftSQL().QueryRowContext(ctx, "SELECT config FROM auto_nft_control WHERE id = 1").Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &config)
	}
	return config, err
}
func (db *Database) NFTCollections(ctx context.Context) ([]NFTChainConfig, error) {
	current, err := db.NFTConfig(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.nftSQL().QueryContext(ctx, "SELECT DISTINCT network, collection FROM auto_nft_grants WHERE collection <> ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	collections := []NFTChainConfig{}
	seen := map[string]bool{}
	for _, config := range current.Chains {
		if config.Contract != "" {
			collections = append(collections, NFTChainConfig{Network: config.Network, Contract: config.Contract})
			seen[config.Network+":"+config.Contract] = true
		}
	}
	for rows.Next() {
		var config NFTChainConfig
		if err = rows.Scan(&config.Network, &config.Contract); err != nil {
			return nil, err
		}
		if !seen[config.Network+":"+config.Contract] {
			collections = append(collections, config)
			seen[config.Network+":"+config.Contract] = true
		}
	}
	return collections, rows.Err()
}
func (db *Database) NFTGrants(ctx context.Context, identity string, offset, limit int, due bool) ([]NFTGrant, error) {
	query := "SELECT body FROM auto_nft_grants"
	args := []any{}
	if identity != "" {
		query += " WHERE identityKey = ?"
		args = append(args, identity)
	}
	if due {
		if identity == "" {
			query += " WHERE "
		} else {
			query += " AND "
		}
		query += "stage NOT IN ('delivered', 'attention') AND dueAt <= ?"
		args = append(args, time.Now().Unix())
	}
	if due {
		query += " ORDER BY pending DESC, dueAt, id LIMIT ? OFFSET ?"
	} else {
		query += " ORDER BY dueAt, id LIMIT ? OFFSET ?"
	}
	args = append(args, limit, offset)
	rows, err := db.nftSQL().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []NFTGrant{}
	for rows.Next() {
		var raw string
		var grant NFTGrant
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &grant); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}
func (db *Database) NFTProfile(ctx context.Context, identity string) (NFTProfile, error) {
	var profile NFTProfile
	err := db.nftSQL().QueryRowContext(ctx, "SELECT firstLoginAt, welcomeNFTDeliveredAt FROM local_profiles WHERE identityKey = ?", identity).Scan(&profile.FirstLoginAt, &profile.WelcomeNFTDeliveredAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	profile.WelcomeNFTDelivered = profile.WelcomeNFTDeliveredAt != 0
	return profile, err
}
func (db *Database) NFTRelease(ctx context.Context, owner string) error {
	_, err := db.nftSQL().ExecContext(ctx, "UPDATE auto_nft_control SET leaseOwner = '', leaseUntil = 0 WHERE id = 1 AND leaseOwner = ?", owner)
	return err
}
func (db *Database) NFTCounts(ctx context.Context) (map[string]int, error) {
	rows, err := db.nftSQL().QueryContext(ctx, "SELECT stage, COUNT(*) FROM auto_nft_grants GROUP BY stage")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var stage string
		var count int
		if err = rows.Scan(&stage, &count); err != nil {
			return nil, err
		}
		counts[stage] = count
	}
	return counts, rows.Err()
}
func (db *Database) NFTUpdate(ctx context.Context, owner string, update func(*NFTStore) error) error {
	db.nftMu.Lock()
	defer db.nftMu.Unlock()
	tx, err := db.nftSQL().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Lock before reading: SQLite obtains its writer lock; InnoDB locks this singleton row.
	if _, err = tx.ExecContext(ctx, "UPDATE auto_nft_control SET revision = revision + 1 WHERE id = 1"); err != nil {
		return err
	}
	var current string
	var expires, now int64
	clock := "SELECT CAST(strftime('%s', 'now') AS INTEGER)"
	if db.Engine == "mysql" {
		clock = "SELECT UNIX_TIMESTAMP()"
	}
	if err = tx.QueryRowContext(ctx, clock).Scan(&now); err != nil {
		return err
	}
	if owner != "" {
		if err = tx.QueryRowContext(ctx, "SELECT leaseOwner, leaseUntil FROM auto_nft_control WHERE id = 1").Scan(&current, &expires); err != nil {
			return err
		}
		if current != owner || expires <= now {
			return ErrNFTLease
		}
	}
	if err = update(&NFTStore{tx: tx, now: now}); err != nil {
		return err
	}
	return tx.Commit()
}
func (store *NFTStore) Config(ctx context.Context) (NFTConfig, error) {
	var config NFTConfig
	var raw string
	err := store.tx.QueryRowContext(ctx, "SELECT config FROM auto_nft_control WHERE id = 1").Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &config)
	}
	return config, err
}
func (store *NFTStore) FirstLogin(ctx context.Context, identity, network, address string) (bool, error) {
	var exists int
	err := store.tx.QueryRowContext(ctx, "SELECT 1 FROM local_profiles WHERE identityKey = ?", identity).Scan(&exists)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	_, err = store.tx.ExecContext(ctx, "INSERT INTO local_profiles (identityKey, network, address, firstLoginAt, welcomeNFTDeliveredAt) VALUES (?, ?, ?, ?, 0)", identity, network, address, store.now)
	return err == nil, err
}
func (store *NFTStore) Grant(ctx context.Context, id string) (NFTGrant, error) {
	var grant NFTGrant
	var raw string
	err := store.tx.QueryRowContext(ctx, "SELECT body FROM auto_nft_grants WHERE id = ?", id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &grant)
	}
	return grant, err
}
func (store *NFTStore) InsertGrant(ctx context.Context, grant NFTGrant) error {
	raw, err := json.Marshal(grant)
	if err != nil {
		return err
	}
	_, err = store.tx.ExecContext(ctx, "INSERT INTO auto_nft_grants (id, revision, identityKey, network, collection, signerKey, pending, stage, dueAt, issuanceDay, body) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)", grant.ID, grant.Revision, grant.Identity, grant.Config.Network, grant.Config.Contract, grant.Config.SignerIdentity, grant.Stage, grant.DueAt, grant.IssuanceDay, string(raw))
	return err
}
func (store *NFTStore) PendingSigner(ctx context.Context, grant NFTGrant) (bool, error) {
	var count int
	err := store.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM auto_nft_grants WHERE signerKey = ? AND pending = 1 AND id <> ?", grant.Config.SignerIdentity, grant.ID).Scan(&count)
	return count > 0, err
}
func (store *NFTStore) Reserve(ctx context.Context, grant *NFTGrant, limit int) error {
	if grant.IssuanceDay != "" {
		return nil
	}
	day := time.Unix(store.now, 0).UTC().Format("2006-01-02")
	var count int
	if err := store.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM auto_nft_grants WHERE issuanceDay = ?", day).Scan(&count); err != nil {
		return err
	}
	if limit > 0 && count >= limit {
		return errors.New("daily_limit")
	}
	grant.IssuanceDay = day
	return nil
}
func (store *NFTStore) SaveConfig(ctx context.Context, config NFTConfig) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = store.tx.ExecContext(ctx, "UPDATE auto_nft_control SET config = ? WHERE id = 1", string(raw))
	return err
}
func (store *NFTStore) SaveGrant(ctx context.Context, grant NFTGrant) error {
	previous := grant.Revision
	grant.Revision++
	raw, err := json.Marshal(grant)
	if err != nil {
		return err
	}
	pending := 0
	if grant.Transaction != nil {
		pending = 1
	}
	result, err := store.tx.ExecContext(ctx, "UPDATE auto_nft_grants SET revision = ?, pending = ?, stage = ?, dueAt = ?, issuanceDay = ?, body = ? WHERE id = ? AND revision = ?", grant.Revision, pending, grant.Stage, grant.DueAt, grant.IssuanceDay, string(raw), grant.ID, previous)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return errors.New("grant_changed")
	}
	if grant.DeliveredAt != 0 {
		_, err = store.tx.ExecContext(ctx, "UPDATE local_profiles SET welcomeNFTDeliveredAt = ? WHERE identityKey = ?", grant.DeliveredAt, grant.Identity)
	}
	return err
}
func (store *NFTStore) Journal(ctx context.Context, grant NFTGrant) error {
	raw, err := json.Marshal(grant.Transaction)
	if err != nil {
		return err
	}
	_, err = store.tx.ExecContext(ctx, "INSERT INTO auto_nft_transactions (grantID, txHash, body, createdAt) VALUES (?, ?, ?, ?)", grant.ID, grant.Transaction.Hash, string(raw), store.now)
	return err
}
func (db *Database) nftSQL() *sql.DB {
	if db.Engine == "mysql" {
		return db.mysql.database
	}
	return db.sqlite.database
}
