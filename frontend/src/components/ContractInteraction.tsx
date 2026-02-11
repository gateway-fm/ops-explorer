import { useState, useMemo } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { BrowserProvider, Contract, parseUnits } from 'ethers';
import type { Eip1193Provider } from 'ethers';
import { api } from '../lib/api';
import type { AbiFragment, AbiInput } from '../lib/api';
import { ChevronDown, ChevronUp, Loader2, Wallet, Upload, AlertCircle, CheckCircle } from 'lucide-react';

// Extend Window interface for ethereum provider
declare global {
  interface Window {
    ethereum?: Eip1193Provider;
  }
}

// Get RPC URL from environment
const RPC_URL = import.meta.env.VITE_RPC_URL || 'http://localhost:8123';

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
      <div className="text-center text-white/50 py-8">
        No ABI available. Upload an ABI to interact with this contract.
      </div>
    );
  }

  if (functions.length === 0) {
    return (
      <div className="text-center text-white/50 py-8">
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
    <div className="border border-white/10 rounded-lg overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between bg-white/5 hover:bg-white/10 transition-colors text-left"
      >
        <span className="font-medium text-white/95">
          {index}. {abiFragment.name}
          {abiFragment.stateMutability === 'payable' && (
            <span className="ml-2 text-xs text-amber-400">(payable)</span>
          )}
        </span>
        {isExpanded ? (
          <ChevronUp className="w-4 h-4 text-white/50" />
        ) : (
          <ChevronDown className="w-4 h-4 text-white/50" />
        )}
      </button>

      {isExpanded && (
        <form onSubmit={handleSubmit} className="p-4 space-y-4 bg-white/[0.02]">
          {/* Inputs */}
          {(abiFragment.inputs || []).map((input) => (
            <div key={input.name} className="space-y-1">
              <label className="text-sm text-white/60">
                {input.name} <span className="text-xs">({input.type})</span>
              </label>
              <input
                type="text"
                value={inputs[input.name] || ''}
                onChange={(e) => handleInputChange(input.name, e.target.value)}
                placeholder={`${input.type}`}
                className="glass-input w-full"
              />
            </div>
          ))}

          {/* ETH value for payable functions */}
          {abiFragment.stateMutability === 'payable' && (
            <div className="space-y-1">
              <label className="text-sm text-white/60">
                ETH Value <span className="text-xs">(wei)</span>
              </label>
              <input
                type="text"
                value={inputs['__ethValue'] || ''}
                onChange={(e) => handleInputChange('__ethValue', e.target.value)}
                placeholder="0.0"
                className="glass-input w-full"
              />
            </div>
          )}

          {/* Outputs display */}
          {(abiFragment.outputs || []).length > 0 && (
            <div className="text-xs text-white/50">
              Returns: {(abiFragment.outputs || []).map((o) => `${o.name || 'unnamed'} (${o.type})`).join(', ')}
            </div>
          )}

          {/* Submit button */}
          <button
            type="submit"
            disabled={isLoading}
            className="glass-button flex items-center gap-2"
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
            <div className="p-3 bg-green-500/10 border border-green-500/20 rounded-lg">
              <div className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <div className="font-mono text-sm text-white/90 break-all">{result}</div>
              </div>
            </div>
          )}

          {/* Transaction hash */}
          {txHash && (
            <div className="p-3 bg-blue-500/10 border border-blue-500/20 rounded-lg">
              <div className="text-sm">
                <span className="text-white/60">Transaction: </span>
                <a
                  href={`/tx/${txHash}`}
                  className="font-mono text-blue-400 hover:text-blue-300 break-all transition-colors"
                >
                  {txHash}
                </a>
              </div>
            </div>
          )}

          {/* Error */}
          {error && (
            <div className="p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
              <div className="flex items-start gap-2">
                <AlertCircle className="w-4 h-4 text-red-400 mt-0.5" />
                <div className="text-sm text-red-400 break-all">{error}</div>
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

// ABI Upload component
interface AbiUploadProps {
  address: string;
  onAbiUpdate: () => void;
}

export function AbiUpload({ address, onAbiUpdate }: AbiUploadProps) {
  const [abiText, setAbiText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async (abi: AbiFragment[]) => {
      return api.updateContractABI(address, abi);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['contract', address] });
      setAbiText('');
      setError(null);
      onAbiUpdate();
    },
    onError: (err: Error) => {
      setError(err.message);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    try {
      const parsed = JSON.parse(abiText);
      if (!Array.isArray(parsed)) {
        throw new Error('ABI must be a JSON array');
      }
      mutation.mutate(parsed);
    } catch (err) {
      if (err instanceof SyntaxError) {
        setError('Invalid JSON format');
      } else if (err instanceof Error) {
        setError(err.message);
      }
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      const content = event.target?.result as string;
      try {
        // Try to parse as JSON and extract ABI if it's a full artifact
        const parsed = JSON.parse(content);
        if (Array.isArray(parsed)) {
          setAbiText(JSON.stringify(parsed, null, 2));
        } else if (parsed.abi && Array.isArray(parsed.abi)) {
          // Hardhat/Truffle artifact format
          setAbiText(JSON.stringify(parsed.abi, null, 2));
        } else {
          setAbiText(content);
        }
        setError(null);
      } catch {
        setAbiText(content);
      }
    };
    reader.readAsText(file);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-white/95">Upload Contract ABI</h3>
        <label className="cursor-pointer text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1 transition-colors">
          <Upload className="w-4 h-4" />
          Upload JSON
          <input
            type="file"
            accept=".json"
            onChange={handleFileUpload}
            className="hidden"
          />
        </label>
      </div>

      <form onSubmit={handleSubmit} className="space-y-3">
        <textarea
          value={abiText}
          onChange={(e) => {
            setAbiText(e.target.value);
            setError(null);
          }}
          placeholder='Paste ABI JSON here, e.g. [{"type":"function","name":"balanceOf",...}]'
          rows={8}
          className="glass-input w-full font-mono text-sm resize-y"
        />

        {error && (
          <div className="p-2 bg-red-500/10 border border-red-500/20 rounded-lg">
            <div className="flex items-center gap-2 text-sm text-red-400">
              <AlertCircle className="w-4 h-4" />
              {error}
            </div>
          </div>
        )}

        <button
          type="submit"
          disabled={!abiText.trim() || mutation.isPending}
          className="glass-button flex items-center gap-2"
        >
          {mutation.isPending ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              Saving...
            </>
          ) : (
            'Save ABI'
          )}
        </button>
      </form>
    </div>
  );
}

// Window.ethereum type declaration is in lib/auth.tsx
