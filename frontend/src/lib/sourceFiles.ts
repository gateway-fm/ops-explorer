export interface SourceFile {
  path: string;
  content: string;
}

// parseSources turns the stored verification source into a list of files.
// The stored value is either a flattened single file, a Solidity standard-JSON
// input ({"language": ..., "sources": {path: {content}}}), a bare sources map
// ({path: {content}}), or Etherscan's double-braced form of either ({{ ... }}).
// The file that declares the verified contract is put first.
export function parseSources(raw: string, contractName?: string): SourceFile[] {
  let src = raw.trim();
  if (src.startsWith('{{') && src.endsWith('}}')) src = src.slice(1, -1);

  let files: SourceFile[] | null = null;
  if (src.startsWith('{')) {
    try {
      const root = JSON.parse(src) as Record<string, unknown>;
      const container = (root.sources && typeof root.sources === 'object' ? root.sources : root) as Record<string, unknown>;
      const parsed: SourceFile[] = [];
      for (const [path, v] of Object.entries(container)) {
        if (v && typeof v === 'object' && typeof (v as { content?: unknown }).content === 'string') {
          parsed.push({ path, content: (v as { content: string }).content });
        }
      }
      if (parsed.length > 0) files = parsed;
    } catch {
      // Not JSON: a flattened source that happens to start with '{'.
    }
  }
  if (!files) return [{ path: `${contractName || 'Contract'}.sol`, content: raw }];

  const declares = contractName
    ? new RegExp(`\\b(contract|library|interface)\\s+${contractName.replace(/[^A-Za-z0-9_$]/g, '')}\\b`)
    : null;
  const rank = (f: SourceFile) => {
    if (declares?.test(f.content)) return 0;
    if (/(^|\/)node_modules\/|^@|(^|\/)lib\//.test(displayPath(f.path))) return 2;
    return 1;
  };
  return files.sort((a, b) => rank(a) - rank(b) || a.path.localeCompare(b.path));
}

// displayPath drops leading "./" and "../" segments, which only describe where
// the compiler ran from.
export function displayPath(path: string): string {
  return path.replace(/^(\.\.?\/)+/, '');
}
