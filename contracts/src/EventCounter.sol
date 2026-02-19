// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

contract EventCounter {
    uint256 public number;

    event NumberSet(address indexed sender, uint256 oldNumber, uint256 newNumber);
    event NumberIncremented(address indexed sender, uint256 oldNumber, uint256 newNumber);

    constructor(uint256 _initialNumber) {
        number = _initialNumber;
        emit NumberSet(msg.sender, 0, _initialNumber);
    }

    function setNumber(uint256 newNumber) public {
        uint256 oldNumber = number;
        number = newNumber;
        emit NumberSet(msg.sender, oldNumber, newNumber);
    }

    function increment() public {
        uint256 oldNumber = number;
        number++;
        emit NumberIncremented(msg.sender, oldNumber, number);
    }
}
