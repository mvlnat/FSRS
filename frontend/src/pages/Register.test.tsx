import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import type { AuthContextType } from '../hooks/auth-context';
import { useAuth } from '../hooks/useAuth';
import { Register } from './Register';

vi.mock('../hooks/useAuth', () => ({
  useAuth: vi.fn(),
}));

const mockedUseAuth = vi.mocked(useAuth);

describe('Register', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('blocks passwords that exceed bcrypts 72-byte limit even when they are under 72 characters', async () => {
    const register = vi.fn();
    const user = userEvent.setup();

    mockedUseAuth.mockReturnValue({
      user: null,
      loading: false,
      error: null,
      login: vi.fn(),
      register,
      logout: vi.fn(),
      isAuthenticated: false,
    } satisfies AuthContextType);

    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>,
    );

    const tooLongPassword = '界'.repeat(25);

    await user.type(screen.getByLabelText('Email'), 'grace@example.com');
    await user.type(screen.getByLabelText('Password'), tooLongPassword);
    await user.type(screen.getByLabelText('Confirm Password'), tooLongPassword);
    await user.click(screen.getByRole('button', { name: 'Register' }));

    expect(register).not.toHaveBeenCalled();
    expect(screen.getByText('Password must be 72 bytes or fewer')).toBeInTheDocument();
  });

  it('blocks passwords that are fewer than 8 characters even when UTF-16 length reaches 8', async () => {
    const register = vi.fn();
    const user = userEvent.setup();

    mockedUseAuth.mockReturnValue({
      user: null,
      loading: false,
      error: null,
      login: vi.fn(),
      register,
      logout: vi.fn(),
      isAuthenticated: false,
    } satisfies AuthContextType);

    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>,
    );

    const tooShortPassword = '🙂🙂🙂🙂';

    await user.type(screen.getByLabelText('Email'), 'grace@example.com');
    await user.type(screen.getByLabelText('Password'), tooShortPassword);
    await user.type(screen.getByLabelText('Confirm Password'), tooShortPassword);
    await user.click(screen.getByRole('button', { name: 'Register' }));

    expect(register).not.toHaveBeenCalled();
    expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
  });
});
