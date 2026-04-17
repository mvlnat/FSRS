import { act, render, screen, waitFor } from '@testing-library/react';
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
  onUnauthorized: vi.fn(),
  getLatestRequestId: vi.fn(),
}));

const mockedApi = vi.mocked(api);
let unauthorizedHandler: ((requestId: number) => void) | null = null;

function apiError(status: number, message: string) {
  return Object.assign(new Error(message), { status });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

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
  const { user, error, login, register, isAuthenticated } = useAuth();

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
      <div data-testid="authenticated">{String(isAuthenticated)}</div>
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
    unauthorizedHandler = null;
    mockedApi.getLatestRequestId.mockReturnValue(0);
    mockedApi.onUnauthorized.mockImplementation((callback: (requestId: number) => void) => {
      unauthorizedHandler = callback;
      return () => {
        if (unauthorizedHandler === callback) {
          unauthorizedHandler = null;
        }
      };
    });
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

  it('clears the authenticated session after an unauthorized API signal', async () => {
    const user: User = { id: 'user-1', email: 'ada@example.com' };
    mockedApi.getMe.mockResolvedValue(user);

    renderWithProvider(<AuthStateHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    });

    if (!unauthorizedHandler) {
      throw new Error('expected unauthorized handler registration');
    }

    act(() => {
      unauthorizedHandler?.(1);
    });

    await waitFor(() => {
      expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    });
    expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('hydrates the session after a successful registration when the backend issues a session', async () => {
    mockedApi.getMe.mockRejectedValue(new Error('missing session'));
    mockedApi.register.mockResolvedValue(undefined);
    mockedApi.getMe
      .mockRejectedValueOnce(new Error('missing session'))
      .mockResolvedValueOnce({ id: 'user-3', email: 'grace@example.com' });
    const user = userEvent.setup();

    renderWithProvider(<AuthActionHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    });

    await user.click(screen.getByRole('button', { name: 'Register' }));

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('grace@example.com');
    });
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('keeps the user anonymous after an accepted registration without a session', async () => {
    mockedApi.register.mockResolvedValue(undefined);
    mockedApi.getMe
      .mockRejectedValueOnce(new Error('missing session'))
      .mockRejectedValueOnce(apiError(401, 'Unauthorized'));
    const user = userEvent.setup();

    renderWithProvider(<AuthActionHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    });

    await user.click(screen.getByRole('button', { name: 'Register' }));

    await waitFor(() => {
      expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    });
    expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('surfaces post-registration verification failures instead of treating them as an anonymous session', async () => {
    mockedApi.register.mockResolvedValue(undefined);
    mockedApi.getMe
      .mockRejectedValueOnce(new Error('missing session'))
      .mockRejectedValueOnce(apiError(503, 'temporary outage'));
    const user = userEvent.setup();

    renderWithProvider(<AuthActionHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    });

    await user.click(screen.getByRole('button', { name: 'Register' }));

    await waitFor(() => {
      expect(screen.getByTestId('error')).toHaveTextContent('temporary outage');
    });
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
  });

  it('exposes hard registration failures through the auth error state', async () => {
    mockedApi.getMe.mockRejectedValue(new Error('missing session'));
    mockedApi.register.mockRejectedValue(new Error('Password must be 72 bytes or fewer'));
    const user = userEvent.setup();

    renderWithProvider(<AuthActionHarness />);

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    });

    await user.click(screen.getByRole('button', { name: 'Register' }));

    await waitFor(() => {
      expect(screen.getByTestId('error')).toHaveTextContent('Password must be 72 bytes or fewer');
    });
    expect(mockedApi.register).toHaveBeenCalledWith('grace@example.com', 'secret123');
  });

  it('ignores stale bootstrap auth failures after a successful login', async () => {
    const bootstrapRequest = deferred<User>();
    const user = userEvent.setup();

    mockedApi.getMe.mockReturnValue(bootstrapRequest.promise);
    mockedApi.login.mockResolvedValue({ id: 'user-2', email: 'ada@example.com' });
    mockedApi.getLatestRequestId.mockReturnValue(2);

    renderWithProvider(<AuthActionHarness />);

    await user.click(screen.getByRole('button', { name: 'Log In' }));

    await waitFor(() => {
      expect(screen.getByTestId('user-email')).toHaveTextContent('ada@example.com');
    });

    act(() => {
      unauthorizedHandler?.(1);
    });
    bootstrapRequest.reject(new Error('expired session'));

    await act(async () => {
      await Promise.resolve();
    });

    expect(screen.getByTestId('user-email')).toHaveTextContent('ada@example.com');
    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });
});
