import { useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { AlertCircle, AlertTriangle, CheckCircle, ChevronDown, ChevronRight, Info, Loader2, Plus, Trash2, Upload } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';

const LICENSE_TYPES = [
  'No License',
  'Unlicense',
  'MIT',
  'GNU GPLv2',
  'GNU GPLv3',
  'GNU LGPLv2.1',
  'GNU LGPLv3',
  'BSD-2-Clause',
  'BSD-3-Clause',
  'MPL-2.0',
  'OSL-3.0',
  'Apache-2.0',
  'GNU AGPLv3',
  'BSL 1.1',
];

const EVM_VERSIONS = [
  'default',
  'homestead',
  'tangerineWhistle',
  'spuriousDragon',
  'byzantium',
  'constantinople',
  'petersburg',
  'istanbul',
  'berlin',
  'london',
  'paris',
  'shanghai',
  'cancun',
];

type VerificationMethod = 'source' | 'standard-json';

export default function ContractVerification() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const initialAddress = searchParams.get('address') || '';

  // Verification method tab
  const [method, setMethod] = useState<VerificationMethod>('source');

  // Form state (shared)
  const [address, setAddress] = useState(initialAddress);
  const [license, setLicense] = useState('No License');
  const [contractName, setContractName] = useState('');
  const [compilerVersion, setCompilerVersion] = useState('');
  const [constructorArgs, setConstructorArgs] = useState('');

  // Source code form state
  const [sourceCode, setSourceCode] = useState('');
  const [evmVersion, setEvmVersion] = useState('default');
  const [optimizationUsed, setOptimizationUsed] = useState(false);
  const [runs, setRuns] = useState(200);
  const [libraries, setLibraries] = useState<{ name: string; address: string }[]>([]);
  const [showLibraries, setShowLibraries] = useState(false);

  // Standard JSON form state
  const [standardJsonInput, setStandardJsonInput] = useState('');
  const [contractFile, setContractFile] = useState('');

  const isValidAddress = address.length === 42 && address.startsWith('0x');

  // Fetch compiler versions from API
  const { data: compilerData } = useQuery({
    queryKey: ['compiler-versions'],
    queryFn: () => api.getCompilerVersions(),
  });

  // Auto-check Sourcify status when address is valid
  const { data: sourcifyStatus } = useQuery({
    queryKey: ['sourcify-check', address],
    queryFn: () => api.checkSourcify(address),
    enabled: isValidAddress,
    retry: false,
  });

  // Fetch from Sourcify mutation
  const fetchMutation = useMutation({
    mutationFn: () => api.fetchFromSourcify(address),
    onSuccess: (data) => {
      navigate(`/address/${data.address}`);
    },
  });

  // Verify mutation
  const verifyMutation = useMutation({
    mutationFn: () => {
      const libMap: Record<string, string> = {};
      for (const lib of libraries) {
        if (lib.name && lib.address) {
          libMap[lib.name] = lib.address;
        }
      }
      return api.verifyContract({
        address,
        sourceCode,
        contractName,
        compilerVersion,
        evmVersion: evmVersion !== 'default' ? evmVersion : undefined,
        optimizationUsed,
        optimizationRuns: runs,
        constructorArgs: constructorArgs || undefined,
        libraries: Object.keys(libMap).length > 0 ? libMap : undefined,
        licenseType: license !== 'No License' ? license : undefined,
      });
    },
  });

  // Standard JSON verify mutation
  const verifyStandardJsonMutation = useMutation({
    mutationFn: () => {
      let parsed: unknown;
      try {
        parsed = JSON.parse(standardJsonInput);
      } catch {
        throw new Error('Invalid JSON input');
      }
      return api.verifyContractStandardJSON({
        address,
        compilerVersion,
        contractName,
        contractFile: contractFile || undefined,
        standardInput: parsed,
        constructorArgs: constructorArgs || undefined,
        licenseType: license !== 'No License' ? license : undefined,
      });
    },
  });

  const activeMutation = method === 'source' ? verifyMutation : verifyStandardJsonMutation;

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      setSourceCode(event.target?.result as string);
    };
    reader.readAsText(file);
  };

  const handleJsonFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      setStandardJsonInput(event.target?.result as string);
    };
    reader.readAsText(file);
  };

  const addLibrary = () => {
    if (libraries.length < 10) {
      setLibraries([...libraries, { name: '', address: '' }]);
    }
  };

  const removeLibrary = (index: number) => {
    setLibraries(libraries.filter((_, i) => i !== index));
  };

  const updateLibrary = (index: number, field: 'name' | 'address', value: string) => {
    const updated = [...libraries];
    updated[index] = { ...updated[index], [field]: value };
    setLibraries(updated);
  };

  const compilerVersions = compilerData?.versions ?? [];

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-neutral-900">Contract Verification</h1>
        <p className="text-neutral-500 mt-1">
          Verify and publish your contract source code
        </p>
      </div>

      {/* Step 1: Contract Address + Sourcify Check */}
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

        {/* Sourcify Status Banner */}
        {sourcifyStatus?.isVerified && (
          <div className="p-3 rounded-lg alert-success">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <CheckCircle className="w-4 h-4 text-success-600" />
                <span className="text-sm text-success-700">
                  This contract is already verified on Sourcify ({sourcifyStatus.status})
                </span>
              </div>
              <button
                onClick={() => fetchMutation.mutate()}
                disabled={fetchMutation.isPending}
                className="btn-secondary text-xs"
              >
                {fetchMutation.isPending ? (
                  <>
                    <Loader2 className="w-3 h-3 animate-spin" />
                    Importing...
                  </>
                ) : (
                  'Fetch & Import'
                )}
              </button>
            </div>
            {fetchMutation.error && (
              <div className="mt-2 text-sm text-error-600">
                {(fetchMutation.error as Error).message}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Verification Method Tabs */}
      <div className="card p-1 flex gap-1">
        <button
          onClick={() => setMethod('source')}
          className={`flex-1 py-2 px-4 rounded-md text-sm font-medium transition-colors ${
            method === 'source'
              ? 'bg-primary text-white'
              : 'text-neutral-600 hover:bg-neutral-100'
          }`}
        >
          Solidity Source Code
        </button>
        <button
          onClick={() => setMethod('standard-json')}
          className={`flex-1 py-2 px-4 rounded-md text-sm font-medium transition-colors ${
            method === 'standard-json'
              ? 'bg-primary text-white'
              : 'text-neutral-600 hover:bg-neutral-100'
          }`}
        >
          Standard JSON Input
        </button>
      </div>

      {method === 'source' && (
        <>
          {/* Step 2: Verification Settings */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Verification Settings</h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Contract License</label>
                <select
                  value={license}
                  onChange={(e) => setLicense(e.target.value)}
                  className="select"
                >
                  {LICENSE_TYPES.map((l) => (
                    <option key={l} value={l}>{l}</option>
                  ))}
                </select>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Compiler Version</label>
                <select
                  value={compilerVersion}
                  onChange={(e) => setCompilerVersion(e.target.value)}
                  className="select"
                >
                  <option value="">Select compiler version</option>
                  {compilerVersions.map((v) => (
                    <option key={v.version} value={v.version}>{v.longVersion || v.version}</option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">EVM Version</label>
                <select
                  value={evmVersion}
                  onChange={(e) => setEvmVersion(e.target.value)}
                  className="select"
                >
                  {EVM_VERSIONS.map((v) => (
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
          </div>

          {/* Step 3: Source Code */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Source Code</h2>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-neutral-700">Solidity Source Code</label>
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
                <label className="text-sm font-medium text-neutral-700">
                  Constructor Arguments
                  <span className="text-neutral-400 font-normal ml-1">(optional)</span>
                </label>
                <input
                  type="text"
                  value={constructorArgs}
                  onChange={(e) => setConstructorArgs(e.target.value)}
                  placeholder="ABI-encoded hex (e.g., 0x00000000...)"
                  className="input font-mono"
                />
              </div>
            </div>
          </div>

          {/* Step 4: Libraries (collapsible) */}
          <div className="card overflow-hidden">
            <button
              onClick={() => setShowLibraries(!showLibraries)}
              className="w-full px-6 py-4 flex items-center justify-between text-left hover:bg-neutral-50 transition-colors"
            >
              <div className="flex items-center gap-2">
                {showLibraries ? (
                  <ChevronDown className="w-4 h-4 text-neutral-400" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-neutral-400" />
                )}
                <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">
                  Contract Libraries
                </h2>
                <span className="text-xs text-neutral-400 font-normal normal-case">(optional)</span>
              </div>
              {libraries.length > 0 && (
                <span className="text-xs text-neutral-500">{libraries.length} added</span>
              )}
            </button>

            {showLibraries && (
              <div className="px-6 pb-6 space-y-3">
                {libraries.map((lib, index) => (
                  <div key={index} className="flex items-start gap-3">
                    <div className="flex-1 grid grid-cols-1 md:grid-cols-2 gap-3">
                      <input
                        type="text"
                        value={lib.name}
                        onChange={(e) => updateLibrary(index, 'name', e.target.value)}
                        placeholder="Library name (e.g., SafeMath)"
                        className="input"
                      />
                      <input
                        type="text"
                        value={lib.address}
                        onChange={(e) => updateLibrary(index, 'address', e.target.value)}
                        placeholder="Library address (0x...)"
                        className="input font-mono"
                      />
                    </div>
                    <button
                      onClick={() => removeLibrary(index)}
                      className="p-2 text-neutral-400 hover:text-error-500 transition-colors mt-0.5"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                ))}
                {libraries.length < 10 && (
                  <button
                    onClick={addLibrary}
                    className="flex items-center gap-1.5 text-sm text-primary hover:text-primary-600 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    Add Library
                  </button>
                )}
                {libraries.length === 0 && (
                  <p className="text-sm text-neutral-400">
                    If your contract uses external libraries, add their deployed addresses here.
                  </p>
                )}
              </div>
            )}
          </div>
        </>
      )}

      {method === 'standard-json' && (
        <>
          {/* Standard JSON Settings */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Compiler Settings</h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Compiler Version</label>
                <select
                  value={compilerVersion}
                  onChange={(e) => setCompilerVersion(e.target.value)}
                  className="select"
                >
                  <option value="">Select compiler version</option>
                  {compilerVersions.map((v) => (
                    <option key={v.version} value={v.version}>{v.longVersion || v.version}</option>
                  ))}
                </select>
              </div>

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
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">
                  Contract File Path
                  <span className="text-neutral-400 font-normal ml-1">(optional)</span>
                </label>
                <input
                  type="text"
                  value={contractFile}
                  onChange={(e) => setContractFile(e.target.value)}
                  placeholder="contracts/MyContract.sol"
                  className="input font-mono"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">
                  Constructor Arguments
                  <span className="text-neutral-400 font-normal ml-1">(optional)</span>
                </label>
                <input
                  type="text"
                  value={constructorArgs}
                  onChange={(e) => setConstructorArgs(e.target.value)}
                  placeholder="ABI-encoded hex (e.g., 0x00000000...)"
                  className="input font-mono"
                />
              </div>
            </div>
          </div>

          {/* Standard JSON Input */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Standard JSON Input</h2>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-neutral-700">Standard JSON Input</label>
                <label className="cursor-pointer text-sm text-primary hover:text-primary-600 flex items-center gap-1 transition-colors">
                  <Upload className="w-4 h-4" />
                  Upload .json file
                  <input
                    type="file"
                    accept=".json"
                    onChange={handleJsonFileUpload}
                    className="hidden"
                  />
                </label>
              </div>
              <textarea
                value={standardJsonInput}
                onChange={(e) => setStandardJsonInput(e.target.value)}
                placeholder='{"language": "Solidity", "sources": { ... }, "settings": { ... }}'
                rows={14}
                className="input font-mono resize-y"
              />
              <p className="text-xs text-neutral-400">
                Paste the standard JSON input or upload a .json file. This is the same format used by solc --standard-json, and can be exported from Hardhat, Foundry, or Truffle.
              </p>
            </div>
          </div>
        </>
      )}

      {/* Submit */}
      <div className="card p-6 space-y-4">
        {activeMutation.error && (
          <div className="alert alert-error">
            <div className="flex items-start gap-3">
              <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
              <span>{(activeMutation.error as Error).message}</span>
            </div>
          </div>
        )}

        {activeMutation.isSuccess && activeMutation.data?.success && (
          <div className="alert alert-success">
            <div className="flex items-center gap-2">
              <CheckCircle className="w-4 h-4" />
              <span className="text-sm">Contract verified successfully!</span>
            </div>
            <button
              onClick={() => navigate(`/address/${address}`)}
              className="btn-secondary text-xs mt-2"
            >
              View Contract
            </button>
          </div>
        )}

        {activeMutation.isSuccess && !activeMutation.data?.success && (
          <div className="alert alert-error">
            <div className="flex items-start gap-3">
              <AlertTriangle className="w-5 h-5 shrink-0 mt-0.5" />
              <span>
                Verification failed: {activeMutation.data?.error || 'Unknown error'}
              </span>
            </div>
          </div>
        )}

        <button
          onClick={() => activeMutation.mutate()}
          disabled={
            !address || !contractName || !compilerVersion || activeMutation.isPending ||
            (method === 'source' && !sourceCode) ||
            (method === 'standard-json' && !standardJsonInput)
          }
          className="btn-primary w-full"
        >
          {activeMutation.isPending ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              Verifying...
            </>
          ) : (
            'Verify & Publish'
          )}
        </button>
      </div>
    </div>
  );
}
