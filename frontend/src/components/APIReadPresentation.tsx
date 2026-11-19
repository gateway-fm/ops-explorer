import { useState, type ReactNode } from 'react';
import { Copy, Check, ChevronDown } from 'lucide-react';
export interface APIParam {
  name: string;
  type: string;
  required?: boolean;
  description: string;
  default?: string;
}

export interface APIEndpoint {
  method: 'GET';
  path: string;
  description: string;
  params?: APIParam[];
  exampleResponse: unknown;
}

export interface APIEndpointGroup {
  id: string;
  label: string;
  endpoints: APIEndpoint[];
}

export function JsonSyntax({ data }: { data: unknown }) {
  const json = JSON.stringify(data, null, 2);

  // Tokenize JSON into spans without dangerouslySetInnerHTML
  const tokens: { text: string; className?: string }[] = [];
  const re = /("(?:\\.|[^"\\])*")\s*:|:\s*("(?:\\.|[^"\\])*")|:\s*(\d+(?:\.\d+)?)|:\s*(true|false|null)|[^":]+|./g;
  let match: RegExpExecArray | null;
  let lastIndex = 0;

  while ((match = re.exec(json)) !== null) {
    // Add any skipped text as plain
    if (match.index > lastIndex) {
      tokens.push({ text: json.slice(lastIndex, match.index) });
    }
    if (match[1]) {
      // Key
      tokens.push({ text: match[1], className: 'text-purple-600' });
      tokens.push({ text: ':' });
    } else if (match[2]) {
      // String value
      tokens.push({ text: ': ' });
      tokens.push({ text: match[2], className: 'text-green-600' });
    } else if (match[3]) {
      // Number value
      tokens.push({ text: ': ' });
      tokens.push({ text: match[3], className: 'text-blue-600' });
    } else if (match[4]) {
      // Boolean/null
      tokens.push({ text: ': ' });
      tokens.push({ text: match[4], className: 'text-amber-600' });
    } else {
      tokens.push({ text: match[0] });
    }
    lastIndex = re.lastIndex;
  }
  if (lastIndex < json.length) {
    tokens.push({ text: json.slice(lastIndex) });
  }

  return (
    <pre className="text-xs leading-relaxed overflow-x-auto">
      {tokens.map((t, i) =>
        t.className ? <span key={i} className={t.className}>{t.text}</span> : t.text
      )}
    </pre>
  );
}

// ---------------------------------------------------------------------------
// Copy button
// ---------------------------------------------------------------------------

export function APICopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const copy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      onClick={copy}
      className="p-1.5 rounded-md text-neutral-400 hover:text-neutral-600 hover:bg-neutral-200 transition-colors"
      title="Copy"
    >
      {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
    </button>
  );
}

// ---------------------------------------------------------------------------
// Build curl command
// ---------------------------------------------------------------------------

export function APIEndpointCard({ endpoint, curl, tryIt }: { endpoint: APIEndpoint; curl: string; tryIt: ReactNode }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="card overflow-hidden">
      {/* Header */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 px-4 py-3 hover:bg-neutral-50 transition-colors text-left"
      >
        <span className="shrink-0 px-2 py-0.5 rounded text-xs font-bold uppercase bg-green-100 text-green-700 border border-green-200">
          {endpoint.method}
        </span>
        <span className="font-mono text-sm text-neutral-800 truncate">{endpoint.path}</span>
        <ChevronDown className={`w-4 h-4 text-neutral-400 ml-auto shrink-0 transition-transform ${expanded ? 'rotate-180' : ''}`} />
      </button>

      {expanded && (
        <div>
          {/* Description */}
          <div className="px-4 pb-3">
            <p className="text-sm text-neutral-600">{endpoint.description}</p>
          </div>

          {/* Parameters */}
          {endpoint.params && endpoint.params.length > 0 && (
            <div className="px-4 pb-4">
              <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Parameters</h4>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-neutral-200">
                      <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Name</th>
                      <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Type</th>
                      <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Required</th>
                      <th className="text-left py-2 text-xs font-medium text-neutral-500">Description</th>
                    </tr>
                  </thead>
                  <tbody>
                    {endpoint.params.map((p) => (
                      <tr key={p.name} className="border-b border-neutral-100 last:border-b-0">
                        <td className="py-2 pr-4 font-mono text-xs text-neutral-800">{p.name}</td>
                        <td className="py-2 pr-4 text-xs text-neutral-500">{p.type}</td>
                        <td className="py-2 pr-4 text-xs">
                          {p.required ? (
                            <span className="text-red-600 font-medium">Yes</span>
                          ) : (
                            <span className="text-neutral-400">No</span>
                          )}
                        </td>
                        <td className="py-2 text-xs text-neutral-600">
                          {p.description}
                          {p.default && <span className="text-neutral-400 ml-1">(default: {p.default})</span>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Curl Example */}
          <div className="px-4 pb-4">
            <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Example Request</h4>
            <div className="relative bg-neutral-900 rounded-lg p-4">
              <div className="absolute top-2 right-2">
                <APICopyButton text={curl} />
              </div>
              <pre className="text-xs text-neutral-100 font-mono overflow-x-auto pr-8">{curl}</pre>
            </div>
          </div>

          {/* Example Response */}
          <div className="px-4 pb-4">
            <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Example Response</h4>
            <div className="relative bg-neutral-50 rounded-lg border border-neutral-200 p-4">
              <div className="absolute top-2 right-2">
                <APICopyButton text={JSON.stringify(endpoint.exampleResponse, null, 2)} />
              </div>
              <JsonSyntax data={endpoint.exampleResponse} />
            </div>
          </div>

          {/* Try It */}
          {tryIt}
        </div>
      )}
    </div>
  );
}

export function APISidebar({ groups, activeGroup, onSelect, showRateLimiting = true }: { groups: APIEndpointGroup[]; activeGroup: string; onSelect: (id: string) => void; showRateLimiting?: boolean }) {
  return (
    <nav className="space-y-1">
      {groups.map((g) => (
        <button
          key={g.id}
          onClick={() => onSelect(g.id)}
          className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
            activeGroup === g.id
              ? 'bg-primary-50 text-primary-700 font-medium border border-primary-200'
              : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900'
          }`}
        >
          {g.label}
          <span className="ml-1.5 text-xs text-neutral-400">({g.endpoints.length})</span>
        </button>
      ))}
      {showRateLimiting && <button
        onClick={() => onSelect('rate-limiting')}
        className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
          activeGroup === 'rate-limiting'
            ? 'bg-primary-50 text-primary-700 font-medium border border-primary-200'
            : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900'
        }`}
      >
        Rate Limiting
      </button>}
    </nav>
  );
}
