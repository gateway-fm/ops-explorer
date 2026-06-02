// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice Fixtures that produce a rich, multi-level call trace in a single tx,
/// exercising CALL / STATICCALL / nesting / a caught revert — used to build and
/// verify the explorer's call-trace UI.
contract Leaf {
    uint256 public x;

    function touch(uint256 v) external payable {
        x = v;
    }

    function boom() external pure {
        revert("leaf boom");
    }

    receive() external payable {}
}

contract TraceDemo {
    event Ran(address indexed leaf);

    /// @notice Produces children: [0] CALL+value, [1] STATICCALL, [2] CALL with a
    /// nested grandchild, [3] a reverting CALL that is caught.
    function run(address payable leaf) external payable {
        Leaf(leaf).touch{value: 1 wei}(42); // child 0: CALL with value
        Leaf(leaf).x();                      // child 1: STATICCALL (view)
        this.helper(leaf);                   // child 2: CALL -> grandchild
        try Leaf(leaf).boom() {} catch {}    // child 3: reverting CALL (caught)
        emit Ran(leaf);
    }

    function helper(address payable leaf) external {
        Leaf(leaf).touch(99); // grandchild of child 2
    }
}
