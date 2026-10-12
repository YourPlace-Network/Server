# Welcome NFT

One Server binary; the existing cron runs the durable minter every minute, including when indexing is disabled. Configure it under **Settings > Content > NFT** as the server owner or configured NFT operator. Auto-Minting defaults off.

## Settings Setup

1. Sign in with the wallet and network used during server setup. Until an NFT operator is saved, this server owner can configure the feature without environment variables.
2. Open **Operator and Issuing Wallets**. Set the operator and each issuing wallet's sender. EVM wallets also need the collection contract and reviewed runtime code hash; Base and Ethereum chain IDs are inferred as mainnet (8453 and 1). Algorand requires its base64 genesis hash. Save. Public configuration is stored in the local operational database on both SQLite and MySQL, not in distributed snapshots.
3. Choose the welcome NFT from the operator's collection, set enabled chains and cost/quantity limits, and save. Configuration remains editable while Auto-Minting is off. Signing keys are not required to save settings; enabled grants wait for credentials when unavailable.

Saving operator/wallet registration pauses Auto-Minting and clears in-memory signing keys. Reprovision credentials and explicitly re-enable issuance. Changing the operator requires signing in as the new operator. Existing grants retain their recipient, sender, collection, and metadata; drain them before replacing issuing wallets or networks.

Only one operator network/address pair is saved. The network dropdown keeps separate address drafts while the form is open; switching networks does not copy addresses. Green/red field borders indicate local format/checksum validity, not ownership or contract safety. Registration saves require reachable configured nodes and matching EVM mainnet/runtime code or Algorand genesis/issuer authority. Invalid input or failed verification leaves the saved registration unchanged. Signing credentials are not required for these checks.

Registration precedence: **Gateway environment override > saved Settings > server owner**. `YOURPLACE_MINTER_REGISTRATION` is optional and only read in Gateway mode (`-g`). When present, its registration fields are read-only in Settings; removing it and restarting restores saved configuration. Limits, template selection, and queue controls remain UI-managed. Invalid overrides fail closed.

A fresh Gateway with no real server owner must first establish an owner through local server setup or receive a trusted deployment registration. The zero-address default and arbitrary visitors cannot claim operator access.

## Deployment Setup

1. Apply the Gateway IAM policy change in Infra before using the bootstrap workflow. No additional service or Make target is required.
2. Optionally set GitHub variable `YOURPLACE_MINTER_REGISTRATION` to override saved public registration. Its JSON contains `operatorNetwork`, `operatorAddress`, and `chains`. Each chain has `network` and `sender`; EVM chains also require `contract` and `codeHash`. Networks are `base`, `ethereum`, and `algorand`. Omit EVM `chainId`: Base uses `"8453"` and Ethereum uses `"1"`; explicitly supplied values must match. Algorand requires `chainId` containing its base64 genesis hash. Leave it unset to use Settings.
3. Review the deployed EVM runtime against the nonproxy YourPlace collectible implementation. It must have no issuer seizure, freeze, forced-burn, or upgrade powers. Set `codeHash` to the reviewed runtime's Keccak-256 hash, including `0x`. A hash copied from an unreviewed contract is not an ownership guarantee. The server refuses mismatched runtime code.
4. Set GitHub secret `YOURPLACE_MINTER_CREDENTIALS` to a JSON object keyed by network. Values are EVM private keys or the Algorand recovery phrase. Each must derive the registered sender. Use a dedicated funded gateway wallet and avoid other transactions from it while grants are pending.
5. If Session Manager is not installed on the runner, configure GitHub variables `SSM_PLUGIN_PACKAGE_URL` and `SSM_PLUGIN_PACKAGE_SHA256` for an approved HTTPS Debian package and its verified checksum.
6. Deploy Gateway and sign in as the operator. Under **Content > NFT > Welcome NFT**, choose an NFT from the wallet's collection, review its image/name/description, then select **Use this NFT**. Set cost limits, select chains, and enable Auto-Minting. You can create the original NFT with the existing profile uploader first.

The selected NFT is a template, not inventory: it stays in the operator's wallet. Each eligible first login receives a newly minted token using the same immutable metadata on the visitor's chain. Selection verifies ownership and pins metadata/media locally. It applies immediately to future grants on every enabled chain; queued grants keep their original template. Original token identity, issuer controls, and contract royalties are not copied into the new token's contract.

The picker uses the same collections supported by the profile gallery. Sources must have `ipfs://CID` JSON metadata with `name` and `image`, IPFS file-CID media, no ARC-3 `extra_metadata`, and media no larger than 64 MiB. Algorand sources must be supply-one ARC-3 assets. The uploader produces this metadata format. Selection works while Auto-Minting is off and does not require gateway signing keys. Asset labels are shortened to Algorand's limits; the metadata name and description remain unchanged.

Runtime wallet keys are sent through an IAM-authorized SSM tunnel to a dedicated host-loopback port and held in Server memory. They are not written to the database, Docker environment, SSM command parameters, or application logs. Host swap must be disabled; container core dumps are disabled. Treat host administrators, the runner, and other containers on that host as trusted. The bootstrap listener is not a public HTTP route.

After **every process restart**, signing pauses until the Gateway workflow is dispatched with `bootstrap_only=true`. Already-submitted transactions can still be confirmed. The usual deployment performs bootstrap automatically. Missing credentials do not disable login.

## Limits and Ownership

- Daily limit `0` means unlimited. A positive limit reserves at most that many new grants per UTC day across all chains. Excess grants wait; retries, confirmations, and delivery do not consume another allowance. This is a quantity cap, not a currency budget or proof of unique humans.
- The daily-limit label estimates USD minting cost using current fees and spot prices, refreshed once per minute while visible. It assumes all allowed mints use the most expensive enabled chain. EVM estimates include the contract mint fee, simulated gas with the minter's safety margin, and Base's L1 fee estimate; Algorand uses its creation transaction fee. Delivery, recipient funding, and locked balances are excluded. Missing templates, RPC failures, or unavailable prices show an unavailable estimate, not zero. Estimates never sign or broadcast transactions.
- Cost ceilings use **wei** on EVM chains and **microAlgos** on Algorand. They apply separately to mint and delivery. Base checks an L1-fee estimate with a safety margin as well as execution gas; L1 fee movement after signing cannot be strictly capped by a legacy transaction. Algorand delivery includes the sponsored top-up and group fees. The separate top-up cap must allow at least 200,000 microAlgos for a completely unfunded recipient. Issuer minimum-balance reserves also require funding.
- Algorand recipients accept through Pera. The gateway pays the required top-up and all group fees. The ASA has no manager, reserve, freeze, or clawback authority from creation. Asset destruction is permanently unavailable. Expired *unsigned* acceptance can be renewed for the same NFT.
- EVM mints go to the gateway, then transfer to the recipient. Successful finalized delivery completes the entitlement permanently, even if the recipient later transfers or burns it.

## Recovery and Backups

- First login is local to this database. Earlier sessions cannot be reconstructed; logins while disabled or on an unconfigured chain are recorded without a retroactive grant.
- Pausing globally or excluding a chain stops its new signatures/submissions, not confirmation checks. Grants keep their original sender, collection, and metadata. Current cost limits apply before new signing; an already-signed transaction cannot be edited. Drain outstanding grants before changing signer registration or removing credentials.
- Retry resumes a grant. It never creates a replacement for an uncertain transaction. Signed bytes and transaction IDs are journaled before submission. An unknown, expired, or externally replaced transaction requires chain reconciliation; do not delete its journal or reset its grant to force a new mint.
- Once the Algorand node advances beyond a journaled transaction's validity window, the grant stays in attention with its journal intact but releases the signer for other grants. Retry only reconciles that transaction; it never rebroadcasts an expired transaction or remints the NFT. Unresolved attention records that still block a signer are rechecked daily.
- Back up the **entire operational database**, including `local_profiles` and all `auto_nft_*` tables. Distributed snapshots intentionally exclude them and cannot restore first-login history. Pause and reconcile on-chain transactions before resuming an old backup. Database deletion loses local entitlement history.

Before enabling production issuance, exercise crash/restart, authorization, duplicate-login, fee-limit, Pera, and ownership scenarios in isolated staging or chain forks. Base/Ethereum signing remains mainnet-ID-only. A passing compile or UI check is not an on-chain acceptance test.

## Operator Access

- **NFT operator not configured:** No valid server owner or saved NFT operator exists. Complete local server setup; Gateway deployments can alternatively supply a public registration override.
- **Sign in:** Complete wallet login, not just wallet connection. After signing in, return to Settings > Content > NFT.
- **Wallet mismatch:** Sign in with the exact registered network and wallet. EVM address casing does not matter, but Base and Ethereum are separate identities.

Operator registration is public configuration, not a signing credential. No private key or recovery phrase is needed to access settings or choose the template. Missing runtime signing keys pause issuance only.

## Security Audit: 2026-09-27

- Algorand users must sign in again after this update. Sessions now retain the verified on-chain authorizer, including through cookie rotation. Each authenticated request rechecks it with a three-second timeout; an authority change invalidates the session. Node outages deny protected requests without revoking otherwise valid sessions. Multisignature and logic-signature login formats are unsupported.
- Settings writes recheck operator authority before committing. Removing a collection or its reviewed code hash prevents further signing/submission for its queued grants. Recipient approval RPC calls no longer hold the whole minter's lock; grant revisions prevent stale updates.
- NFT request bodies are capped before CSRF parsing. Receipt identity, numeric bounds, and opt-in/group/signature checks reject malformed inputs. Browser acceptance also checks the authenticated grant's genesis hash, issuer, asset, and recipient.
- RPC nodes are trusted infrastructure: consistency checks cannot authenticate fabricated chain history from a compromised provider. Use operator-controlled endpoints. A configured runtime hash is not a substitute for manually reviewing the deployed EVM contract's ownership and upgrade powers.
- Host administrators, deployment runners, and containers on the host remain trusted. SSM bootstrap and memory-only signing credentials are unchanged; no additional durable secrets are introduced.
- Isolated checks passed on SQLite and MariaDB (the MySQL engine): rekey/login/session handling, cookie rotation, RPC failure handling, operator handover, CSRF/body limits, and expired-grant signer release without broadcasts. Forty in-memory acceptance cases passed with a stubbed wallet signer. Go build/package checks, selected existing tests, TypeScript, and the frontend build passed; webpack retains two existing dependency warnings.
- Malformed EVM receipts (missing block number, null log, wrong transaction hash) retained the journal without advancement or broadcast. Desktop/mobile settings checks passed at 1440, 704, and 360 pixels. Full `go test ./...` remains blocked by existing integration prerequisites: snapshot CSRF/server access and the macOS helper socket; the helper test panics after its connection failure.
- No new automated test files or real transactions were used. Live wallet, deployed-contract, and full crash/restart acceptance remain release requirements.
