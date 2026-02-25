# YourPlace Collectible - Shared ERC-721 Contract (Ethereum Mainnet)

## Overview

`YourPlaceCollectible.sol` is a shared ERC-721 NFT contract deployed once on Ethereum mainnet. All YourPlace users mint collectibles through this single contract, which provides:

- **ERC721Enumerable** - On-chain token discovery via `balanceOf()` + `tokenOfOwnerByIndex()`
- **ERC721URIStorage** - Per-token metadata URIs pointing to IPFS JSON
- **ERC721Burnable** - Token owners can burn their own NFTs
- **ERC2981** - Royalty info (creator automatically set as royalty receiver at mint time)
- **contractURI()** - Collection-level metadata for marketplace display (OpenSea, Rarible, etc.)

## Architecture

```
User clicks "Create Collectible" in YourPlace UI
    |
    +-- Media file uploaded to IPFS -> mediaCID
    +-- Metadata JSON uploaded to IPFS -> metadataCID
    |       { name, description, image: "ipfs://mediaCID", image_mimetype, royalty_percentage }
    |
    +-- mint("ipfs://metadataCID") called on shared contract
            -> tokenId assigned, caller becomes owner + royalty receiver
```

## Public Functions

| Function | Access | Description |
|----------|--------|-------------|
| `mint(string uri)` | Anyone (payable) | Mint a new NFT. Requires 0.0001 ETH platform fee. Caller becomes owner and royalty receiver. Returns tokenId. |
| `mintFee()` | View | Returns the current platform mint fee in wei. |
| `burn(uint256 tokenId)` | Token owner | Destroy an NFT permanently. |
| `safeTransferFrom(from, to, tokenId)` | Token owner / approved | Transfer an NFT to another address. |
| `balanceOf(address owner)` | View | Count of NFTs owned by an address. |
| `tokenOfOwnerByIndex(address, uint256)` | View | Get tokenId at index for an owner. |
| `tokenURI(uint256 tokenId)` | View | Get the IPFS metadata URI for a token. |
| `ownerOf(uint256 tokenId)` | View | Get the current owner of a token. |
| `contractURI()` | View | Collection-level metadata URI. |
| `royaltyInfo(tokenId, salePrice)` | View | ERC2981 royalty info for marketplaces. |
| `setContractURI(string)` | Owner only | Update collection metadata URI. |
| `setDefaultRoyaltyBps(uint96)` | Owner only | Update default royalty basis points for future mints. |
| `setMintFee(uint256)` | Owner only | Update the platform mint fee in wei. |
| `setPlatformFeeReceiver(address)` | Owner only | Update the address that receives platform mint fees. |
| `withdraw()` | Owner only | Withdraw any ETH stuck in the contract. |

## Prerequisites

Install Foundry:

```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

## Setup

```bash
cd src/solidity/ethereum/nft/
forge init --no-git --force .
forge install OpenZeppelin/openzeppelin-contracts --no-git
rm -rf lib/openzeppelin-contracts/fv/
```

Add to `foundry.toml`:
```toml
[profile.default]
src = "."
out = "out"
libs = ["lib"]
remappings = ["@openzeppelin/contracts/=lib/openzeppelin-contracts/contracts/"]
via_ir = true
```

## Compile

```bash
forge build
```

## Test

Create a test file `YourPlaceCollectible.t.sol`:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "./YourPlaceCollectible.sol";

contract YourPlaceCollectibleTest is Test {
    YourPlaceCollectible.sol:YourPlaceCollectible public nft;
    address public alice = address(0x1);
    address public bob = address(0x2);

    function setUp() public {
        nft = new YourPlaceCollectible("ipfs://collection-metadata", 500, address(0xFEE)); // 5% royalty
    }

    function testMint() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        uint256 tokenId = nft.mint{value: 0.0001 ether}("ipfs://QmTest123");
        assertEq(tokenId, 0);
        assertEq(nft.ownerOf(0), alice);
        assertEq(nft.tokenURI(0), "ipfs://QmTest123");
        assertEq(nft.balanceOf(alice), 1);
    }

    function testMintSetsRoyalty() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        nft.mint{value: 0.0001 ether}("ipfs://QmTest123");
        (address receiver, uint256 amount) = nft.royaltyInfo(0, 10000);
        assertEq(receiver, alice);
        assertEq(amount, 500); // 5% of 10000
    }

    function testMintInsufficientFee() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        vm.expectRevert("Incorrect mint fee");
        nft.mint{value: 0.00009 ether}("ipfs://QmTest123");
    }

    function testMintRejectsOverpayment() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        vm.expectRevert("Incorrect mint fee");
        nft.mint{value: 0.001 ether}("ipfs://QmTest123");
    }

    function testBurn() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        uint256 tokenId = nft.mint{value: 0.0001 ether}("ipfs://QmTest123");
        vm.prank(alice);
        nft.burn(tokenId);
        assertEq(nft.balanceOf(alice), 0);
    }

    function testBurnOnlyOwner() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        uint256 tokenId = nft.mint{value: 0.0001 ether}("ipfs://QmTest123");
        vm.prank(bob);
        vm.expectRevert();
        nft.burn(tokenId);
    }

    function testTransfer() public {
        vm.deal(alice, 1 ether);
        vm.prank(alice);
        uint256 tokenId = nft.mint{value: 0.0001 ether}("ipfs://QmTest123");
        vm.prank(alice);
        nft.safeTransferFrom(alice, bob, tokenId);
        assertEq(nft.ownerOf(tokenId), bob);
        assertEq(nft.balanceOf(alice), 0);
        assertEq(nft.balanceOf(bob), 1);
    }

    function testEnumeration() public {
        vm.deal(alice, 1 ether);
        vm.startPrank(alice);
        nft.mint{value: 0.0001 ether}("ipfs://QmA");
        nft.mint{value: 0.0001 ether}("ipfs://QmB");
        nft.mint{value: 0.0001 ether}("ipfs://QmC");
        vm.stopPrank();
        assertEq(nft.balanceOf(alice), 3);
        assertEq(nft.tokenOfOwnerByIndex(alice, 0), 0);
        assertEq(nft.tokenOfOwnerByIndex(alice, 1), 1);
        assertEq(nft.tokenOfOwnerByIndex(alice, 2), 2);
    }

    function testContractURI() public view {
        assertEq(nft.contractURI(), "ipfs://collection-metadata");
    }

    function testSetContractURIOnlyOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        nft.setContractURI("ipfs://new-metadata");
    }
}
```

Run tests:
```bash
forge test -vvv
```

## Derive Private Key from Seed Phrase

Use Foundry's `cast` to derive the deployer private key from a BIP-39 mnemonic seed phrase:

```bash
cast wallet private-key "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"
```

This derives the private key at the default BIP-44 derivation path (`m/44'/60'/0'/0/0`). To use a different account index:

```bash
cast wallet private-key "word1 word2 ... word12" --mnemonic-index 1
```

Then export it for use in the deploy commands below:

```bash
export PRIVATE_KEY=$(cast wallet private-key "word1 word2 ... word12")
```

## Deploy

### Sepolia (Testnet)

```bash
export PRIVATE_KEY=<deployer-private-key>
export SEPOLIA_RPC=https://rpc.sepolia.org

forge create \
    --rpc-url $SEPOLIA_RPC \
    --private-key $PRIVATE_KEY \
    --broadcast \
    YourPlaceCollectible.sol:YourPlaceCollectible \
    --constructor-args "ipfs://<collection-metadata-cid>" 500 <platform-fee-receiver-address>
```

### Ethereum Mainnet

```bash
export PRIVATE_KEY=<deployer-private-key>
export ETH_RPC=https://eth.llamarpc.com

forge create \
    --rpc-url $ETH_RPC \
    --private-key $PRIVATE_KEY \
    --broadcast \
    YourPlaceCollectible.sol:YourPlaceCollectible \
    --constructor-args "ipfs://<collection-metadata-cid>" 500 <platform-fee-receiver-address>
```

Constructor arguments:
- `contractMetadataURI` - IPFS URI to a JSON file with collection metadata (name, description, image)
- `defaultRoyaltyBps` - Default royalty in basis points (500 = 5%, 1000 = 10%)
- `platformFeeReceiver` - Address that receives the platform mint fee

### Collection Metadata JSON

Upload this JSON to IPFS before deploying. The `contractURI()` function returns this URI for marketplace collection pages.

```json
{
    "name": "YourPlace",
    "description": "Collectibles from YourPlace - Distributed Social Media",
    "image": "ipfs://<yourplace-logo-cid>",
    "external_link": "https://yourplace.network"
}
```

## Verify on Etherscan

```bash
forge verify-contract \
    --chain mainnet \
    --compiler-version v0.8.20 \
    <deployed-contract-address> \
    YourPlaceCollectible.sol:YourPlaceCollectible \
    --constructor-args $(cast abi-encode "constructor(string,uint96,address)" "ipfs://<cid>" 500 <platform-fee-receiver-address>)
```

## After Deployment

1. Copy the deployed contract address
2. Update `src/typescript/util/blockchain/ethereum.ts` with the deployed contract address and ABI (following the same pattern as `base.ts`)
3. Rebuild the frontend: `npx webpack --config src/typescript/webpack.prod.js`

## Gas Costs (Ethereum Mainnet Estimates)

| Operation | Estimated Gas | Estimated Cost (at 30 gwei) |
|-----------|--------------|----------------------------|
| Mint | ~150,000 | ~$1.00-5.00 |
| Transfer | ~80,000 | ~$0.50-2.50 |
| Burn | ~60,000 | ~$0.40-2.00 |

Gas costs on Ethereum mainnet are significantly higher than Base L2. Costs vary with network congestion and ETH price.

## Security Notes

- The contract uses OpenZeppelin v5 audited implementations
- `mint()` is permissionless - any address can mint
- `mint()` requires exact fee payment (no overpayment/refund) and follows the Checks-Effects-Interactions pattern
- Only token owners (or approved operators) can burn or transfer
- `setContractURI()`, `setDefaultRoyaltyBps()`, `setMintFee()`, `setPlatformFeeReceiver()`, and `withdraw()` are restricted to the contract deployer (Ownable)
- `withdraw()` is a safety net to recover ETH sent to the contract outside of `mint()` (e.g., via `selfdestruct`)
- No pause, blacklist, or admin mint functions - the contract is intentionally minimal
- Royalties are set per-token at mint time and cannot be changed after minting
