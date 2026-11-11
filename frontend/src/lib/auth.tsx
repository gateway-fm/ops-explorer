import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api } from './api';
import { isV2 } from './v2Session';
import { V2AuthProvider } from './authV2';
import { AuthContext, type AuthState } from './authContext';
export { useAuth } from './authContext';
export type { AuthContextType } from './authContext';

export function AuthProvider({ children }: { children: ReactNode }) {
  return isV2() ? <V2AuthProvider context={AuthContext}>{children}</V2AuthProvider> : <LegacyAuthProvider>{children}</LegacyAuthProvider>;
}

function LegacyAuthProvider({ children }: { children: ReactNode }) {
  const [isLoading, setIsLoading] = useState(true);
  const [auth, setAuth] = useState<AuthState>({
    authenticated: false,
    did: null,
    expiresAt: null,
  });

  const queryClient = useQueryClient();

  const checkStatus = useCallback(async () => {
    try {
      const status = await api.auth.status();
      setAuth({
        authenticated: status.authenticated,
        did: status.did || null,
        expiresAt: status.expires_at || null,
      });
    } catch {
      setAuth({ authenticated: false, did: null, expiresAt: null });
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    checkStatus();
  }, [checkStatus]);

  const logout = useCallback(async () => {
    try {
      await api.auth.logout();
    } finally {
      setAuth({ authenticated: false, did: null, expiresAt: null });
      // Invalidate all queries so mounted components refetch with the new
      // (unauthenticated) auth state.  clear() only removes queries from
      // the cache without triggering refetches, so components keep showing
      // stale authenticated data until a manual page refresh.
      await queryClient.invalidateQueries();
    }
  }, [queryClient]);

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: auth.authenticated,
        isLoading,
        auth,
        logout,
        checkStatus,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

