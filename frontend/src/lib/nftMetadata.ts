// Resolves an ERC-721 tokenURI into displayable metadata.
//
// Handles the three shapes we see in practice:
//   * data:application/json[;base64],…  — decoded inline (our test fixtures)
//   * ipfs://…                          — rewritten to a public gateway
//   * http(s)://…                       — fetched as JSON
// The `image` field is normalised the same way (ipfs:// → gateway). A data:
// tokenURI whose media type is image/* is treated as the artwork directly.

const IPFS_GATEWAY = 'https://ipfs.io/ipfs/';

export interface NftAttribute {
  trait_type?: string;
  value: string | number;
}

export interface NftMetadata {
  name?: string;
  description?: string;
  image?: string;
  attributes?: NftAttribute[];
}

function ipfsToHttp(uri: string): string {
  if (!uri.startsWith('ipfs://')) return uri;
  let path = uri.slice('ipfs://'.length);
  if (path.startsWith('ipfs/')) path = path.slice('ipfs/'.length);
  return IPFS_GATEWAY + path;
}

function decodeDataBody(uri: string, mediatype: string): string {
  const comma = uri.indexOf(',');
  const body = comma === -1 ? '' : uri.slice(comma + 1);
  if (/;base64/i.test(mediatype)) {
    // atob yields a byte string; decode it as UTF-8 so multi-byte characters
    // (em dashes, emoji, accents in names/descriptions) survive.
    const bytes = Uint8Array.from(atob(body), (c) => c.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  }
  return decodeURIComponent(body);
}

export async function resolveNftMetadata(tokenUri?: string): Promise<NftMetadata> {
  const uri = tokenUri?.trim();
  if (!uri) return {};

  let jsonText: string;

  if (uri.startsWith('data:')) {
    const comma = uri.indexOf(',');
    const mediatype = comma === -1 ? '' : uri.slice('data:'.length, comma);
    // A bare image data URI is the artwork itself, not a metadata document.
    if (/^image\//i.test(mediatype)) {
      return { image: uri };
    }
    jsonText = decodeDataBody(uri, mediatype);
  } else {
    const res = await fetch(ipfsToHttp(uri));
    if (!res.ok) throw new Error(`metadata fetch failed: ${res.status}`);
    jsonText = await res.text();
  }

  const json = JSON.parse(jsonText) as Record<string, unknown>;
  const attributes = Array.isArray(json.attributes)
    ? (json.attributes as unknown[]).filter(
        (a): a is NftAttribute =>
          !!a && typeof a === 'object' && 'value' in (a as Record<string, unknown>),
      )
    : undefined;
  return {
    name: typeof json.name === 'string' ? json.name : undefined,
    description: typeof json.description === 'string' ? json.description : undefined,
    image: typeof json.image === 'string' ? ipfsToHttp(json.image) : undefined,
    attributes,
  };
}
