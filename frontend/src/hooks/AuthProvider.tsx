import { useState, useEffect, useCallback, type ReactNode } from 'react';
import * as api from '../api/client';
import { AuthContext, type AuthState, type AuthContextType } from './auth-context';

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
    void checkAuth();
  }, [checkAuth]);

  const login = async (email: string, password: string) => {
    setState((current) => ({ ...current, loading: true, error: null }));
    try {
      const user = await api.login(email, password);
      setState({ user, loading: false, error: null });
      return user;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Login failed';
      setState((current) => ({ ...current, loading: false, error: message }));
      throw err;
    }
  };

  const register = async (email: string, password: string) => {
    setState((current) => ({ ...current, loading: true, error: null }));
    try {
      const user = await api.register(email, password);
      setState({ user, loading: false, error: null });
      return user;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Registration failed';
      setState((current) => ({ ...current, loading: false, error: message }));
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
