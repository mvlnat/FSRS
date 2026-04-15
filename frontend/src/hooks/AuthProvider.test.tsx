import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import type { User } from '../types';
import * as api from '../api/client';
import { AuthProvider } from './AuthProvider';
import { useAuth } from './useAuth';

vi.mock('../api/client', () => ({
  getMe: vi.fn(),
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
}));

const mockedApi = vi.mocked(api);

function AuthStateHarness() {
  const { user, loading, error, isAuthenticated } = useAuth();

  return (
    <div>
      <div data-testid="user-email">{user?.email ?? 'none'}</div>
      <div data-testid="loading">{String(loading)}</div>
      <div data-testid="error">{error ?? 'none'}</div>
      <div data-testid="authenticated">{String(isAuthenticated)}</div>
    </div>
  );
}

function AuthActionHarness() {
  const { user, error, login, register } = useAuth();

  const handleLogin = () => {
    void login('ada@example.com', 'secret123').catch(() => undefined);
  };

  const handleRegister = () => {
    void register('grace@example.com', 'secret123').catch(() => undefined);
  };

  return (
    <div>
      <div data-testid="user-email">{user?.email ?? 'none'}</div>
      <div data-testid="error">{error ?? 'none'}</div>
      <button onClick={handleLogin}>Log In</button>
      <button onClick={handleRegister}>Register</button>
    </div>
  );
}

function renderWithProvider(children: ReactNode) {
  return render(<AuthProvider>{children}</AuthProvider>);
}

describe('AuthProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('hydrates the authenticated user on mount', async () => {
    const user: User = { id: 'user-1', email: 'ada@example.com' };
    mockedApi.getMe.mockResolvedValue(user);

    renderWithProvider(<AuthStateHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent(user.email);
    });
    expect(screen.getByTestId('loading')).toHaveTextContent('false');
    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('falls back to an anonymous state when bootstrap auth fails', async () => {
    mockedApi.getMe.mockRejectedValue(new Error('missing session'));

    renderWithProvider(<AuthStateHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });
    expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('updates the session state after a successful login', async () => {
    mockedApi.getMe.mockRejectedValue(new Error('missing session'));
    mockedApi.login.mockResolvedValue({ id: 'user-2', email: 'ada@example.com' });
    const user = userEvent.setup();

    renderWithProvider(<AuthActionHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    });

    await user.click(screen.getByRole('button', { name: 'Log In' }));

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('ada@example.com');
    });
    expect(mockedApi.login).toHaveBeenCalledWith('ada@example.com', 'secret123');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('exposes registration failures through the auth error state', async () => {
    mockedApi.getMe.mockRejectedValue(new Error('missing session'));
    mockedApi.register.mockRejectedValue(new Error('Email already exists'));
    const user = userEvent.setup();

    renderWithProvider(<AuthActionHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    });

    await user.click(screen.getByRole('button', { name: 'Register' }));

    await waitFor(() => {
      expect(screen.getByTestId('error')).toHaveTextContent('Email already exists');
    });
    expect(mockedApi.register).toHaveBeenCalledWith('grace@example.com', 'secret123');
  });
});
