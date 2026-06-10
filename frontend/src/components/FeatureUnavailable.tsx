import { Info } from 'lucide-react';

// FeatureUnavailable is a small generic card shown when a feature is gated off
// in the current deployment (see lib/features.ts). It generalizes the
// VerificationDisabled card so any feature-gated page can early-return it.
//
// The copy is intentionally minimal — it states only that the feature isn't
// available in this deployment, and does NOT disclose the privacy-proxy
// topology, RBAC model, or why it's disabled.
export function FeatureUnavailable({ feature }: { feature: string }) {
  return (
    <div className="max-w-2xl mx-auto py-16 px-6 text-center">
      <div className="mx-auto w-12 h-12 rounded-full bg-neutral-100 flex items-center justify-center mb-4">
        <Info className="w-6 h-6 text-neutral-400" />
      </div>
      <h1 className="text-xl font-semibold text-neutral-900 mb-2">
        {feature} is not available in this deployment
      </h1>
      <p className="text-sm text-neutral-500">
        Contact your operator if you need this feature enabled.
      </p>
    </div>
  );
}

export default FeatureUnavailable;
