# Blockchain API
Communicating with YourPlace on the blockchain requires a unified API.
JSON was chosen to provide a succinct, human-readable mechanism to send
messages via blockchain. These messages will be what is stored on the
blockchain and consumed by the world. Both YourPlace indexers and servers
will need to understand this layout to be able to the network.

### Structure
`yp/1/p:{"p":"payload"}` = YourPlace/Version/Action:{JSON}

### Burn Address Protocol
All YourPlace transactions that broadcast user data (posts, metadata, settings, etc.)
are sent to the **burn address**: `0x0000000000000000000000000000000000000000` (0x0).
This signals to the world that the transaction contains protocol data meant for public
consumption, not a transfer between accounts. The indexer identifies YourPlace
transactions by checking if the recipient is the burn address.

### Implementation Status
🚢 = Currently implemented and shipped in indexer and client
Actions without 🚢 are documented but not yet implemented or shipped to users

### Actions Prefixes
Actions are the 1 to 5 long character code between the `/` and `:` in the structure of the message. The first character is the "prefix" and the remaining characters are the "postfix". This established an action category (prefix) and specific action tag (postfix) for every API payload.
- p = post
- r = reaction
- f = follow
- m = metadata
- b = block
- s = settings

### Payload Tags
Payload tags are special tags that the user can type themselves that exist in the post body. Those tags are the basis of communities and topics on YourPlace, and allow people to congregate around a topic or perform a specific action.
- \# = hashtag
- $ = cashtag
- @ = mention
- $@ = tip
- \<\> = NFT

# JSON API
### Enrollment (to burn address 0x0)
- `yp/1/e:{"e":"url"}` - Enroll/register server endpoint URL

### Posting (to burn address 0x0)
- `yp/1/p:{"p":"payload"}` - Post a message 🚢
- `yp/1/pa:{"p":"payload","a":[["ipfs://CID/path.exe","application/vnd.microsoft.portable-executable",4096,"filename.exe"],["ipfs://CID2/path2.jpg","image/jpeg",2048,"filename.jpg"]]}` - Post a message with file attachment(s) [{path, mimetype, size, filename}] 🚢
- `yp/1/pr:{"txh":"txnHash"}` - Repost a post
- `yp/1/pry:{"txh":"txnHash","p":"payload"}` - Reply to a post (to original poster)
- `yp/1/prp:{"txh":"txnHash","p":"payload"}` - Repost a post with a message
- `yp/1/pe:{"txh":"txnHash","p":"payload"}` - Edit a post
- `yp/1/par:{"txh":"txnHash"}` - Archive a post

### Reaction (to burn address 0x0)
- `yp/1/rl:{"txh":"txnHash"}` - Like a post
- `yp/1/rdl:{"txh":"txnHash"}` - Dislike a post

### Following (to burn address 0x0)
- `yp/1/f:{"a":"address", "b":"blockchain"}` - Follow an address at a blockchain 🚢
- `yp/1/fu:{"a":"address", "b":"blockchain"}` - Unfollow an address at a blockchain 🚢
- `yp/1/fh:{"h#":"#hashtag"}` - Follow a hashtag on all blockchains
- `yp/1/fuh:{"h#":"#hashtag"}` - Unfollow a hashtag on all blockchains

### Metadata (to burn address 0x0)
- `yp/1/mn:{"n":"name"}` - Update name 🚢
- `yp/1/ma:{"a":"ipfs://CID"}` - Update avatar Img (IPFS) 🚢
- `yp/1/mb:{"b":"ipfs://CID"}` - Update banner Img (IPFS) 🚢
- `yp/1/mbd:{"bd":"0000000000"}` - Update birth date (Unix timestamp) 🚢
- `yp/1/ms:{"s":"12.34.56.78:42424"}` - Update server location (IP/DNS:PORT)
- `yp/1/ml:{"l":"location"}` - Update user specified location 🚢
- `yp/1/mw:{"w":"website"}` - Update user specified website 🚢
- `yp/1/md:{"d":"description"}` - Update profile description 🚢
- `yp/1/mao:{"ao":"true"}` = Set profile to adults only
- `yp/1/mbot:{"bot":"true"}` = Set profile to bot
### Block (to burn address 0x0)
- `yp/1/bl:{"l":"https://block.list.com/list.json"}` - Subscribe to block list (HTTPS/IPFS)
- `yp/1/ba:{"a":"address"}` - Block an address
- `yp/1/bh:{"h#":"#hashtag"}` - Block a hashtag
- `yp/1/bw:{"w":"word"}` - Block a word
- `yp/1/br:{"r":"^regex$"}` - Block a regex
- `yp/1/bul:{"l":"https://block.list.com/list.json"}` - Unblock list (HTTPS/IPFS)
- `yp/1/bua:{"a":"address"}` - Unblock an address
- `yp/1/buh:{"h#":"#hashtag"}` - Unblock a hashtag
- `yp/1/buw:{"w":"word"}` - Unblock a word
- `yp/1/bur:{"r":"^regex$"}` - Unblock a regex
### Settings (to burn address 0x0)
- `yp/1/sa:{"a":"yourplace"}` - Set avatar preference to yourplace
- `yp/1/sa:{"a":"nfdomains"}` - Set avatar preference to nfdomains
- `yp/1/sd:{"d":"yourplace"}` - Set description preference to yourplace
- `yp/1/sd:{"d":"nfdomains"}` - Set description preference to nfdomains
- `yp/1/ss:{"s":"false"}` - Set saved post preference to false
- `yp/1/sap:{"ap":"false"}` - Set autoplay preference to false