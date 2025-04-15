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
* **ipfsFiles** - Files that the user uploads to IPFS
  * **fileUUID** - TEXT (Primary Key)
  * **cid** - TEXT
  * **addedDate** - INTEGER (Unix timestamp)
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
* **onchain_post** - Posts that a user creates
  * **txHash** - TEXT (Primary Key)
  * **blockchain** - TEXT (Primary Key)
  * **fromAddr** - TEXT
  * **toAddr** - TEXT
  * **parentTxHash** - TEXT
  * **amount** - REAL
  * **timestamp** - INTEGER (Unix timestamp)
  * **data** - TEXT
  * **blockNumber** - INTEGER
* **onchain_attachment** - Files that the user uploads to a post
  * **txHash** - TEXT (Primary Key)
  * **blockchain** - TEXT (Primary Key)
  * **address** - TEXT
  * **name** - TEXT
  * **contentType** - TEXT
  * **size** - INTEGER
  * **timestamp** - INTEGER (Unix timestamp)
* **onchain_meta** - Metadata about profiles and their server
  * **blockchain** - TEXT (Primary Key)
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
* **onchain_block** - Blocking and unblocking of content
  * **txHash** - TEXT  (Primary Key)
  * **blockchain** - TEXT (Primary Key)
  * **address** - TEXT
  * **key** - TEXT
  * **value** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)
* **onchain_follow** - Following and unfollowing of profiles
  * **txHash** - TEXT (Primary Key)
  * **blockchain** - TEXT (Primary Key)
  * **followerAddress** - TEXT
  * **followerBlockchain** - TEXT
  * **followeeAddress** - TEXT
  * **followeeBlockchain** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)