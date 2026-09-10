// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

/// @title MockERC1155
/// @notice Minimal, dependency-free ERC-1155 (+ MetadataURI + ERC165) used for
///         testing the block explorer's multi-token features. Mirrors the
///         hand-rolled style of MockNFT.sol — no OpenZeppelin, no imports.
/// @dev    Implements just enough of the standard for an indexer to detect the
///         contract as ERC-1155 (via the TransferSingle / TransferBatch event
///         signatures) and to read per-id metadata via uri(). Each id carries
///         its own metadata URI so the explorer can decode artwork offline.
///         A single id is fungible: mint a quantity > 1 to create an edition,
///         or quantity == 1 for a 1-of-1. Minting is gated to the deployer.
contract MockERC1155 {
    address public owner;
    string public name;
    string public symbol;

    // balances[id][account] = quantity held.
    mapping(uint256 => mapping(address => uint256)) private _balances;
    mapping(address => mapping(address => bool)) private _operatorApprovals;
    mapping(uint256 => string) private _uris;

    event TransferSingle(
        address indexed operator,
        address indexed from,
        address indexed to,
        uint256 id,
        uint256 value
    );
    event TransferBatch(
        address indexed operator,
        address indexed from,
        address indexed to,
        uint256[] ids,
        uint256[] values
    );
    event ApprovalForAll(address indexed account, address indexed operator, bool approved);
    event URI(string value, uint256 indexed id);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "1155: not owner");
        _;
    }

    constructor(string memory _name, string memory _symbol) {
        name = _name;
        symbol = _symbol;
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "1155: owner zero");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    // ----- ERC165 -----

    /// @notice Advertises ERC165, ERC1155, and ERC1155MetadataURI.
    function supportsInterface(bytes4 interfaceId) external pure returns (bool) {
        return
            interfaceId == 0x01ffc9a7 || // ERC165
            interfaceId == 0xd9b67a26 || // ERC1155
            interfaceId == 0x0e89341c;   // ERC1155MetadataURI
    }

    // ----- ERC1155MetadataURI -----

    function uri(uint256 id) external view returns (string memory) {
        return _uris[id];
    }

    // ----- ERC1155 core -----

    function balanceOf(address account, uint256 id) public view returns (uint256) {
        require(account != address(0), "1155: zero address");
        return _balances[id][account];
    }

    function balanceOfBatch(address[] calldata accounts, uint256[] calldata ids)
        external
        view
        returns (uint256[] memory)
    {
        require(accounts.length == ids.length, "1155: length mismatch");
        uint256[] memory batch = new uint256[](accounts.length);
        for (uint256 i = 0; i < accounts.length; i++) {
            batch[i] = balanceOf(accounts[i], ids[i]);
        }
        return batch;
    }

    function setApprovalForAll(address operator, bool approved) external {
        _operatorApprovals[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    function isApprovedForAll(address account, address operator) public view returns (bool) {
        return _operatorApprovals[account][operator];
    }

    /// @notice Mint `amount` of token `id` to `to`, setting its metadata URI on
    ///         first mint. Fungible: minting more of an existing id increases
    ///         supply rather than creating a new token.
    function mint(address to, uint256 id, uint256 amount, string calldata tokenURI) external onlyOwner {
        require(to != address(0), "1155: mint to zero");
        _balances[id][to] += amount;
        if (bytes(tokenURI).length > 0) {
            _uris[id] = tokenURI;
            emit URI(tokenURI, id);
        }
        emit TransferSingle(msg.sender, address(0), to, id, amount);
    }

    /// @notice Batch variant of mint: emits a single TransferBatch.
    function mintBatch(
        address to,
        uint256[] calldata ids,
        uint256[] calldata amounts,
        string[] calldata uris
    ) external onlyOwner {
        require(to != address(0), "1155: mint to zero");
        require(ids.length == amounts.length && ids.length == uris.length, "1155: length mismatch");
        for (uint256 i = 0; i < ids.length; i++) {
            _balances[ids[i]][to] += amounts[i];
            if (bytes(uris[i]).length > 0) {
                _uris[ids[i]] = uris[i];
                emit URI(uris[i], ids[i]);
            }
        }
        emit TransferBatch(msg.sender, address(0), to, ids, amounts);
    }

    /// @dev The receiver-hook check (onERC1155Received) is intentionally
    ///      omitted; this is a test fixture, not production code.
    function safeTransferFrom(address from, address to, uint256 id, uint256 amount, bytes calldata) external {
        require(from == msg.sender || isApprovedForAll(from, msg.sender), "1155: not authorized");
        require(to != address(0), "1155: transfer to zero");
        require(_balances[id][from] >= amount, "1155: insufficient balance");
        _balances[id][from] -= amount;
        _balances[id][to] += amount;
        emit TransferSingle(msg.sender, from, to, id, amount);
    }

    function safeBatchTransferFrom(
        address from,
        address to,
        uint256[] calldata ids,
        uint256[] calldata amounts,
        bytes calldata
    ) external {
        require(from == msg.sender || isApprovedForAll(from, msg.sender), "1155: not authorized");
        require(to != address(0), "1155: transfer to zero");
        require(ids.length == amounts.length, "1155: length mismatch");
        for (uint256 i = 0; i < ids.length; i++) {
            require(_balances[ids[i]][from] >= amounts[i], "1155: insufficient balance");
            _balances[ids[i]][from] -= amounts[i];
            _balances[ids[i]][to] += amounts[i];
        }
        emit TransferBatch(msg.sender, from, to, ids, amounts);
    }
}
