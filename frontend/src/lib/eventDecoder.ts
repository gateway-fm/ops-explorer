// Event signature decoder for common ERC20/ERC721/ERC1155 events
// Topic 0 is keccak256 hash of event signature

export interface EventParam {
  name: string;
  type: string;
  indexed: boolean;
  value?: string;
}

export interface DecodedEvent {
  name: string;
  signature: string;
  params: EventParam[];
}

export interface EventSignature {
  name: string;
  signature: string;
  params: { name: string; type: string; indexed: boolean }[];
}

// Common event signatures with their keccak256 hashes as topic0
// Hash is keccak256(signature) e.g., keccak256("Transfer(address,address,uint256)")
export const KNOWN_EVENTS: Record<string, EventSignature> = {
  // ERC20 Transfer
  '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef': {
    name: 'Transfer',
    signature: 'Transfer(address indexed from, address indexed to, uint256 value)',
    params: [
      { name: 'from', type: 'address', indexed: true },
      { name: 'to', type: 'address', indexed: true },
      { name: 'value', type: 'uint256', indexed: false },
    ],
  },
  // ERC20 Approval
  '0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925': {
    name: 'Approval',
    signature: 'Approval(address indexed owner, address indexed spender, uint256 value)',
    params: [
      { name: 'owner', type: 'address', indexed: true },
      { name: 'spender', type: 'address', indexed: true },
      { name: 'value', type: 'uint256', indexed: false },
    ],
  },
  // ERC721 Transfer (same signature as ERC20 but tokenId instead of value)
  // Note: Same topic0 as ERC20 Transfer - context determines interpretation

  // ERC721 ApprovalForAll
  '0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31': {
    name: 'ApprovalForAll',
    signature: 'ApprovalForAll(address indexed owner, address indexed operator, bool approved)',
    params: [
      { name: 'owner', type: 'address', indexed: true },
      { name: 'operator', type: 'address', indexed: true },
      { name: 'approved', type: 'bool', indexed: false },
    ],
  },
  // ERC1155 TransferSingle
  '0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62': {
    name: 'TransferSingle',
    signature: 'TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint256 value)',
    params: [
      { name: 'operator', type: 'address', indexed: true },
      { name: 'from', type: 'address', indexed: true },
      { name: 'to', type: 'address', indexed: true },
      { name: 'id', type: 'uint256', indexed: false },
      { name: 'value', type: 'uint256', indexed: false },
    ],
  },
  // ERC1155 TransferBatch
  '0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb': {
    name: 'TransferBatch',
    signature: 'TransferBatch(address indexed operator, address indexed from, address indexed to, uint256[] ids, uint256[] values)',
    params: [
      { name: 'operator', type: 'address', indexed: true },
      { name: 'from', type: 'address', indexed: true },
      { name: 'to', type: 'address', indexed: true },
      { name: 'ids', type: 'uint256[]', indexed: false },
      { name: 'values', type: 'uint256[]', indexed: false },
    ],
  },
  // Deposit (WETH)
  '0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c': {
    name: 'Deposit',
    signature: 'Deposit(address indexed dst, uint256 wad)',
    params: [
      { name: 'dst', type: 'address', indexed: true },
      { name: 'wad', type: 'uint256', indexed: false },
    ],
  },
  // Withdrawal (WETH)
  '0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65': {
    name: 'Withdrawal',
    signature: 'Withdrawal(address indexed src, uint256 wad)',
    params: [
      { name: 'src', type: 'address', indexed: true },
      { name: 'wad', type: 'uint256', indexed: false },
    ],
  },
  // Uniswap V2 Swap
  '0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822': {
    name: 'Swap',
    signature: 'Swap(address indexed sender, uint256 amount0In, uint256 amount1In, uint256 amount0Out, uint256 amount1Out, address indexed to)',
    params: [
      { name: 'sender', type: 'address', indexed: true },
      { name: 'amount0In', type: 'uint256', indexed: false },
      { name: 'amount1In', type: 'uint256', indexed: false },
      { name: 'amount0Out', type: 'uint256', indexed: false },
      { name: 'amount1Out', type: 'uint256', indexed: false },
      { name: 'to', type: 'address', indexed: true },
    ],
  },
  // Uniswap V2 Sync
  '0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1': {
    name: 'Sync',
    signature: 'Sync(uint112 reserve0, uint112 reserve1)',
    params: [
      { name: 'reserve0', type: 'uint112', indexed: false },
      { name: 'reserve1', type: 'uint112', indexed: false },
    ],
  },
  // Uniswap V2 Mint
  '0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fef26394f4c03821c4f': {
    name: 'Mint',
    signature: 'Mint(address indexed sender, uint256 amount0, uint256 amount1)',
    params: [
      { name: 'sender', type: 'address', indexed: true },
      { name: 'amount0', type: 'uint256', indexed: false },
      { name: 'amount1', type: 'uint256', indexed: false },
    ],
  },
  // Uniswap V2 Burn
  '0xdccd412f0b1252819cb1fd330b93224ca42612892bb3f4f789976e6d81936496': {
    name: 'Burn',
    signature: 'Burn(address indexed sender, uint256 amount0, uint256 amount1, address indexed to)',
    params: [
      { name: 'sender', type: 'address', indexed: true },
      { name: 'amount0', type: 'uint256', indexed: false },
      { name: 'amount1', type: 'uint256', indexed: false },
      { name: 'to', type: 'address', indexed: true },
    ],
  },
  // OwnershipTransferred
  '0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0': {
    name: 'OwnershipTransferred',
    signature: 'OwnershipTransferred(address indexed previousOwner, address indexed newOwner)',
    params: [
      { name: 'previousOwner', type: 'address', indexed: true },
      { name: 'newOwner', type: 'address', indexed: true },
    ],
  },
};

// Decode a hex topic to an address (last 20 bytes)
function decodeAddress(hex: string): string {
  const clean = hex.startsWith('0x') ? hex.slice(2) : hex;
  return '0x' + clean.slice(-40).toLowerCase();
}

// Decode hex to uint256
function decodeUint256(hex: string): string {
  try {
    const clean = hex.startsWith('0x') ? hex : '0x' + hex;
    return BigInt(clean).toString();
  } catch {
    return hex;
  }
}

// Decode hex to bool
function decodeBool(hex: string): string {
  try {
    const clean = hex.startsWith('0x') ? hex : '0x' + hex;
    return BigInt(clean) !== 0n ? 'true' : 'false';
  } catch {
    return hex;
  }
}

// Decode a single value based on type
function decodeValue(hex: string, type: string): string {
  if (type === 'address') {
    return decodeAddress(hex);
  } else if (type === 'bool') {
    return decodeBool(hex);
  } else if (type.startsWith('uint') || type.startsWith('int')) {
    return decodeUint256(hex);
  } else if (type.startsWith('bytes')) {
    return hex.startsWith('0x') ? hex : '0x' + hex;
  }
  return hex;
}

// Decode non-indexed parameters from data
function decodeDataParams(data: string, params: { name: string; type: string; indexed: boolean }[]): Map<string, string> {
  const result = new Map<string, string>();
  const nonIndexedParams = params.filter(p => !p.indexed);

  if (!data || data === '0x' || data === '') return result;

  const cleanData = data.startsWith('0x') ? data.slice(2) : data;

  // Each parameter is 32 bytes (64 hex chars)
  let offset = 0;
  for (const param of nonIndexedParams) {
    if (offset + 64 > cleanData.length) break;

    const chunk = cleanData.slice(offset, offset + 64);
    result.set(param.name, decodeValue('0x' + chunk, param.type));
    offset += 64;
  }

  return result;
}

// Main decode function
export function decodeEvent(
  topic0: string | null,
  topic1: string | null,
  topic2: string | null,
  topic3: string | null,
  data: string
): DecodedEvent | null {
  if (!topic0) return null;

  const eventSig = KNOWN_EVENTS[topic0.toLowerCase()];
  if (!eventSig) return null;

  const topics = [topic1, topic2, topic3].filter(Boolean) as string[];
  const dataValues = decodeDataParams(data, eventSig.params);

  const decodedParams: EventParam[] = [];
  let topicIndex = 0;

  for (const param of eventSig.params) {
    if (param.indexed) {
      const topicValue = topics[topicIndex];
      decodedParams.push({
        ...param,
        value: topicValue ? decodeValue(topicValue, param.type) : undefined,
      });
      topicIndex++;
    } else {
      decodedParams.push({
        ...param,
        value: dataValues.get(param.name),
      });
    }
  }

  return {
    name: eventSig.name,
    signature: eventSig.signature,
    params: decodedParams,
  };
}

// Get method ID from topic0 (first 4 bytes equivalent for events)
export function getMethodId(topic0: string): string {
  return topic0.slice(0, 10);
}

// Cache for fetched signatures from 4byte.directory
const signatureCache = new Map<string, EventSignature | null>();

// Parse a text signature like "AirDropClaimed(address,bytes32,uint256)" into structured params
// Note: 4byte.directory doesn't provide indexed info, so we infer from topic count
export function parseTextSignature(textSig: string, topicCount: number): EventSignature | null {
  // Match: EventName(type1,type2,...)
  const match = textSig.match(/^(\w+)\((.*)\)$/);
  if (!match) return null;

  const [, name, paramsStr] = match;
  if (!paramsStr) {
    return { name, signature: textSig, params: [] };
  }

  // Parse parameters - handle nested tuples
  const paramTypes = parseParamTypes(paramsStr);

  // Infer indexed parameters based on topic count
  // Topics: topic0 is event sig, topic1-3 are indexed params
  // So if we have 3 topics total, we have 2 indexed params
  const indexedCount = Math.max(0, topicCount - 1);

  const params = paramTypes.map((type, i) => ({
    name: `arg${i}`,
    type,
    indexed: i < indexedCount,
  }));

  // Build a more readable signature with indexed annotations
  const sigWithIndexed = `${name}(${params.map(p =>
    `${p.type}${p.indexed ? ' indexed' : ''} ${p.name}`
  ).join(', ')})`;

  return { name, signature: sigWithIndexed, params };
}

// Parse comma-separated param types, handling nested tuples
function parseParamTypes(paramsStr: string): string[] {
  const types: string[] = [];
  let depth = 0;
  let current = '';

  for (const char of paramsStr) {
    if (char === '(' || char === '[') {
      depth++;
      current += char;
    } else if (char === ')' || char === ']') {
      depth--;
      current += char;
    } else if (char === ',' && depth === 0) {
      if (current.trim()) types.push(current.trim());
      current = '';
    } else {
      current += char;
    }
  }
  if (current.trim()) types.push(current.trim());

  return types;
}

// Fetch event signature from 4byte.directory
export async function fetchEventSignature(topic0: string, topicCount: number): Promise<EventSignature | null> {
  const cacheKey = topic0.toLowerCase();

  // Check cache first
  if (signatureCache.has(cacheKey)) {
    return signatureCache.get(cacheKey) || null;
  }

  try {
    const response = await fetch(
      `https://www.4byte.directory/api/v1/event-signatures/?hex_signature=${topic0}`
    );

    if (!response.ok) {
      signatureCache.set(cacheKey, null);
      return null;
    }

    const data = await response.json();

    if (!data.results || data.results.length === 0) {
      signatureCache.set(cacheKey, null);
      return null;
    }

    // Get the first (most common) result
    const textSig = data.results[0].text_signature;
    const parsed = parseTextSignature(textSig, topicCount);

    signatureCache.set(cacheKey, parsed);
    return parsed;
  } catch (error) {
    console.error('Failed to fetch event signature:', error);
    signatureCache.set(cacheKey, null);
    return null;
  }
}

// Decode event with fetched signature
export function decodeEventWithSignature(
  eventSig: EventSignature,
  topic1: string | null,
  topic2: string | null,
  topic3: string | null,
  data: string
): DecodedEvent {
  const topics = [topic1, topic2, topic3].filter(Boolean) as string[];
  const dataValues = decodeDataParams(data, eventSig.params);

  const decodedParams: EventParam[] = [];
  let topicIndex = 0;

  for (const param of eventSig.params) {
    if (param.indexed) {
      const topicValue = topics[topicIndex];
      decodedParams.push({
        ...param,
        value: topicValue ? decodeValue(topicValue, param.type) : undefined,
      });
      topicIndex++;
    } else {
      decodedParams.push({
        ...param,
        value: dataValues.get(param.name),
      });
    }
  }

  return {
    name: eventSig.name,
    signature: eventSig.signature,
    params: decodedParams,
  };
}
