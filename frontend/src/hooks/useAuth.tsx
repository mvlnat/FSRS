import { useState, useEffect, useCallback, createContext, useContext, type ReactNode } from 'react';
import type { User } from '../types';
import * as api from '../api/client';

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string | null;
}

interface AuthContextType extends AuthState {
  login: (email: string, password: string) => Promise<User>;
  register: (email: string, password: string) => Promise<User>;
  logout: () => Promise<void>;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    loading: true,
    error: null,
  });

  const checkAuth = useCallback(async () => {
    try {
      const user = await api.getMe();
      setState({ user, loading: false, error: null });
    } catch {
      setState({ user: null, loading: false, error: null });
    }
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const login = async (email: string, password: string) => {
    setState(s => ({ ...s, loading: true, error: null }));
    try {
      const user = await api.login(email, password);
      setState({ user, loading: false, error: null });
      return user;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Login failed';
      setState(s => ({ ...s, loading: false, error: message }));
      throw err;
    }
  };

  const register = async (email: string, password: string) => {
    setState(s => ({ ...s, loading: true, error: null }));
    try {
      const user = await api.register(email, password);
      setState({ user, loading: false, error: null });
      return user;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Registration failed';
      setState(s => ({ ...s, loading: false, error: message }));
      throw err;
    }
  };

  const logout = async () => {
    try {
      await api.logout();
    } finally {
      setState({ user: null, loading: false, error: null });
      window.location.href = '/';
    }
  };

  const value: AuthContextType = {
    user: state.user,
    loading: state.loading,
    error: state.error,
    login,
    register,
    logout,
    isAuthenticated: !!state.user,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
