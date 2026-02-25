// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Burnable.sol";
import "@openzeppelin/contracts/token/common/ERC2981.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract YourPlaceCollectible is ERC721, ERC721Enumerable, ERC721URIStorage, ERC721Burnable, ERC2981, Ownable {
    uint256 private _nextTokenId;
    string private _contractURI;
    uint96 private _defaultRoyaltyBps;
    uint256 private _mintFee = 100000000000000; // 0.0001 ETH
    address private _platformFeeReceiver;

    event Minted(uint256 indexed tokenId, address indexed minter, string uri);

    constructor(
        string memory contractMetadataURI,
        uint96 defaultRoyaltyBps,
        address platformFeeReceiver
    ) ERC721("YourPlace", "YPC") Ownable(msg.sender) {
        require(platformFeeReceiver != address(0), "Invalid fee receiver");
        _contractURI = contractMetadataURI;
        _defaultRoyaltyBps = defaultRoyaltyBps;
        _platformFeeReceiver = platformFeeReceiver;
    }

    function contractURI() public view returns (string memory) {
        return _contractURI;
    }

    function mint(string memory uri) public payable returns (uint256) {
        require(msg.value == _mintFee, "Incorrect mint fee");
        uint256 tokenId = _nextTokenId++;
        _safeMint(msg.sender, tokenId);
        _setTokenURI(tokenId, uri);
        _setTokenRoyalty(tokenId, msg.sender, _defaultRoyaltyBps);
        emit Minted(tokenId, msg.sender, uri);
        (bool sent, ) = _platformFeeReceiver.call{value: _mintFee}("");
        require(sent, "Fee transfer failed");
        return tokenId;
    }

    function mintFee() public view returns (uint256) {
        return _mintFee;
    }

    function setContractURI(string memory newContractURI) public onlyOwner {
        _contractURI = newContractURI;
    }

    function setDefaultRoyaltyBps(uint96 newDefaultRoyaltyBps) public onlyOwner {
        _defaultRoyaltyBps = newDefaultRoyaltyBps;
    }

    function setMintFee(uint256 newMintFee) public onlyOwner {
        _mintFee = newMintFee;
    }
    function setPlatformFeeReceiver(address newReceiver) public onlyOwner {
        require(newReceiver != address(0), "Invalid fee receiver");
        _platformFeeReceiver = newReceiver;
    }
    function withdraw() public onlyOwner {
        uint256 balance = address(this).balance;
        require(balance > 0, "No ETH to withdraw");
        (bool sent, ) = msg.sender.call{value: balance}("");
        require(sent, "Withdraw failed");
    }

    // --- Required Overrides --- //
    function _increaseBalance(address account, uint128 value) internal override(ERC721, ERC721Enumerable) {
        super._increaseBalance(account, value);
    }

    function _update(address to, uint256 tokenId, address auth) internal override(ERC721, ERC721Enumerable) returns (address) {
        return super._update(to, tokenId, auth);
    }

    function supportsInterface(bytes4 interfaceId) public view override(ERC721, ERC721Enumerable, ERC721URIStorage, ERC2981) returns (bool) {
        return super.supportsInterface(interfaceId);
    }

    function tokenURI(uint256 tokenId) public view override(ERC721, ERC721URIStorage) returns (string memory) {
        return super.tokenURI(tokenId);
    }
}
