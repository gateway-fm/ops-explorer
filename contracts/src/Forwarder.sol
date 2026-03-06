// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice A simple contract that forwards ETH to a recipient, creating an internal transaction.
contract Forwarder {
    event Forwarded(address indexed from, address indexed to, uint256 amount);

    /// @notice Forward the sent ETH to the given recipient.
    function forward(address payable _to) external payable {
        require(msg.value > 0, "Must send ETH");
        (bool success, ) = _to.call{value: msg.value}("");
        require(success, "Forward failed");
        emit Forwarded(msg.sender, _to, msg.value);
    }

    receive() external payable {}
}
