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
* **posts** - Posts that a user creates
  * **txHash** - TEXT (Primary Key)
  * **blockchain** - TEXT (Primary Key)
  * **fromAddr** - TEXT
  * **toAddr** - TEXT
  * **parentTxHash** - TEXT
  * **amount** - REAL
  * **timestamp** - INTEGER (Unix timestamp)
  * **data** - TEXT
  * **blockNumber** - INTEGER
* **profiles** - Profile information for a given user
  * **address** - TEXT (Primary Key)
  * **blockchain** - TEXT (Primary Key)
  * **name** - TEXT
  * **avatar** - TEXT
  * **banner** - TEXT
  * **description** - TEXT
  * **location** - TEXT
  * **website** - TEXT
  * **birthdate** - TEXT
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
  * **extension** - TEXT
  * **path** - TEXT
  * **unsafeNameB64** - TEXT
  * **addedDate** - INTEGER (Unix timestamp)
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
## SQLite Views and Schema
* **parsed_posts** - posts with parsed data
  * **txHash** - TEXT
  * **blockchain** - TEXT
  * **fromAddr** - TEXT
  * **toAddr** - TEXT
  * **parentTxHash** - TEXT
  * **amount** - REAL
  * **timestamp** - INTEGER (Unix timestamp)
  * **blockNumber** - INTEGER
  * **text** - TEXT


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
* **onchain_meta** - Metadata about the profile and server
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
  * **txHash** - TEXT 
  * **blockchain** - TEXT
  * **address** - TEXT
  * **key** - TEXT
  * **value** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)
* **onchain_follow** - Following and unfollowing of profiles
  * **txHash** - TEXT
  * **blockchain** - TEXT
  * **toAddr** - TEXT
  * **fromAddr** - TEXT
  * **timestamp** - INTEGER (Unix timestamp)