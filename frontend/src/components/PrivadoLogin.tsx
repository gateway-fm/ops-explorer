import { useState, useEffect, useCallback, useRef } from 'react';
import { createPortal } from 'react-dom';
import { useAuth } from '../lib/auth';
import { X, Loader2, CheckCircle2, Shield } from 'lucide-react';

interface PrivadoLoginProps {
  onClose: () => void;
  returnUrl?: string;
}

export function PrivadoLogin({ onClose, returnUrl }: PrivadoLoginProps) {
  const { initiatePrivadoLogin, pollSSOStatus, ssoLoginSession, isLoading } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [completed, setCompleted] = useState(false);
  const [qrData, setQrData] = useState<string | null>(null);
  const [retryCount, setRetryCount] = useState(0);

  // Start the login flow - on mount or retry
  const hasStartedRef = useRef(false);

  useEffect(() => {
    // On first mount, start automatically. On retry, retryCount changes.
    if (hasStartedRef.current && retryCount === 0) {
      return;
    }
    hasStartedRef.current = true;

    let mounted = true;

    const startLogin = async () => {
      try {
        setError(null);
        setQrData(null);
        const session = await initiatePrivadoLogin(returnUrl);
        if (mounted && session.authRequest) {
          setQrData(JSON.stringify(session.authRequest));
        }
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to start authentication');
        }
      }
    };

    startLogin();

    return () => {
      mounted = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retryCount]);

  // Poll for completion
  useEffect(() => {
    if (!ssoLoginSession || completed) return;

    const pollInterval = setInterval(async () => {
      const done = await pollSSOStatus();
      if (done) {
        setCompleted(true);
        clearInterval(pollInterval);
      }
    }, 2000);

    return () => clearInterval(pollInterval);
  }, [ssoLoginSession, pollSSOStatus, completed]);

  // Generate QR code as data URL
  const generateQRCodeURL = useCallback((data: string) => {
    const encodedData = encodeURIComponent(data);
    return `https://api.qrserver.com/v1/create-qr-code/?size=256x256&data=${encodedData}`;
  }, []);

  return createPortal(
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-[60] p-4">
      <div className="bg-white rounded-xl shadow-elevated max-w-md w-full border border-neutral-200">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-neutral-200">
          <div className="flex items-center gap-2">
            <Shield className="w-5 h-5 text-primary" />
            <h2 className="text-lg font-semibold text-neutral-900">Sign in with Privado</h2>
          </div>
          <button
            onClick={onClose}
            className="p-1 hover:bg-neutral-100 rounded-lg transition-colors"
          >
            <X className="w-5 h-5 text-neutral-500" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          {error ? (
            <div className="text-center">
              <div className="text-error-600 mb-2">{error}</div>
              <p className="text-sm text-neutral-500 mb-4">The authentication service may be temporarily unavailable.</p>
              <div className="flex items-center justify-center gap-3">
                <button
                  onClick={() => setRetryCount((c) => c + 1)}
                  className="btn-primary"
                >
                  Try again
                </button>
                <button
                  onClick={onClose}
                  className="btn-secondary"
                >
                  Close
                </button>
              </div>
            </div>
          ) : completed ? (
            <div className="text-center">
              <CheckCircle2 className="w-16 h-16 mx-auto mb-4 text-success-500" />
              <p className="text-lg font-medium mb-2 text-neutral-900">Authentication Successful!</p>
              <p className="text-neutral-500">Redirecting...</p>
            </div>
          ) : isLoading || !qrData ? (
            <div className="text-center py-8">
              <Loader2 className="w-8 h-8 mx-auto mb-4 animate-spin text-primary" />
              <p className="text-neutral-500">Initializing authentication...</p>
            </div>
          ) : (
            <div className="text-center">
              <p className="mb-4 text-neutral-500">
                Scan this QR code with the Privado ID app to authenticate
              </p>

              <div className="bg-neutral-50 p-4 rounded-xl inline-block mb-4 border border-neutral-200">
                <img
                  src={generateQRCodeURL(qrData)}
                  alt="Authentication QR Code"
                  className="w-64 h-64"
                />
              </div>

              <div className="flex items-center justify-center gap-2 text-sm text-neutral-500">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Waiting for authentication...</span>
              </div>

              <p className="mt-4 text-xs text-neutral-400">
                Don't have the Privado ID app?{' '}
                <a
                  href="https://privado.id"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  Download it here
                </a>
              </p>
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body
  );
}
