// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

/// @title Mock Tether USD
/// @notice Minimal ERC20 used for testing the block explorer's token features.
/// @dev Decimals are 6 to match real USDT. Mint is gated to the deployer so
///      static analysis doesn't flag this as a public-mint vulnerability;
///      the companion deploy-mock-usdt.sh script uses the same Anvil key for
///      both deployment and subsequent funding, so the gate doesn't get in
///      the way of testing.
contract MockUSDT {
    string public constant name = "Tether USD";
    string public constant symbol = "USDT";
    uint8 public constant decimals = 6;

    uint256 public totalSupply;
    address public owner;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "USDT: not owner");
        _;
    }

    constructor(uint256 _initialSupply) {
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
        if (_initialSupply > 0) {
            _mint(msg.sender, _initialSupply);
        }
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "USDT: owner zero");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        uint256 allowed = allowance[from][msg.sender];
        require(allowed >= amount, "USDT: allowance");
        if (allowed != type(uint256).max) {
            allowance[from][msg.sender] = allowed - amount;
        }
        _transfer(from, to, amount);
        return true;
    }

    function _mint(address to, uint256 amount) internal {
        require(to != address(0), "USDT: mint to zero");
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function _transfer(address from, address to, uint256 amount) internal {
        require(to != address(0), "USDT: transfer to zero");
        uint256 bal = balanceOf[from];
        require(bal >= amount, "USDT: balance");
        unchecked {
            balanceOf[from] = bal - amount;
            balanceOf[to] += amount;
        }
        emit Transfer(from, to, amount);
    }
}
