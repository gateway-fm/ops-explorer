export interface SourceFile {
  path: string;
  content: string;
}

// parseSources turns the stored verification source into a list of files.
// The stored value is either a flattened single file, a Solidity standard-JSON
// input ({"language": ..., "sources": {path: {content}}}), a bare sources map
// ({path: {content}}), Etherscan's double-braced form of either ({{ ... }}), or
// this explorer's own verifier output: every submitted file concatenated, each
// preceded by a "// File: <path>" line (backend/internal/verifier, storeVerification).
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
  if (!files) files = splitFileHeaders(raw);
  if (!files) return [{ path: `${contractName || 'Contract'}.sol`, content: raw }];

  const name = contractName?.replace(/[^A-Za-z0-9_$]/g, '');
  const declares = name
    ? new RegExp(`\\b(contract|library|interface)\\s+${name}\\b`)
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

const FILE_HEADER = /^\/\/ File: (.+)$/gm;

// splitFileHeaders splits text made of "// File: <path>" sections into files.
// Only text that starts with such a header qualifies, so a single source that
// merely mentions one in a comment further down stays one file.
function splitFileHeaders(raw: string): SourceFile[] | null {
  if (!raw.startsWith('// File: ')) return null;
  const headers = [...raw.matchAll(FILE_HEADER)];
  const files = headers.map((m, i) => {
    const start = (m.index ?? 0) + m[0].length + 1;
    const end = i + 1 < headers.length ? headers[i + 1].index ?? raw.length : raw.length;
    return { path: m[1].trim(), content: raw.slice(start, end).replace(/\n+$/, '\n') };
  });
  return files.length > 0 ? files : null;
}
