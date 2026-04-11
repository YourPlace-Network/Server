# YourPlace Database Schema
The YourPlace server uses an embedded database engine to store information in a durable format.

## SQLite Tables and Schema
* **meta** - Metadata about the server and blockchain and server owner
  * **algoPriceUSD** - REAL
  * **ethPriceUSD** - REAL
  * **installedDate** - INTEGER (Unix timestamp)
  * **publicIP** - TEXT
  * **walletAddress** - TEXT
  * **ypPortOpen** - INTEGER (0 or 1)
* **settings** - Local server settings
  * **algodURL** - TEXT
  * **algodToken** - TEXT
  * **algoIndexerURL** - TEXT
  * **algoIndexerToken** - TEXT
  * **baseFullNode** - INTEGER (0 or 1)
  * **baseThrottle** - INTEGER
  * **baseURL** - TEXT
  * **cryptoSeed** - TEXT
  * **defaultSaved** - INTEGER (0 or 1)
  * **historyDays** - INTEGER
  * **ipfsPort** - INTEGER
  * **uploadDirectory** - TEXT
* **files** - Files that the user uploads
  * **fileUUID** - TEXT (Primary Key)
  * **fileHash** - TEXT
  * **mimeType** - TEXT
  * **unsafeNameB64** - TEXT
  * **size** - INTEGER
  * **addedDate** - INTEGER (Unix timestamp)
  * **cid** - TEXT
  * **fileURL** - TEXT
  * **txHash** - TEXT
  * **source** - TEXT
* **postsBackfill** - Status of backfill jobs running on the server
  * **uuid** - TEXT (Primary Key)
  * **blockchain** - TEXT
  * **headBlock** - INTEGER
  * **status** - TEXT
  * **tailBlock** - INTEGER
  * **timestamp** - INTEGER (Unix timestamp)
* **authNonce** - Nonces used for user authentication
  * **nonce** - TEXT (Primary Key)
  * **status** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)
* **authExpired** - List of cookies that are expired / invalid
  * **uuid** - TEXT (Primary Key)
  * **status** - TEXT
* **loginNonce** - Nonce used for user login
  * **nonce** - TEXT (Primary Key)
  * **domain** - TEXT
  * **expiration** - INTEGER (Unix timestamp)
  * **nonceHash** - TEXT

## Onchain Tables and Schema
Each supported blockchain (Base, Algorand, Ethereum) has its own set of chain-specific tables, prefixed with `onchain_<chain>_`. Because the blockchain is already encoded in the table name, the chain-specific tables do not carry a redundant `blockchain` column. The schemas below apply identically to each chain's variant (e.g. `onchain_base_post`, `onchain_algorand_post`, `onchain_ethereum_post`).

* **onchain_\<chain\>_post** - Posts that a user creates
  * **txHash** - TEXT (Primary Key)
  * **fromAddr** - TEXT
  * **toAddr** - TEXT
  * **parentTxHash** - TEXT
  * **amount** - REAL
  * **timestamp** - INTEGER (Unix timestamp)
  * **data** - TEXT
  * **blockNumber** - INTEGER
* **onchain_\<chain\>_meta** - Metadata about profiles and their server
  * **address** - TEXT (Primary Key)
  * **name** - TEXT
  * **avatar** - TEXT
  * **description** - TEXT
  * **location** - TEXT
  * **banner** - TEXT
  * **website** - TEXT
  * **birthdate** - TEXT
  * **server** - TEXT
  * **blockchainTimestamp** - INTEGER (Unix timestamp)
  * **addressTimestamp** - INTEGER (Unix timestamp)
  * **nameTimestamp** - INTEGER (Unix timestamp)
  * **avatarTimestamp** - INTEGER (Unix timestamp)
  * **locationTimestamp** - INTEGER (Unix timestamp)
  * **bannerTimestamp** - INTEGER (Unix timestamp)
  * **websiteTimestamp** - INTEGER (Unix timestamp)
  * **birthdateTimestamp** - INTEGER (Unix timestamp)
  * **serverTimestamp** - INTEGER (Unix timestamp)
* **onchain_\<chain\>_block** - Blocking and unblocking of content. Cross-chain blocking is supported via the per-row `blockerBlockchain` / `blockeeBlockchain` fields.
  * **txHash** - TEXT (Primary Key)
  * **blockerAddress** - TEXT
  * **blockerBlockchain** - TEXT
  * **blockeeAddress** - TEXT
  * **blockeeBlockchain** - TEXT
  * **key** - TEXT
  * **value** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)
* **onchain_\<chain\>_comment** - Comments on posts
  * **txHash** - TEXT (Primary Key)
  * **fromAddress** - TEXT
  * **parentTxHash** - TEXT
  * **parentType** - TEXT (post or comment)
  * **amount** - REAL
  * **timestamp** - INTEGER (Unix timestamp)
  * **data** - TEXT
* **onchain_\<chain\>_follow** - Following and unfollowing of profiles. Cross-chain follows are supported via the per-row `followerBlockchain` / `followeeBlockchain` fields.
  * **txHash** - TEXT (Primary Key)
  * **followerAddress** - TEXT
  * **followerBlockchain** - TEXT
  * **followeeAddress** - TEXT
  * **followeeBlockchain** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)
* **onchain_\<chain\>_reaction** - Reactions on posts and comments
  * **txHash** - TEXT (Primary Key)
  * **fromAddress** - TEXT
  * **targetTxHash** - TEXT
  * **targetType** - TEXT (post or comment)
  * **reactionType** - TEXT (like, dislike, or emoji character)
  * **timestamp** - INTEGER (Unix timestamp)