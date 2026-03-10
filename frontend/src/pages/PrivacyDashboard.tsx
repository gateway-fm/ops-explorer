import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Eye,
  Users,
  Clock,
  AlertTriangle,
  Copy,
  Check,
  ExternalLink,
  Fingerprint,
} from 'lucide-react';
import { useAuth } from '../lib/auth';
import { useViewableAddresses } from '../hooks/useAddressVisibility';
import { PageHeader } from '../components/PageHeader';
import { redirectToLogin } from '../lib/login';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';

type TabType = 'own' | 'disclosed';

interface StatCardProps {
  title: string;
  value: number | string;
  icon: React.ElementType;
  color: 'primary' | 'success' | 'warning' | 'neutral';
}

function StatCard({ title, value, icon: Icon, color }: StatCardProps) {
  const colorClasses = {
    primary: 'bg-primary-50 text-primary-600 border-primary-200',
    success: 'bg-success-50 text-success-600 border-success-200',
    warning: 'bg-warning-50 text-warning-600 border-warning-200',
    neutral: 'bg-neutral-100 text-neutral-600 border-neutral-200',
  };

  return (
    <div className="card p-4 sm:p-6">
      <div className="flex items-center gap-3">
        <div className={`w-10 h-10 rounded-xl flex items-center justify-center border ${colorClasses[color]}`}>
          <Icon className="w-5 h-5" />
        </div>
        <div>
          <p className="text-sm text-neutral-500">{title}</p>
          <p className="text-xl sm:text-2xl font-bold text-neutral-900">{value}</p>
        </div>
      </div>
    </div>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          onClick={handleCopy}
          className="p-1 rounded hover:bg-neutral-100 transition-colors"
        >
          {copied ? (
            <Check className="w-3.5 h-3.5 text-success-600" />
          ) : (
            <Copy className="w-3.5 h-3.5 text-neutral-400" />
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent>{copied ? 'Copied!' : 'Copy address'}</TooltipContent>
    </Tooltip>
  );
}

function truncateAddress(address: string): string {
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

function truncateDID(did: string): string {
  if (did.length <= 24) return did;
  return `${did.slice(0, 12)}...${did.slice(-8)}`;
}

function formatExpiration(expiresAt?: string): string {
  if (!expiresAt) return 'Never';

  const expDate = new Date(expiresAt);
  const now = new Date();
  const diffMs = expDate.getTime() - now.getTime();

  if (diffMs < 0) return 'Expired';

  const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return '1 day';
  if (diffDays < 7) return `${diffDays} days`;
  if (diffDays < 30) return `${Math.ceil(diffDays / 7)} weeks`;
  if (diffDays < 365) return `${Math.ceil(diffDays / 30)} months`;
  return `${Math.ceil(diffDays / 365)} years`;
}

function isExpiringSoon(expiresAt?: string): boolean {
  if (!expiresAt) return false;

  const expDate = new Date(expiresAt);
  const now = new Date();
  const diffMs = expDate.getTime() - now.getTime();
  const diffDays = diffMs / (1000 * 60 * 60 * 24);

  return diffDays > 0 && diffDays <= 7;
}

export function PrivacyDashboard() {
  const { auth, isLoading: authLoading } = useAuth();
  const { data, loading: dataLoading, error } = useViewableAddresses();
  const [activeTab, setActiveTab] = useState<TabType>('own');

  // Calculate stats
  const ownCount = data?.ownAddresses?.length ?? 0;
  const disclosedCount = data?.disclosedAddresses?.length ?? 0;
  const totalGrants = disclosedCount;
  const expiringSoon = data?.disclosedAddresses?.filter(d => isExpiringSoon(d.expires_at)).length ?? 0;

  // Show authentication prompt if not authenticated via Privado
  if (!auth.authenticated && !authLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-primary-50 flex items-center justify-center border border-primary-200">
          <Fingerprint className="w-8 h-8 text-primary" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Privacy Dashboard</h2>
        <p className="text-neutral-500 text-center max-w-md">
          Sign in with Privado ID to view your addresses and privacy disclosures.
        </p>
        <div className="mt-4">
          <button
            onClick={() => redirectToLogin('/privacy')}
            className="btn-primary flex items-center gap-2"
          >
            <Fingerprint className="w-4 h-4" />
            Sign in with Privado
          </button>
        </div>
      </div>
    );
  }

  if (authLoading || dataLoading) {
    return <div className="text-neutral-400">Loading...</div>;
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-error-50 flex items-center justify-center border border-error-200">
          <AlertTriangle className="w-8 h-8 text-error-600" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Error Loading Data</h2>
        <p className="text-neutral-500 text-center max-w-md">
          {error instanceof Error ? error.message : 'Failed to load privacy data. Please try again.'}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Privacy Dashboard"
        subtitle="Manage your address visibility and disclosures"
      />

      {/* Stats Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <StatCard
          title="Your Addresses"
          value={ownCount}
          icon={Eye}
          color="primary"
        />
        <StatCard
          title="Disclosed To You"
          value={disclosedCount}
          icon={Users}
          color="success"
        />
        <StatCard
          title="Active Grants"
          value={totalGrants}
          icon={Clock}
          color="neutral"
        />
        <StatCard
          title="Expiring Soon"
          value={expiringSoon}
          icon={AlertTriangle}
          color={expiringSoon > 0 ? 'warning' : 'neutral'}
        />
      </div>

      {/* Tabs */}
      <div className="card">
        <div className="flex border-b border-neutral-200">
          <button
            onClick={() => setActiveTab('own')}
            className={`px-4 py-3 text-sm font-medium transition-colors ${
              activeTab === 'own'
                ? 'text-neutral-900 border-b-2 border-primary'
                : 'text-neutral-500 hover:text-neutral-700'
            }`}
          >
            Your Addresses ({ownCount})
          </button>
          <button
            onClick={() => setActiveTab('disclosed')}
            className={`px-4 py-3 text-sm font-medium transition-colors ${
              activeTab === 'disclosed'
                ? 'text-neutral-900 border-b-2 border-primary'
                : 'text-neutral-500 hover:text-neutral-700'
            }`}
          >
            Disclosed to You ({disclosedCount})
          </button>
        </div>

        {/* Own Addresses Tab */}
        {activeTab === 'own' && (
          <>
            {ownCount === 0 ? (
              <div className="empty-state">
                <Eye className="w-12 h-12 mx-auto mb-4 text-neutral-300" />
                <p className="text-neutral-500">No addresses linked to your identity</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Address</th>
                      <th className="text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data?.ownAddresses?.map((address) => (
                      <tr key={address}>
                        <td>
                          <div className="flex items-center gap-2">
                            <Link
                              to={`/address/${address}`}
                              className="font-mono text-primary hover:text-primary-600 transition-colors"
                            >
                              <span className="hidden sm:inline">{address}</span>
                              <span className="sm:hidden">{truncateAddress(address)}</span>
                            </Link>
                            <CopyButton text={address} />
                          </div>
                        </td>
                        <td className="text-right">
                          <div className="flex items-center justify-end gap-2">
                            <Link
                              to={`/address/${address}`}
                              className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-primary hover:bg-primary-50 rounded-lg transition-colors"
                            >
                              <ExternalLink className="w-3.5 h-3.5" />
                              View
                            </Link>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}

        {/* Disclosed Addresses Tab */}
        {activeTab === 'disclosed' && (
          <>
            {disclosedCount === 0 ? (
              <div className="empty-state">
                <Users className="w-12 h-12 mx-auto mb-4 text-neutral-300" />
                <p className="text-neutral-500">No addresses have been disclosed to you</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Address</th>
                      <th className="hidden md:table-cell">Level</th>
                      <th className="hidden lg:table-cell">From (DID)</th>
                      <th className="hidden sm:table-cell">Expires</th>
                      <th className="text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data?.disclosedAddresses?.map((disclosed) => {
                      const isPseudonymous = disclosed.disclosure_level === 'pseudonymous';
                      const isRedacted = disclosed.disclosure_level === 'redacted';

                      const displayAddress = disclosed.address;
                      const displayAddressTruncated = isRedacted || isPseudonymous
                        ? disclosed.address
                        : truncateAddress(disclosed.address);

                      const isFull = disclosed.disclosure_level === 'full';
                      const viewLink = isFull
                        ? `/address/${disclosed.address}`
                        : `/grant/${disclosed.grantId}/${disclosed.addressId}`;

                      return (
                        <tr key={disclosed.grantId + disclosed.addressId}>
                          <td>
                            <div className="flex items-center gap-2">
                              {isRedacted ? (
                                <Link
                                  to={viewLink}
                                  className="font-mono text-neutral-400 italic hover:text-neutral-600 transition-colors"
                                >
                                  {displayAddress}
                                </Link>
                              ) : (
                                <Link
                                  to={viewLink}
                                  className="font-mono text-primary hover:text-primary-600 transition-colors"
                                >
                                  <span className="hidden sm:inline">{displayAddress}</span>
                                  <span className="sm:hidden">{displayAddressTruncated}</span>
                                </Link>
                              )}
                              {isFull && (
                                <CopyButton text={disclosed.address} />
                              )}
                            </div>
                          </td>
                          <td className="hidden md:table-cell">
                            <span className={`badge capitalize ${
                              isPseudonymous
                                ? 'bg-amber-100 text-amber-700 border-amber-200'
                                : isRedacted
                                  ? 'bg-red-100 text-red-700 border-red-200'
                                  : 'badge-primary'
                            }`}>
                              {disclosed.disclosure_level || 'Full'}
                            </span>
                          </td>
                          <td className="hidden lg:table-cell">
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="font-mono text-xs text-neutral-500 cursor-help">
                                  {truncateDID(disclosed.ownerDid)}
                                </span>
                              </TooltipTrigger>
                              <TooltipContent>
                                <span className="font-mono text-xs">{disclosed.ownerDid}</span>
                              </TooltipContent>
                            </Tooltip>
                          </td>
                          <td className="hidden sm:table-cell">
                            <span className={`text-sm ${
                              isExpiringSoon(disclosed.expires_at)
                                ? 'text-warning-600 font-medium'
                                : 'text-neutral-500'
                            }`}>
                              {formatExpiration(disclosed.expires_at)}
                            </span>
                          </td>
                          <td className="text-right">
                            <Link
                              to={viewLink}
                              className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-primary hover:bg-primary-50 rounded-lg transition-colors"
                            >
                              <ExternalLink className="w-3.5 h-3.5" />
                              View
                            </Link>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
