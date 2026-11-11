import { createContext, useContext } from 'react';

export interface AuthState { authenticated: boolean; did: string | null; expiresAt: number | null }
export interface AuthContextType { isAuthenticated: boolean; isLoading: boolean; auth: AuthState; logout: () => Promise<void>; checkStatus: () => Promise<void> }
export const AuthContext = createContext<AuthContextType | undefined>(undefined);
export function useAuth() { const context = useContext(AuthContext); if (context === undefined) throw Error('useAuth must be used within an AuthProvider'); return context; }
