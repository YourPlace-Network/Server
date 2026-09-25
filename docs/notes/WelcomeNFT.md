# Welcome NFT

One Server binary; the existing cron runs the durable minter every minute, including when indexing is disabled. Configure it under **Settings > Content > NFT** as the registered operator. Auto-Minting defaults off.

## Deployment Setup

1. Apply the Gateway IAM policy change in Infra before using the bootstrap workflow. No additional service or Make target is required.
2. Set GitHub variable `YOURPLACE_MINTER_REGISTRATION` to public JSON with `operatorNetwork`, `operatorAddress`, and `chains`. Each chain has `network`, `chainId`, and `sender`; EVM chains also require `contract` and `codeHash`. Networks are `base`, `ethereum`, and `algorand`. EVM chain IDs are decimal strings; Algorand uses its base64 genesis hash.
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
- Cost ceilings use **wei** on EVM chains and **microAlgos** on Algorand. They apply separately to mint and delivery. Base checks an L1-fee estimate with a safety margin as well as execution gas; L1 fee movement after signing cannot be strictly capped by a legacy transaction. Algorand delivery includes the sponsored top-up and group fees. The separate top-up cap must allow at least 200,000 microAlgos for a completely unfunded recipient. Issuer minimum-balance reserves also require funding.
- Algorand recipients accept through Pera. The gateway pays the required top-up and all group fees. The ASA has no manager, reserve, freeze, or clawback authority from creation. Asset destruction is permanently unavailable. Expired *unsigned* acceptance can be renewed for the same NFT.
- EVM mints go to the gateway, then transfer to the recipient. Successful finalized delivery completes the entitlement permanently, even if the recipient later transfers or burns it.

## Recovery and Backups

- First login is local to this database. Earlier sessions cannot be reconstructed; logins while disabled or on an unconfigured chain are recorded without a retroactive grant.
- Pausing globally or excluding a chain stops its new signatures/submissions, not confirmation checks. Grants keep their original sender, collection, and metadata. Current cost limits apply before new signing; an already-signed transaction cannot be edited. Drain outstanding grants before changing signer registration or removing credentials.
- Retry resumes a grant. It never creates a replacement for an uncertain transaction. Signed bytes and transaction IDs are journaled before submission. An unknown, expired, or externally replaced transaction requires chain reconciliation; do not delete its journal or reset its grant to force a new mint.
- Back up the **entire operational database**, including `local_profiles` and all `auto_nft_*` tables. Distributed snapshots intentionally exclude them and cannot restore first-login history. Pause and reconcile on-chain transactions before resuming an old backup. Database deletion loses local entitlement history.

Before enabling production issuance, exercise the approved crash/restart, authorization, duplicate-login, fee-limit, Pera, and ownership scenarios on funded test networks. A passing compile or UI check is not an on-chain acceptance test.
