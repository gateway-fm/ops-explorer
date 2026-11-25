// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract SelfRef {
    address private immutable self = address(this);
    uint256 public immutable seed;

    constructor(uint256 _seed) {
        seed = _seed;
    }

    function onlyProxy() external view returns (bool) {
        return address(this) != self;
    }

    function me() external view returns (address) {
        return self;
    }
}
