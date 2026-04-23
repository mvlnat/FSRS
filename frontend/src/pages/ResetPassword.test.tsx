import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import * as api from '../api/client';
import { ResetPassword } from './ResetPassword';

vi.mock('../api/client', () => ({
  confirmPasswordReset: vi.fn(),
}));

const mockedApi = vi.mocked(api);

describe('ResetPassword', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('blocks passwords that exceed bcrypts 72-byte limit even when they are under 72 characters', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter initialEntries={['/reset-password?token=test-token']}>
        <ResetPassword />
      </MemoryRouter>,
    );

    const tooLongPassword = '界'.repeat(25);

    await user.type(screen.getByLabelText('New Password'), tooLongPassword);
    await user.type(screen.getByLabelText('Confirm New Password'), tooLongPassword);
    await user.click(screen.getByRole('button', { name: 'Reset Password' }));

    expect(mockedApi.confirmPasswordReset).not.toHaveBeenCalled();
    expect(screen.getByText('Password must be 72 bytes or fewer')).toBeInTheDocument();
  });

  it('submits the reset token and password', async () => {
    mockedApi.confirmPasswordReset.mockResolvedValue({
      message: 'Password has been reset. You can now sign in.',
    });
    const user = userEvent.setup();

    render(
      <MemoryRouter initialEntries={['/reset-password?token=test-token']}>
        <ResetPassword />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText('New Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm New Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Reset Password' }));

    expect(mockedApi.confirmPasswordReset).toHaveBeenCalledWith('test-token', 'password123');
    expect(await screen.findByText('Password has been reset. You can now sign in.')).toBeInTheDocument();
  });
});
