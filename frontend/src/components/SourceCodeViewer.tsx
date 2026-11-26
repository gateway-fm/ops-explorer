import { useMemo, useState } from 'react';
import { FileCode2 } from 'lucide-react';
import { CopyButton } from './CopyButton';
import { displayPath, parseSources } from '../lib/sourceFiles';

const KEYWORDS = new Set([
  'pragma', 'solidity', 'import', 'from', 'as', 'contract', 'abstract', 'library', 'interface', 'is',
  'function', 'modifier', 'event', 'error', 'struct', 'enum', 'mapping', 'constructor', 'fallback',
  'receive', 'returns', 'return', 'if', 'else', 'for', 'while', 'do', 'break', 'continue', 'emit',
  'revert', 'require', 'assert', 'new', 'delete', 'using', 'type', 'assembly', 'unchecked', 'try',
  'catch', 'public', 'private', 'internal', 'external', 'view', 'pure', 'payable', 'virtual',
  'override', 'constant', 'immutable', 'memory', 'storage', 'calldata', 'indexed', 'anonymous',
  'true', 'false', 'this', 'super', 'let',
]);
const TYPE_RE = /^(address|bool|string|bytes\d*|u?int\d*|u?fixed[\dx]*)$/;
const TOKEN_RE = /(\/\/[^\n]*|\/\*[\s\S]*?(?:\*\/|$)|"(?:\\.|[^"\\\n])*"?|'(?:\\.|[^'\\\n])*'?|\b0x[0-9a-fA-F]+\b|\b\d[\d_]*(?:\.\d+)?(?:e\d+)?\b|\b[A-Za-z_$][\w$]*\b)/g;

type Tok = { text: string; cls?: string };

// highlightLines tokenises the whole file once (so block comments spanning
// lines are coloured correctly) and splits the tokens back into lines.
function highlightLines(content: string): Tok[][] {
  const lines: Tok[][] = [[]];
  const push = (text: string, cls?: string) => {
    const parts = text.split('\n');
    parts.forEach((p, i) => {
      if (i > 0) lines.push([]);
      if (p) lines[lines.length - 1].push({ text: p, cls });
    });
  };
  let last = 0;
  for (const m of content.matchAll(TOKEN_RE)) {
    const t = m[0];
    const at = m.index ?? 0;
    if (at > last) push(content.slice(last, at));
    let cls: string | undefined;
    if (t.startsWith('//') || t.startsWith('/*')) cls = 'text-neutral-400 italic';
    else if (t.startsWith('"') || t.startsWith("'")) cls = 'text-success-700 dark:text-success-500';
    else if (/^(0x|\d)/.test(t)) cls = 'text-warning-700 dark:text-warning-500';
    else if (KEYWORDS.has(t)) cls = 'text-primary font-medium';
    else if (TYPE_RE.test(t)) cls = 'text-primary-600 dark:text-primary-300';
    push(t, cls);
    last = at + t.length;
  }
  if (last < content.length) push(content.slice(last));
  return lines;
}

export function SourceCodeViewer({ sourceCode, contractName }: { sourceCode: string; contractName?: string }) {
  const files = useMemo(() => parseSources(sourceCode, contractName), [sourceCode, contractName]);
  const [selected, setSelected] = useState(0);
  const file = files[Math.min(selected, files.length - 1)];
  const lines = useMemo(() => highlightLines(file.content), [file.content]);
  const gutter = String(lines.length).length;

  return (
    <section className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50" data-testid="source-code">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-neutral-200 px-5 py-3">
        <h3 className="text-sm font-semibold text-neutral-900">Source code</h3>
        <span className="text-xs text-neutral-500">
          {files.length === 1 ? '1 file' : `${files.length} files`}
        </span>
      </header>
      <div className="flex flex-col md:flex-row">
        {files.length > 1 && (
          <nav
            aria-label="Source files"
            className="max-h-48 shrink-0 overflow-y-auto border-b border-neutral-200 py-2 md:max-h-[32rem] md:w-72 md:border-b-0 md:border-r"
          >
            {files.map((f, i) => {
              const path = displayPath(f.path);
              const slash = path.lastIndexOf('/');
              const active = f === file;
              return (
                <button
                  key={f.path}
                  type="button"
                  data-testid="source-file"
                  aria-current={active ? 'true' : undefined}
                  onClick={() => setSelected(i)}
                  title={path}
                  className={`flex w-full items-start gap-2 px-4 py-1.5 text-left transition-colors ${
                    active ? 'bg-primary/10 text-primary' : 'text-neutral-700 hover:bg-neutral-100'
                  }`}
                >
                  <FileCode2 className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  <span className="min-w-0">
                    <span className="block truncate font-mono text-xs font-medium">{path.slice(slash + 1)}</span>
                    {slash > 0 && <span className="block truncate font-mono text-[11px] text-neutral-500">{path.slice(0, slash)}</span>}
                  </span>
                </button>
              );
            })}
          </nav>
        )}
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-3 border-b border-neutral-200 bg-neutral-100/60 px-4 py-2">
            <span className="truncate font-mono text-xs text-neutral-600" title={displayPath(file.path)}>
              {displayPath(file.path)}
            </span>
            <div className="flex shrink-0 items-center gap-3">
              <span className="font-mono text-xs tabular-nums text-neutral-500">{lines.length.toLocaleString()} lines</span>
              <CopyButton text={file.content} />
            </div>
          </div>
          <div className="max-h-[32rem] overflow-auto bg-neutral-100/60">
            <pre className="py-3 font-mono text-xs leading-relaxed text-neutral-800">
              {lines.map((toks, i) => (
                <div key={i} className="flex">
                  <span
                    aria-hidden="true"
                    className="sticky left-0 shrink-0 select-none bg-neutral-100 pl-4 pr-4 text-right tabular-nums text-neutral-400"
                    style={{ minWidth: `${gutter + 3}ch` }}
                  >
                    {i + 1}
                  </span>
                  <code className="whitespace-pre pr-4">
                    {toks.length === 0
                      ? ' '
                      : toks.map((t, j) => (t.cls ? <span key={j} className={t.cls}>{t.text}</span> : t.text))}
                  </code>
                </div>
              ))}
            </pre>
          </div>
        </div>
      </div>
    </section>
  );
}
