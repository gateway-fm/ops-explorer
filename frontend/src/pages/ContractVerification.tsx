import { useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { AlertCircle, AlertTriangle, CheckCircle, Info, Loader2, Search, Upload } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';

const SOLC_VERSIONS = [
  'v0.8.28+commit.7893614a',
  'v0.8.27+commit.40a35a09',
  'v0.8.26+commit.8a97fa7a',
  'v0.8.25+commit.b61c2a91',
  'v0.8.24+commit.e11b9ed9',
  'v0.8.23+commit.f704f362',
  'v0.8.22+commit.4fc1097e',
  'v0.8.21+commit.d9974bed',
  'v0.8.20+commit.a1b79de6',
  'v0.8.19+commit.7dd6d404',
  'v0.8.18+commit.87f61d96',
  'v0.8.17+commit.8df45f5f',
  'v0.8.16+commit.07a7930e',
  'v0.8.15+commit.e14f2714',
  'v0.8.14+commit.80d49f37',
  'v0.8.13+commit.abaa5c0e',
  'v0.8.12+commit.f00d7308',
  'v0.8.11+commit.d7f03943',
  'v0.8.10+commit.fc410830',
  'v0.8.9+commit.e5eed63a',
  'v0.8.8+commit.dddeac2f',
  'v0.8.7+commit.e28d00a7',
  'v0.8.6+commit.11564f7e',
  'v0.8.5+commit.a4f2e591',
  'v0.8.4+commit.c7e474f2',
  'v0.8.3+commit.8d00100c',
  'v0.8.2+commit.661d1103',
  'v0.8.1+commit.df193b15',
  'v0.8.0+commit.c7dfd78e',
  'v0.7.6+commit.7338295f',
  'v0.7.5+commit.eb77ed08',
  'v0.6.12+commit.27d51765',
  'v0.6.6+commit.6c089d02',
  'v0.5.17+commit.d19bba13',
  'v0.4.26+commit.4563c3fc',
];

export default function ContractVerification() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const initialAddress = searchParams.get('address') || '';

  const [address, setAddress] = useState(initialAddress);
  const [chainId, setChainId] = useState('1001');
  const [sourceCode, setSourceCode] = useState('');
  const [contractName, setContractName] = useState('');
  const [compilerVersion, setCompilerVersion] = useState('v0.8.20+commit.a1b79de6');
  const [optimizationUsed, setOptimizationUsed] = useState(false);
  const [runs, setRuns] = useState(200);
  const [activeTab, setActiveTab] = useState<'verify' | 'fetch'>('verify');

  // Check if contract is already verified on Sourcify
  const { data: sourcifyStatus, refetch: refetchStatus } = useQuery({
    queryKey: ['sourcify-check', address, chainId],
    queryFn: () => api.checkSourcify(address, chainId),
    enabled: address.length === 42 && address.startsWith('0x'),
    retry: false,
  });

  // Verify mutation
  const verifyMutation = useMutation({
    mutationFn: () =>
      api.verifySourcify({
        address,
        chainId,
        sourceCode,
        contractName,
        compilerVersion,
        optimizationUsed,
        runs,
      }),
    onSuccess: () => {
      refetchStatus();
    },
  });

  // Fetch from Sourcify mutation
  const fetchMutation = useMutation({
    mutationFn: () => api.fetchFromSourcify(address, chainId),
    onSuccess: (data) => {
      // Navigate to contract page after successful fetch
      navigate(`/address/${data.address}`);
    },
  });

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      setSourceCode(event.target?.result as string);
    };
    reader.readAsText(file);
  };

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-neutral-900">Contract Verification</h1>
        <p className="text-neutral-500 mt-1">
          Verify contract source code or fetch verified contracts from Sourcify
        </p>
      </div>

      {/* Address Input */}
      <div className="card p-6 space-y-4">
        <div className="space-y-2">
          <label className="text-sm font-medium text-neutral-700">Contract Address</label>
          <input
            type="text"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder="0x..."
            className="input font-mono"
          />
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium text-neutral-700">Chain ID</label>
          <select
            value={chainId}
            onChange={(e) => setChainId(e.target.value)}
            className="select"
          >
            <option value="1001">Local Dev Chain (1001)</option>
            <option value="1">Ethereum Mainnet (1)</option>
            <option value="11155111">Sepolia (11155111)</option>
            <option value="137">Polygon (137)</option>
            <option value="42161">Arbitrum One (42161)</option>
            <option value="10">Optimism (10)</option>
            <option value="8453">Base (8453)</option>
            <option value="31337">Hardhat/Anvil (31337)</option>
            <option value="1337">Ganache (1337)</option>
          </select>
        </div>

        {/* Sourcify Status */}
        {sourcifyStatus && (
          <div
            className={`p-3 rounded-lg ${
              sourcifyStatus.isVerified
                ? 'alert-success'
                : 'bg-neutral-50 border border-neutral-200'
            }`}
          >
            <div className="flex items-center gap-2">
              {sourcifyStatus.isVerified ? (
                <>
                  <CheckCircle className="w-4 h-4 text-success-600" />
                  <span className="text-sm text-success-700">
                    Contract is verified on Sourcify ({sourcifyStatus.status})
                  </span>
                </>
              ) : (
                <>
                  <AlertCircle className="w-4 h-4 text-neutral-400" />
                  <span className="text-sm text-neutral-500">Contract not verified on Sourcify</span>
                </>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="card overflow-hidden">
        <div className="tabs">
          <button
            onClick={() => setActiveTab('verify')}
            className={activeTab === 'verify' ? 'tab-active' : 'tab'}
          >
            Submit Verification
          </button>
          <button
            onClick={() => setActiveTab('fetch')}
            className={activeTab === 'fetch' ? 'tab-active' : 'tab'}
          >
            Fetch from Sourcify
          </button>
        </div>

        {/* Verify Tab */}
        {activeTab === 'verify' && (
          <div className="p-6 space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-neutral-700">Source Code</label>
                <label className="cursor-pointer text-sm text-primary hover:text-primary-600 flex items-center gap-1 transition-colors">
                  <Upload className="w-4 h-4" />
                  Upload .sol file
                  <input
                    type="file"
                    accept=".sol"
                    onChange={handleFileUpload}
                    className="hidden"
                  />
                </label>
              </div>
              <textarea
                value={sourceCode}
                onChange={(e) => setSourceCode(e.target.value)}
                placeholder="// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract MyContract {
    ...
}"
                rows={12}
                className="input font-mono resize-y"
              />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Contract Name</label>
                <input
                  type="text"
                  value={contractName}
                  onChange={(e) => setContractName(e.target.value)}
                  placeholder="MyContract"
                  className="input"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Compiler Version</label>
                <select
                  value={compilerVersion}
                  onChange={(e) => setCompilerVersion(e.target.value)}
                  className="select"
                >
                  {SOLC_VERSIONS.map((v) => (
                    <option key={v} value={v}>{v}</option>
                  ))}
                </select>
              </div>
            </div>

            <div className="flex items-center gap-6">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={optimizationUsed}
                  onChange={(e) => setOptimizationUsed(e.target.checked)}
                  className="checkbox"
                />
                <span className="text-sm text-neutral-700">Optimization</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className="w-3.5 h-3.5 text-neutral-400 cursor-help" />
                  </TooltipTrigger>
                  <TooltipContent side="top" className="max-w-[240px]">
                    Enables the Solidity optimizer, which reduces gas costs and contract size. Must match the settings used during deployment.
                  </TooltipContent>
                </Tooltip>
              </label>

              {optimizationUsed && (
                <div className="flex items-center gap-2">
                  <label className="text-sm text-neutral-500">Runs:</label>
                  <input
                    type="number"
                    value={runs}
                    onChange={(e) => setRuns(parseInt(e.target.value) || 200)}
                    className="input w-24 py-1.5"
                  />
                </div>
              )}
            </div>

            {verifyMutation.error && (
              <div className="alert alert-error">
                <div className="flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 mt-0.5" />
                  <span className="text-sm">{(verifyMutation.error as Error).message}</span>
                </div>
              </div>
            )}

            {verifyMutation.isSuccess && verifyMutation.data?.success && (
              <div className="alert alert-success">
                <div className="flex items-center gap-2">
                  <CheckCircle className="w-4 h-4" />
                  <span className="text-sm">
                    Contract verified successfully! Status: {verifyMutation.data?.status}
                  </span>
                </div>
              </div>
            )}

            {verifyMutation.isSuccess && !verifyMutation.data?.success && (
              <div className="alert alert-error">
                <div className="flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4" />
                  <span className="text-sm">
                    Verification failed: {verifyMutation.data?.error || 'Unknown error'}
                  </span>
                </div>
              </div>
            )}

            <button
              onClick={() => verifyMutation.mutate()}
              disabled={!address || !sourceCode || !contractName || verifyMutation.isPending}
              className="btn-primary"
            >
              {verifyMutation.isPending ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Verifying...
                </>
              ) : (
                'Submit Verification'
              )}
            </button>
          </div>
        )}

        {/* Fetch Tab */}
        {activeTab === 'fetch' && (
          <div className="p-6 space-y-4">
            <p className="text-sm text-neutral-500">
              If this contract is already verified on Sourcify, you can fetch the ABI and source code automatically.
            </p>

            {fetchMutation.error && (
              <div className="alert alert-error">
                <div className="flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 mt-0.5" />
                  <span className="text-sm">{(fetchMutation.error as Error).message}</span>
                </div>
              </div>
            )}

            {fetchMutation.isSuccess && (
              <div className="alert alert-success">
                <div className="flex items-center gap-2">
                  <CheckCircle className="w-4 h-4" />
                  <span className="text-sm">
                    Contract fetched: {fetchMutation.data?.contractName} ({fetchMutation.data?.abiLength} ABI entries)
                  </span>
                </div>
              </div>
            )}

            <button
              onClick={() => fetchMutation.mutate()}
              disabled={!address || fetchMutation.isPending}
              className="btn-primary"
            >
              {fetchMutation.isPending ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Fetching...
                </>
              ) : (
                <>
                  <Search className="w-4 h-4" />
                  Fetch from Sourcify
                </>
              )}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
