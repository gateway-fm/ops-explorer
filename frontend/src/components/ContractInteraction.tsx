import { useState, useMemo } from 'react';
import { BrowserProvider, Contract, parseUnits } from 'ethers';
import type { Eip1193Provider } from 'ethers';
import type { AbiFragment, AbiInput } from '../lib/api';
import { ChevronDown, ChevronUp, Loader2, Wallet, AlertCircle, CheckCircle } from 'lucide-react';

// Extend Window interface for ethereum provider
declare global {
  interface Window {
    ethereum?: Eip1193Provider;
  }
}

// Get RPC URL from environment
const RPC_URL = import.meta.env.VITE_RPC_URL || 'http://localhost:8545';

interface ContractInteractionProps {
  address: string;
  abi?: AbiFragment[];
  type: 'read' | 'write';
}

export function ContractInteraction({ address, abi, type }: ContractInteractionProps) {
  const [expandedFunctions, setExpandedFunctions] = useState<Set<string>>(new Set());

  // Filter functions based on type
  const functions = useMemo(() => {
    if (!abi) return [];
    return abi.filter((item) => {
      if (item.type !== 'function') return false;
      if (type === 'read') {
        return item.stateMutability === 'view' || item.stateMutability === 'pure';
      } else {
        return item.stateMutability === 'nonpayable' || item.stateMutability === 'payable';
      }
    });
  }, [abi, type]);

  const toggleFunction = (name: string) => {
    const newExpanded = new Set(expandedFunctions);
    if (newExpanded.has(name)) {
      newExpanded.delete(name);
    } else {
      newExpanded.add(name);
    }
    setExpandedFunctions(newExpanded);
  };

  if (!abi || abi.length === 0) {
    return (
      <div className="text-center text-neutral-400 py-8">
        No ABI available. Upload an ABI to interact with this contract.
      </div>
    );
  }

  if (functions.length === 0) {
    return (
      <div className="text-center text-neutral-400 py-8">
        No {type === 'read' ? 'read' : 'write'} functions found in ABI.
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {functions.map((fn, index) => (
        <FunctionCard
          key={`${fn.name}-${index}`}
          contractAddress={address}
          abiFragment={fn}
          index={index + 1}
          type={type}
          isExpanded={expandedFunctions.has(fn.name || '')}
          onToggle={() => toggleFunction(fn.name || '')}
        />
      ))}
    </div>
  );
}

interface FunctionCardProps {
  contractAddress: string;
  abiFragment: AbiFragment;
  index: number;
  type: 'read' | 'write';
  isExpanded: boolean;
  onToggle: () => void;
}

function FunctionCard({ contractAddress, abiFragment, index, type, isExpanded, onToggle }: FunctionCardProps) {
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [txHash, setTxHash] = useState<string | null>(null);

  const handleInputChange = (name: string, value: string) => {
    setInputs((prev) => ({ ...prev, [name]: value }));
    setError(null);
    setResult(null);
    setTxHash(null);
  };

  const executeRead = async () => {
    setIsLoading(true);
    setError(null);
    setResult(null);

    try {
      // Use JSON-RPC provider for read operations
      const { JsonRpcProvider } = await import('ethers');
      const provider = new JsonRpcProvider(RPC_URL);
      const contract = new Contract(contractAddress, [abiFragment], provider);

      const args = (abiFragment.inputs || []).map((input) => {
        const value = inputs[input.name] || '';
        return parseInputValue(value, input.type);
      });

      const res = await contract[abiFragment.name!](...args);
      setResult(formatResult(res, abiFragment.outputs || []));
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Unknown error';
      setError(errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  const executeWrite = async () => {
    setIsLoading(true);
    setError(null);
    setTxHash(null);

    try {
      // Check for wallet
      if (typeof window === 'undefined' || !window.ethereum) {
        throw new Error('Please install MetaMask or another Web3 wallet');
      }

      const provider = new BrowserProvider(window.ethereum);
      await provider.send('eth_requestAccounts', []);
      const signer = await provider.getSigner();

      const contract = new Contract(contractAddress, [abiFragment], signer);

      const args = (abiFragment.inputs || []).map((input) => {
        const value = inputs[input.name] || '';
        return parseInputValue(value, input.type);
      });

      // Handle payable functions
      const options: { value?: bigint } = {};
      if (abiFragment.stateMutability === 'payable') {
        const ethValue = inputs['__ethValue'] || '0';
        options.value = parseUnits(ethValue, 18);
      }

      const tx = await contract[abiFragment.name!](...args, options);
      setTxHash(tx.hash);
      await tx.wait();
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'Unknown error';
      setError(errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (type === 'read') {
      executeRead();
    } else {
      executeWrite();
    }
  };

  return (
    <div className="border border-neutral-200 rounded-lg overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between bg-neutral-50 hover:bg-neutral-100 transition-colors text-left"
      >
        <span className="font-medium text-neutral-900">
          {index}. {abiFragment.name}
          {abiFragment.stateMutability === 'payable' && (
            <span className="ml-2 text-xs text-amber-600">(payable)</span>
          )}
        </span>
        {isExpanded ? (
          <ChevronUp className="w-4 h-4 text-neutral-400" />
        ) : (
          <ChevronDown className="w-4 h-4 text-neutral-400" />
        )}
      </button>

      {isExpanded && (
        <form onSubmit={handleSubmit} className="p-4 space-y-4">
          {/* Inputs */}
          {(abiFragment.inputs || []).map((input) => (
            <div key={input.name} className="space-y-1">
              <label className="text-sm text-neutral-500">
                {input.name} <span className="text-xs">({input.type})</span>
              </label>
              <input
                type="text"
                value={inputs[input.name] || ''}
                onChange={(e) => handleInputChange(input.name, e.target.value)}
                placeholder={`${input.type}`}
                className="w-full px-3 py-2 border border-neutral-200 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
            </div>
          ))}

          {/* ETH value for payable functions */}
          {abiFragment.stateMutability === 'payable' && (
            <div className="space-y-1">
              <label className="text-sm text-neutral-500">
                ETH Value <span className="text-xs">(wei)</span>
              </label>
              <input
                type="text"
                value={inputs['__ethValue'] || ''}
                onChange={(e) => handleInputChange('__ethValue', e.target.value)}
                placeholder="0.0"
                className="w-full px-3 py-2 border border-neutral-200 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
            </div>
          )}

          {/* Outputs display */}
          {(abiFragment.outputs || []).length > 0 && (
            <div className="text-xs text-neutral-400">
              Returns: {(abiFragment.outputs || []).map((o) => `${o.name || 'unnamed'} (${o.type})`).join(', ')}
            </div>
          )}

          {/* Submit button */}
          <button
            type="submit"
            disabled={isLoading}
            className="btn-primary flex items-center gap-2 text-sm"
          >
            {isLoading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                {type === 'read' ? 'Reading...' : 'Sending...'}
              </>
            ) : type === 'read' ? (
              'Query'
            ) : (
              <>
                <Wallet className="w-4 h-4" />
                Write
              </>
            )}
          </button>

          {/* Result */}
          {result !== null && (
            <div className="p-3 bg-success-50 border border-success-200 rounded-lg">
              <div className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-success-600 mt-0.5" />
                <div className="font-mono text-sm text-neutral-900 break-all">{result}</div>
              </div>
            </div>
          )}

          {/* Transaction hash */}
          {txHash && (
            <div className="p-3 bg-primary-50 border border-primary-200 rounded-lg">
              <div className="text-sm">
                <span className="text-neutral-500">Transaction: </span>
                <a
                  href={`/tx/${txHash}`}
                  className="font-mono text-primary hover:text-primary-600 break-all transition-colors"
                >
                  {txHash}
                </a>
              </div>
            </div>
          )}

          {/* Error */}
          {error && (
            <div className="p-3 bg-error-50 border border-error-200 rounded-lg">
              <div className="flex items-start gap-2">
                <AlertCircle className="w-4 h-4 text-error-600 mt-0.5" />
                <div className="text-sm text-error-600 break-all">{error}</div>
              </div>
            </div>
          )}
        </form>
      )}
    </div>
  );
}

// Helper to parse input values based on Solidity type
function parseInputValue(value: string, type: string): unknown {
  if (!value) return value;

  // Handle arrays
  if (type.endsWith('[]')) {
    try {
      return JSON.parse(value);
    } catch {
      return value.split(',').map((v) => v.trim());
    }
  }

  // Handle bool
  if (type === 'bool') {
    return value.toLowerCase() === 'true' || value === '1';
  }

  // Handle bytes
  if (type.startsWith('bytes')) {
    return value.startsWith('0x') ? value : `0x${value}`;
  }

  // Handle address
  if (type === 'address') {
    return value;
  }

  // Handle integers - return as string for ethers to parse
  if (type.startsWith('uint') || type.startsWith('int')) {
    return value;
  }

  return value;
}

// Helper to format results
function formatResult(result: unknown, outputs: AbiInput[]): string {
  if (result === null || result === undefined) {
    return 'null';
  }

  if (typeof result === 'bigint') {
    return result.toString();
  }

  if (Array.isArray(result)) {
    if (outputs.length > 1) {
      // Multiple outputs - format as named values
      return outputs
        .map((output, i) => `${output.name || `[${i}]`}: ${formatResult(result[i], [])}`)
        .join('\n');
    }
    return JSON.stringify(result.map((r) => (typeof r === 'bigint' ? r.toString() : r)));
  }

  if (typeof result === 'object') {
    return JSON.stringify(result, (_, v) => (typeof v === 'bigint' ? v.toString() : v), 2);
  }

  return String(result);
}

// Window.ethereum type declaration is in lib/auth.tsx
