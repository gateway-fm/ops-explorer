import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api } from './api';

interface AuthState {
  authenticated: boolean;
  did: string | null;
  expiresAt: number | null;
}

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  auth: AuthState;
  logout: () => Promise<void>;
  checkStatus: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
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
      queryClient.clear();
      setAuth({ authenticated: false, did: null, expiresAt: null });
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

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
