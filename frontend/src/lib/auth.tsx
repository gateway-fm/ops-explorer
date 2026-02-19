import { createContext, useContext, useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import { api } from './api';

// SSO Auth state
interface SSOAuthState {
  authenticated: boolean;
  did: string | null;
  expiresAt: number | null;
}

// SSO Login session for QR code flow
interface SSOLoginSession {
  oauthSessionId: string;
  authSessionId: string;
  authRequest: unknown;
  state: string;
}

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  // SSO (Privado) authentication
  ssoAuth: SSOAuthState;
  ssoLoginSession: SSOLoginSession | null;
  initiatePrivadoLogin: (returnUrl?: string) => Promise<SSOLoginSession>;
  pollSSOStatus: () => Promise<boolean>;
  cancelPrivadoLogin: () => void;
  privadoLogout: () => Promise<void>;
  checkSSOStatus: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isLoading, setIsLoading] = useState(true);

  // SSO (Privado) auth state
  const [ssoAuth, setSSOAuth] = useState<SSOAuthState>({
    authenticated: false,
    did: null,
    expiresAt: null,
  });
  const [ssoLoginSession, setSSOLoginSession] = useState<SSOLoginSession | null>(null);

  // Ref to prevent duplicate login calls
  const loginInProgressRef = useRef(false);

  // Check SSO status from backend (cookie-based)
  const checkSSOStatus = useCallback(async () => {
    try {
      const status = await api.auth.status();
      setSSOAuth({
        authenticated: status.authenticated,
        did: status.did || null,
        expiresAt: status.expires_at || null,
      });
    } catch {
      setSSOAuth({ authenticated: false, did: null, expiresAt: null });
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Check SSO status on mount
  useEffect(() => {
    checkSSOStatus();
  }, [checkSSOStatus]);

  // Initiate Privado SSO login
  const initiatePrivadoLogin = useCallback(async (returnUrl?: string): Promise<SSOLoginSession> => {
    // Prevent duplicate concurrent calls but don't throw - just wait
    if (loginInProgressRef.current) {
      return new Promise(() => {}); // Never resolves, caller will be cleaned up on unmount
    }
    loginInProgressRef.current = true;

    setIsLoading(true);
    try {
      const response = await api.auth.login(returnUrl);
      const session: SSOLoginSession = {
        oauthSessionId: response.oauth_session_id,
        authSessionId: response.auth_session_id,
        authRequest: response.auth_request,
        state: response.state,
      };
      setSSOLoginSession(session);
      return session;
    } finally {
      setIsLoading(false);
      loginInProgressRef.current = false;
    }
  }, []);

  // Poll SSO session status
  const pollSSOStatus = useCallback(async (): Promise<boolean> => {
    if (!ssoLoginSession) return false;
    try {
      const status = await api.auth.sessionStatus(ssoLoginSession.oauthSessionId);
      if (status.completed && status.redirect_url) {
        // Auth completed - redirect to callback to set cookie
        window.location.href = status.redirect_url;
        return true;
      }
      return false;
    } catch {
      return false;
    }
  }, [ssoLoginSession]);

  // Cancel Privado login
  const cancelPrivadoLogin = useCallback(() => {
    setSSOLoginSession(null);
  }, []);

  // Logout from Privado SSO
  const privadoLogout = useCallback(async () => {
    try {
      await api.auth.logout();
    } finally {
      setSSOAuth({ authenticated: false, did: null, expiresAt: null });
      setSSOLoginSession(null);
    }
  }, []);

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: ssoAuth.authenticated,
        isLoading,
        // SSO auth
        ssoAuth,
        ssoLoginSession,
        initiatePrivadoLogin,
        pollSSOStatus,
        cancelPrivadoLogin,
        privadoLogout,
        checkSSOStatus,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
