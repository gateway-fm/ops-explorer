import { useState, useRef, useCallback } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { AlertCircle, AlertTriangle, CheckCircle, ChevronDown, ChevronRight, Info, Loader2, Plus, Trash2, Upload, X, FileText } from 'lucide-react';
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

type VerificationMethod = 'source' | 'multi-part' | 'standard-json' | 'hardhat-foundry';

interface UploadedFile {
  name: string;
  size: number;
  content: string;
}

interface HardhatParsed {
  standardInput: unknown;
  compilerVersion?: string;
  contracts: { file: string; name: string }[];
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function parseHardhatArtifact(raw: string): HardhatParsed {
  const parsed = JSON.parse(raw);
  const contracts: { file: string; name: string }[] = [];

  // Hardhat build-info has an `input` field; Foundry standard JSON does not
  const hasInput = parsed.input && typeof parsed.input === 'object';
  const standardInput = hasInput ? parsed.input : parsed;

  // Extract compiler version
  const compilerVersion = parsed.solcLongVersion || parsed.solcVersion || undefined;

  // Extract contract names from output.contracts
  if (parsed.output?.contracts) {
    for (const [filePath, fileContracts] of Object.entries(parsed.output.contracts)) {
      for (const contractName of Object.keys(fileContracts as Record<string, unknown>)) {
        contracts.push({ file: filePath, name: contractName });
      }
    }
  }

  return { standardInput, compilerVersion, contracts };
}

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

  // Multi-part form state
  const [multiPartFiles, setMultiPartFiles] = useState<UploadedFile[]>([]);
  const [multiPartMainFile, setMultiPartMainFile] = useState('');
  const [multiPartContractName, setMultiPartContractName] = useState('');
  const [multiPartEvmVersion, setMultiPartEvmVersion] = useState('default');
  const [multiPartOptimization, setMultiPartOptimization] = useState(false);
  const [multiPartRuns, setMultiPartRuns] = useState(200);
  const [multiPartLibraries, setMultiPartLibraries] = useState<{ name: string; address: string }[]>([]);
  const [showMultiPartLibraries, setShowMultiPartLibraries] = useState(false);
  const [multiPartConstructorArgs, setMultiPartConstructorArgs] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const multiPartFileInputRef = useRef<HTMLInputElement>(null);

  // Hardhat/Foundry form state
  const [hfArtifact, setHfArtifact] = useState<HardhatParsed | null>(null);
  const [hfFileName, setHfFileName] = useState('');
  const [hfCompilerVersion, setHfCompilerVersion] = useState('');
  const [hfContractName, setHfContractName] = useState('');
  const [hfContractFile, setHfContractFile] = useState('');
  const [hfConstructorArgs, setHfConstructorArgs] = useState('');
  const [hfLicense, setHfLicense] = useState('No License');
  const [hfParseError, setHfParseError] = useState('');

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

  // Verify mutation (source tab)
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

  // Multi-part verify mutation
  const verifyMultiPartMutation = useMutation({
    mutationFn: () => {
      const sourceFiles: Record<string, string> = {};
      for (const f of multiPartFiles) {
        sourceFiles[f.name] = f.content;
      }
      const libMap: Record<string, string> = {};
      for (const lib of multiPartLibraries) {
        if (lib.name && lib.address) {
          libMap[lib.name] = lib.address;
        }
      }
      return api.verifyContract({
        address,
        sourceFiles,
        mainContractFile: multiPartMainFile,
        contractName: multiPartContractName,
        compilerVersion,
        evmVersion: multiPartEvmVersion !== 'default' ? multiPartEvmVersion : undefined,
        optimizationUsed: multiPartOptimization,
        optimizationRuns: multiPartRuns,
        constructorArgs: multiPartConstructorArgs || undefined,
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

  // Hardhat/Foundry verify mutation
  const verifyHardhatFoundryMutation = useMutation({
    mutationFn: () => {
      if (!hfArtifact) throw new Error('No artifact uploaded');
      return api.verifyContractStandardJSON({
        address,
        compilerVersion: hfCompilerVersion,
        contractName: hfContractName,
        contractFile: hfContractFile || undefined,
        standardInput: hfArtifact.standardInput,
        constructorArgs: hfConstructorArgs || undefined,
        licenseType: hfLicense !== 'No License' ? hfLicense : undefined,
      });
    },
  });

  const activeMutation =
    method === 'source' ? verifyMutation :
    method === 'multi-part' ? verifyMultiPartMutation :
    method === 'standard-json' ? verifyStandardJsonMutation :
    verifyHardhatFoundryMutation;

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

  // Multi-part file handling
  const addMultiPartFiles = useCallback((files: FileList | File[]) => {
    const fileArray = Array.from(files).filter(f => f.name.endsWith('.sol'));
    if (fileArray.length === 0) return;

    const readers = fileArray.map(file => {
      return new Promise<UploadedFile>((resolve) => {
        const reader = new FileReader();
        reader.onload = (event) => {
          resolve({
            name: file.name,
            size: file.size,
            content: event.target?.result as string,
          });
        };
        reader.readAsText(file);
      });
    });

    Promise.all(readers).then((newFiles) => {
      setMultiPartFiles(prev => {
        const existingNames = new Set(prev.map(f => f.name));
        const unique = newFiles.filter(f => !existingNames.has(f.name));
        return [...prev, ...unique];
      });
    });
  }, []);

  const removeMultiPartFile = (name: string) => {
    setMultiPartFiles(prev => prev.filter(f => f.name !== name));
    if (multiPartMainFile === name) {
      setMultiPartMainFile('');
    }
  };

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    if (e.dataTransfer.files.length > 0) {
      addMultiPartFiles(e.dataTransfer.files);
    }
  }, [addMultiPartFiles]);

  const handleMultiPartFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      addMultiPartFiles(e.target.files);
    }
    // Reset so the same files can be selected again
    e.target.value = '';
  };

  // Hardhat/Foundry file handling
  const handleHardhatFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setHfParseError('');
    setHfFileName(file.name);

    const reader = new FileReader();
    reader.onload = (event) => {
      const raw = event.target?.result as string;
      try {
        const parsed = parseHardhatArtifact(raw);
        setHfArtifact(parsed);

        // Auto-fill compiler version
        if (parsed.compilerVersion) {
          // Try to find matching version in compilerVersions
          const match = compilerVersions.find(
            v => v.version === parsed.compilerVersion ||
                 v.longVersion === parsed.compilerVersion ||
                 v.version.startsWith(parsed.compilerVersion?.split('+')[0] || '')
          );
          setHfCompilerVersion(match?.version || parsed.compilerVersion);
        }

        // Auto-fill first contract if available
        if (parsed.contracts.length > 0) {
          setHfContractFile(parsed.contracts[0].file);
          setHfContractName(parsed.contracts[0].name);
        }
      } catch {
        setHfParseError('Failed to parse JSON file. Make sure it is a valid Hardhat build-info or Foundry standard JSON file.');
        setHfArtifact(null);
      }
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

  const addMultiPartLibrary = () => {
    if (multiPartLibraries.length < 10) {
      setMultiPartLibraries([...multiPartLibraries, { name: '', address: '' }]);
    }
  };

  const removeMultiPartLibrary = (index: number) => {
    setMultiPartLibraries(multiPartLibraries.filter((_, i) => i !== index));
  };

  const updateMultiPartLibrary = (index: number, field: 'name' | 'address', value: string) => {
    const updated = [...multiPartLibraries];
    updated[index] = { ...updated[index], [field]: value };
    setMultiPartLibraries(updated);
  };

  const compilerVersions = compilerData?.versions ?? [];

  // Compute disabled state for submit
  const isSubmitDisabled = () => {
    if (!address || activeMutation.isPending) return true;
    switch (method) {
      case 'source':
        return !contractName || !compilerVersion || !sourceCode;
      case 'multi-part':
        return !multiPartContractName || !compilerVersion || multiPartFiles.length === 0 || !multiPartMainFile;
      case 'standard-json':
        return !contractName || !compilerVersion || !standardJsonInput;
      case 'hardhat-foundry':
        return !hfContractName || !hfCompilerVersion || !hfArtifact;
    }
  };

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
          Solidity Source
        </button>
        <button
          onClick={() => setMethod('multi-part')}
          className={`flex-1 py-2 px-4 rounded-md text-sm font-medium transition-colors ${
            method === 'multi-part'
              ? 'bg-primary text-white'
              : 'text-neutral-600 hover:bg-neutral-100'
          }`}
        >
          Multi-Part Files
        </button>
        <button
          onClick={() => setMethod('standard-json')}
          className={`flex-1 py-2 px-4 rounded-md text-sm font-medium transition-colors ${
            method === 'standard-json'
              ? 'bg-primary text-white'
              : 'text-neutral-600 hover:bg-neutral-100'
          }`}
        >
          Standard JSON
        </button>
        <button
          onClick={() => setMethod('hardhat-foundry')}
          className={`flex-1 py-2 px-4 rounded-md text-sm font-medium transition-colors ${
            method === 'hardhat-foundry'
              ? 'bg-primary text-white'
              : 'text-neutral-600 hover:bg-neutral-100'
          }`}
        >
          Hardhat / Foundry
        </button>
      </div>

      {/* ========== Tab 1: Solidity Source (existing) ========== */}
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

      {/* ========== Tab 2: Multi-Part Files (NEW) ========== */}
      {method === 'multi-part' && (
        <>
          {/* Verification Settings */}
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
                  value={multiPartEvmVersion}
                  onChange={(e) => setMultiPartEvmVersion(e.target.value)}
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
                  checked={multiPartOptimization}
                  onChange={(e) => setMultiPartOptimization(e.target.checked)}
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

              {multiPartOptimization && (
                <div className="flex items-center gap-2">
                  <label className="text-sm text-neutral-500">Runs:</label>
                  <input
                    type="number"
                    value={multiPartRuns}
                    onChange={(e) => setMultiPartRuns(parseInt(e.target.value) || 200)}
                    className="input w-24 py-1.5"
                  />
                </div>
              )}
            </div>
          </div>

          {/* Source Files */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Source Files</h2>

            {/* Drop Zone */}
            <div
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => multiPartFileInputRef.current?.click()}
              className={`border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors ${
                isDragging
                  ? 'border-primary bg-primary/5'
                  : 'border-neutral-300 hover:border-neutral-400'
              }`}
            >
              <Upload className="w-8 h-8 text-neutral-400 mx-auto mb-2" />
              <p className="text-sm text-neutral-600">
                Drag and drop .sol files here, or click to select
              </p>
              <p className="text-xs text-neutral-400 mt-1">
                Upload all Solidity source files including imports, interfaces, and libraries
              </p>
              <input
                ref={multiPartFileInputRef}
                type="file"
                multiple
                accept=".sol"
                onChange={handleMultiPartFileInput}
                className="hidden"
              />
            </div>

            {/* File List */}
            {multiPartFiles.length > 0 && (
              <div className="space-y-2">
                {multiPartFiles.map((file) => (
                  <div
                    key={file.name}
                    className="flex items-center justify-between px-3 py-2 rounded-md bg-neutral-50 border border-neutral-200"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <FileText className="w-4 h-4 text-neutral-400 shrink-0" />
                      <span className="text-sm text-neutral-700 font-mono truncate">{file.name}</span>
                      <span className="text-xs text-neutral-400 shrink-0">{formatFileSize(file.size)}</span>
                    </div>
                    <button
                      onClick={() => removeMultiPartFile(file.name)}
                      className="p-1 text-neutral-400 hover:text-error-500 transition-colors shrink-0"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Main Contract File</label>
                <select
                  value={multiPartMainFile}
                  onChange={(e) => setMultiPartMainFile(e.target.value)}
                  className="select"
                >
                  <option value="">Select main contract file</option>
                  {multiPartFiles.map((f) => (
                    <option key={f.name} value={f.name}>{f.name}</option>
                  ))}
                </select>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Contract Name</label>
                <input
                  type="text"
                  value={multiPartContractName}
                  onChange={(e) => setMultiPartContractName(e.target.value)}
                  placeholder="MyContract"
                  className="input"
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-neutral-700">
                Constructor Arguments
                <span className="text-neutral-400 font-normal ml-1">(optional)</span>
              </label>
              <input
                type="text"
                value={multiPartConstructorArgs}
                onChange={(e) => setMultiPartConstructorArgs(e.target.value)}
                placeholder="ABI-encoded hex (e.g., 0x00000000...)"
                className="input font-mono"
              />
            </div>
          </div>

          {/* Libraries (collapsible) */}
          <div className="card overflow-hidden">
            <button
              onClick={() => setShowMultiPartLibraries(!showMultiPartLibraries)}
              className="w-full px-6 py-4 flex items-center justify-between text-left hover:bg-neutral-50 transition-colors"
            >
              <div className="flex items-center gap-2">
                {showMultiPartLibraries ? (
                  <ChevronDown className="w-4 h-4 text-neutral-400" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-neutral-400" />
                )}
                <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">
                  Contract Libraries
                </h2>
                <span className="text-xs text-neutral-400 font-normal normal-case">(optional)</span>
              </div>
              {multiPartLibraries.length > 0 && (
                <span className="text-xs text-neutral-500">{multiPartLibraries.length} added</span>
              )}
            </button>

            {showMultiPartLibraries && (
              <div className="px-6 pb-6 space-y-3">
                {multiPartLibraries.map((lib, index) => (
                  <div key={index} className="flex items-start gap-3">
                    <div className="flex-1 grid grid-cols-1 md:grid-cols-2 gap-3">
                      <input
                        type="text"
                        value={lib.name}
                        onChange={(e) => updateMultiPartLibrary(index, 'name', e.target.value)}
                        placeholder="Library name (e.g., SafeMath)"
                        className="input"
                      />
                      <input
                        type="text"
                        value={lib.address}
                        onChange={(e) => updateMultiPartLibrary(index, 'address', e.target.value)}
                        placeholder="Library address (0x...)"
                        className="input font-mono"
                      />
                    </div>
                    <button
                      onClick={() => removeMultiPartLibrary(index)}
                      className="p-2 text-neutral-400 hover:text-error-500 transition-colors mt-0.5"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                ))}
                {multiPartLibraries.length < 10 && (
                  <button
                    onClick={addMultiPartLibrary}
                    className="flex items-center gap-1.5 text-sm text-primary hover:text-primary-600 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    Add Library
                  </button>
                )}
                {multiPartLibraries.length === 0 && (
                  <p className="text-sm text-neutral-400">
                    If your contract uses external libraries, add their deployed addresses here.
                  </p>
                )}
              </div>
            )}
          </div>
        </>
      )}

      {/* ========== Tab 3: Standard JSON (existing) ========== */}
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

      {/* ========== Tab 4: Hardhat / Foundry (NEW) ========== */}
      {method === 'hardhat-foundry' && (
        <>
          {/* CLI Instructions */}
          <div className="card p-6 space-y-5">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Verify via CLI</h2>

            <p className="text-sm text-neutral-600">
              Use Hardhat or Foundry to verify contracts directly from your development environment. This explorer exposes an Etherscan-compatible API.
            </p>

            {/* Foundry */}
            <div className="space-y-2">
              <h3 className="text-sm font-semibold text-neutral-800">Foundry</h3>
              <div className="code-block">
                <pre className="text-xs whitespace-pre-wrap break-all">{`forge verify-contract \\
  --rpc-url <YOUR_RPC_URL> \\
  --verifier blockscout \\
  --verifier-url '${window.location.origin}/api/' \\
  <CONTRACT_ADDRESS> \\
  [src/MyContract.sol:MyContract]`}</pre>
              </div>
              <p className="text-xs text-neutral-400 mt-1">
                Note: <code className="bg-neutral-200 px-1 py-0.5 rounded font-mono text-xs">--verifier blockscout</code> is Foundry's flag for any Etherscan-compatible API — it is not specific to Blockscout.
              </p>
            </div>

            {/* Hardhat */}
            <div className="space-y-2">
              <h3 className="text-sm font-semibold text-neutral-800">Hardhat</h3>
              <p className="text-xs text-neutral-500">Add this to your <code className="bg-neutral-200 px-1 py-0.5 rounded font-mono text-xs">hardhat.config.ts</code>:</p>
              <div className="code-block">
                <pre className="text-xs whitespace-pre-wrap break-all">{`etherscan: {
  apiKey: {
    'custom-network': 'empty'
  },
  customChains: [
    {
      network: "custom-network",
      chainId: ${import.meta.env.VITE_CHAIN_ID || '<CHAIN_ID>'},
      urls: {
        apiURL: "${window.location.origin}/api",
        browserURL: "${window.location.origin}"
      }
    }
  ]
}`}</pre>
              </div>
              <p className="text-xs text-neutral-500 mt-2">Then run:</p>
              <div className="code-block">
                <pre className="text-xs whitespace-pre-wrap break-all">{`npx hardhat verify \\
  --network custom-network \\
  <CONTRACT_ADDRESS> \\
  [...constructorArgs]`}</pre>
              </div>
            </div>

            <div className="p-3 rounded-lg bg-neutral-50 border border-neutral-200">
              <div className="flex items-start gap-2">
                <Info className="w-4 h-4 text-neutral-500 mt-0.5 shrink-0" />
                <p className="text-xs text-neutral-600">
                  The API key can be any non-empty string — no registration required.
                </p>
              </div>
            </div>
          </div>

          {/* Divider */}
          <div className="flex items-center gap-3">
            <div className="flex-1 border-t border-neutral-200" />
            <span className="text-xs text-neutral-400 uppercase">or upload artifact manually</span>
            <div className="flex-1 border-t border-neutral-200" />
          </div>

          {/* Manual upload fallback */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Upload Build Artifact</h2>

            <div className="p-3 rounded-lg bg-neutral-50 border border-neutral-200">
              <div className="flex items-start gap-2">
                <Info className="w-4 h-4 text-neutral-500 mt-0.5 shrink-0" />
                <div className="text-sm text-neutral-600 space-y-1">
                  <p><span className="font-medium">Hardhat:</span> Upload a build-info file from <code className="text-xs bg-neutral-200 px-1 py-0.5 rounded font-mono">artifacts/build-info/*.json</code></p>
                  <p><span className="font-medium">Foundry:</span> Use <code className="text-xs bg-neutral-200 px-1 py-0.5 rounded font-mono">forge verify-contract --show-standard-json-input</code> and paste below</p>
                </div>
              </div>
            </div>

            {/* File Upload */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-neutral-700">Build Artifact (.json)</label>
                {hfFileName && (
                  <span className="text-xs text-neutral-500 font-mono">{hfFileName}</span>
                )}
              </div>
              <label className="flex items-center justify-center border-2 border-dashed border-neutral-300 hover:border-neutral-400 rounded-lg p-6 cursor-pointer transition-colors">
                <div className="text-center">
                  <Upload className="w-8 h-8 text-neutral-400 mx-auto mb-2" />
                  <p className="text-sm text-neutral-600">
                    {hfFileName ? 'Click to replace file' : 'Click to select a .json file'}
                  </p>
                </div>
                <input
                  type="file"
                  accept=".json"
                  onChange={handleHardhatFileUpload}
                  className="hidden"
                />
              </label>
            </div>

            {hfParseError && (
              <div className="p-3 rounded-lg bg-error-50 border border-error-200">
                <div className="flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 text-error-500 mt-0.5 shrink-0" />
                  <span className="text-sm text-error-700">{hfParseError}</span>
                </div>
              </div>
            )}

            {hfArtifact && (
              <div className="p-3 rounded-lg bg-success-50 border border-success-200">
                <div className="flex items-center gap-2">
                  <CheckCircle className="w-4 h-4 text-success-600" />
                  <span className="text-sm text-success-700">
                    Artifact parsed successfully
                    {hfArtifact.contracts.length > 0 && ` - ${hfArtifact.contracts.length} contract(s) found`}
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* Contract Settings */}
          <div className="card p-6 space-y-4">
            <h2 className="text-sm font-semibold text-neutral-900 uppercase tracking-wide">Contract Settings</h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">Compiler Version</label>
                <select
                  value={hfCompilerVersion}
                  onChange={(e) => setHfCompilerVersion(e.target.value)}
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
                {hfArtifact && hfArtifact.contracts.length > 0 ? (
                  <select
                    value={hfContractName}
                    onChange={(e) => {
                      setHfContractName(e.target.value);
                      // Auto-fill file path when selecting a contract
                      const match = hfArtifact.contracts.find(c => c.name === e.target.value);
                      if (match) {
                        setHfContractFile(match.file);
                      }
                    }}
                    className="select"
                  >
                    <option value="">Select contract</option>
                    {hfArtifact.contracts.map((c, i) => (
                      <option key={`${c.file}:${c.name}:${i}`} value={c.name}>
                        {c.name} ({c.file})
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type="text"
                    value={hfContractName}
                    onChange={(e) => setHfContractName(e.target.value)}
                    placeholder="MyContract"
                    className="input"
                  />
                )}
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">
                  Contract File Path
                  <span className="text-neutral-400 font-normal ml-1">(optional)</span>
                </label>
                {hfArtifact && hfArtifact.contracts.length > 0 ? (
                  <select
                    value={hfContractFile}
                    onChange={(e) => setHfContractFile(e.target.value)}
                    className="select"
                  >
                    <option value="">Select file</option>
                    {[...new Set(hfArtifact.contracts.map(c => c.file))].map((file) => (
                      <option key={file} value={file}>{file}</option>
                    ))}
                  </select>
                ) : (
                  <input
                    type="text"
                    value={hfContractFile}
                    onChange={(e) => setHfContractFile(e.target.value)}
                    placeholder="contracts/MyContract.sol"
                    className="input font-mono"
                  />
                )}
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">
                  Constructor Arguments
                  <span className="text-neutral-400 font-normal ml-1">(optional)</span>
                </label>
                <input
                  type="text"
                  value={hfConstructorArgs}
                  onChange={(e) => setHfConstructorArgs(e.target.value)}
                  placeholder="ABI-encoded hex (e.g., 0x00000000...)"
                  className="input font-mono"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-neutral-700">
                  Contract License
                  <span className="text-neutral-400 font-normal ml-1">(optional)</span>
                </label>
                <select
                  value={hfLicense}
                  onChange={(e) => setHfLicense(e.target.value)}
                  className="select"
                >
                  {LICENSE_TYPES.map((l) => (
                    <option key={l} value={l}>{l}</option>
                  ))}
                </select>
              </div>
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
          disabled={isSubmitDisabled()}
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
