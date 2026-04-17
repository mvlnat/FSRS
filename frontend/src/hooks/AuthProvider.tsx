import { useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import * as api from '../api/client';
import type { User } from '../types';
import { AuthContext, type AuthState, type AuthContextType } from './auth-context';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    loading: true,
    error: null,
  });
  const authGenerationRef = useRef(0);
  const ignoreUnauthorizedBeforeRef = useRef(0);

  const checkAuth = useCallback(async () => {
    const generation = authGenerationRef.current;

    try {
      const user = await api.getMe();
      if (generation !== authGenerationRef.current) {
        return;
      }
      setState({ user, loading: false, error: null });
    } catch {
      if (generation !== authGenerationRef.current) {
        return;
      }
      setState({ user: null, loading: false, error: null });
    }
  }, []);

  useEffect(() => {
    void checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    return api.onUnauthorized((requestId) => {
      if (requestId <= ignoreUnauthorizedBeforeRef.current) {
        return;
      }

      authGenerationRef.current += 1;
      setState((current) => {
        if (!current.user && !current.loading) {
          return current;
        }

        return { user: null, loading: false, error: null };
      });
    });
  }, []);

  const setAuthenticated = (user: User) => {
    authGenerationRef.current += 1;
    ignoreUnauthorizedBeforeRef.current = api.getLatestRequestId();
    setState({ user, loading: false, error: null });
  };

  const clearSession = () => {
    authGenerationRef.current += 1;
    ignoreUnauthorizedBeforeRef.current = api.getLatestRequestId();
    setState({ user: null, loading: false, error: null });
  };

  const setAuthError = (err: unknown, fallbackMessage: string) => {
    const message = err instanceof Error ? err.message : fallbackMessage;
    setState((current) => ({ ...current, loading: false, error: message }));
  };

  const login = async (email: string, password: string) => {
    setState((current) => ({ ...current, loading: true, error: null }));
    try {
      const user = await api.login(email, password);
      setAuthenticated(user);
      return user;
    } catch (err) {
      setAuthError(err, 'Login failed');
      throw err;
    }
  };

  const register = async (email: string, password: string) => {
    setState((current) => ({ ...current, loading: true, error: null }));
    try {
      await api.register(email, password);
      setState((current) => ({ ...current, loading: false, error: null }));
    } catch (err) {
      setAuthError(err, 'Registration failed');
      throw err;
    }
  };

  const logout = async () => {
    try {
      await api.logout();
    } finally {
      clearSession();
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
