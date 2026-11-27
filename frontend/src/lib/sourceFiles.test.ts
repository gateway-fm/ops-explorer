import { describe, it, expect } from 'vitest';
import { displayPath, parseSources } from './sourceFiles';

const proxy = 'contract ERC1967Proxy is Proxy {}';
const standardJSON = JSON.stringify({
  language: 'Solidity',
  sources: {
    '../node_modules/@openzeppelin/contracts/proxy/Proxy.sol': { content: 'abstract contract Proxy {}' },
    '../node_modules/@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol': { content: proxy },
    'src/Helper.sol': { content: 'library Helper {}' },
  },
  settings: { optimizer: { enabled: true } },
});

describe('parseSources', () => {
  it('splits a standard-JSON input into files, verified contract first', () => {
    const files = parseSources(standardJSON, 'ERC1967Proxy');
    expect(files.map((f) => displayPath(f.path))).toEqual([
      'node_modules/@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol',
      'src/Helper.sol',
      'node_modules/@openzeppelin/contracts/proxy/Proxy.sol',
    ]);
    expect(files[0].content).toBe(proxy);
  });

  it("unwraps Etherscan's double-braced form", () => {
    const files = parseSources(`{${standardJSON}}`, 'ERC1967Proxy');
    expect(files).toHaveLength(3);
  });

  it('accepts a bare sources map', () => {
    const files = parseSources(JSON.stringify({ 'A.sol': { content: 'contract A {}' } }), 'A');
    expect(files).toEqual([{ path: 'A.sol', content: 'contract A {}' }]);
  });

  it('keeps a flattened source as one file named after the contract', () => {
    const src = '// SPDX-License-Identifier: MIT\ncontract Token {}';
    expect(parseSources(src, 'Token')).toEqual([{ path: 'Token.sol', content: src }]);
  });

  it('falls back to one file when JSON has no sources', () => {
    const src = '{"not":"sources"}';
    expect(parseSources(src)).toEqual([{ path: 'Contract.sol', content: src }]);
  });
  it("splits this explorer's own verifier output on // File: headers", () => {
    const raw = '// File: contracts/Lib.sol\nlibrary Lib {}\n\n// File: contracts/Token.sol\ncontract Token {}\n\n';
    expect(parseSources(raw, 'Token')).toEqual([
      { path: 'contracts/Token.sol', content: 'contract Token {}\n' },
      { path: 'contracts/Lib.sol', content: 'library Lib {}\n' },
    ]);
  });

  it('keeps a single source that only mentions // File: further down as one file', () => {
    const src = 'contract A {}\n// File: not/a/header.sol\n';
    expect(parseSources(src, 'A')).toEqual([{ path: 'A.sol', content: src }]);
  });

  it('splits verifier output whose file names contain spaces', () => {
    const raw = '// File: Token (1).sol\ncontract Token {}\n\n// File: my libs/Lib copy.sol\nlibrary Lib {}\n\n';
    expect(parseSources(raw, 'Token')).toEqual([
      { path: 'Token (1).sol', content: 'contract Token {}\n' },
      { path: 'my libs/Lib copy.sol', content: 'library Lib {}\n' },
    ]);
  });

  it('does not split verifier output on a // File: comment inside a contract body', () => {
    const raw =
      '// File: contracts/Token.sol\ncontract Token {\n\n// File: contracts/Fake.sol\n  uint256 x;\n}\n\n';
    expect(parseSources(raw, 'Token')).toEqual([{ path: 'Token.sol', content: raw }]);
  });

  it('does not split verifier output on a // File: comment inside a block comment', () => {
    const raw =
      '// File: contracts/Token.sol\n/* notes\n\n// File: contracts/Fake.sol\n{ */\ncontract Token {}\n\n';
    expect(parseSources(raw, 'Token')).toEqual([{ path: 'Token.sol', content: raw }]);
  });

  it('keeps a // File: line that is not on its own paragraph or not a path inside its file', () => {
    const raw =
      '// File: contracts/Token.sol\ncontract A {}\n// File: contracts/B.sol\n\n// File: notes on B\ncontract Token {}\n\n';
    expect(parseSources(raw, 'Token')).toEqual([
      {
        path: 'contracts/Token.sol',
        content: 'contract A {}\n// File: contracts/B.sol\n\n// File: notes on B\ncontract Token {}\n',
      },
    ]);
  });
});
