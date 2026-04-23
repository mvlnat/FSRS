import { useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import * as api from '../api/client';
import {
  getPasswordByteLength,
  getPasswordCharacterCount,
  maxPasswordBytes,
  minPasswordCharacters,
} from '../utils/password';

export function ResetPassword() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token')?.trim() ?? '';
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');

    if (!token) {
      setError('Reset token is missing');
      return;
    }
    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }
    if (getPasswordCharacterCount(password) < minPasswordCharacters) {
      setError('Password must be at least 8 characters');
      return;
    }
    if (getPasswordByteLength(password) > maxPasswordBytes) {
      setError('Password must be 72 bytes or fewer');
      return;
    }

    setSubmitting(true);
    try {
      const response = await api.confirmPasswordReset(token, password);
      setSuccess(response.message);
      setPassword('');
      setConfirmPassword('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Password reset failed');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-container">
      <h1>Reset Password</h1>
      <form onSubmit={handleSubmit}>
        {success && (
          <div className="success" role="status" aria-live="polite">
            {success}
          </div>
        )}
        {error && <div className="error" role="alert">{error}</div>}
        <div className="form-group">
          <label htmlFor="password">New Password</label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={minPasswordCharacters}
          />
        </div>
        <div className="form-group">
          <label htmlFor="confirmPassword">Confirm New Password</label>
          <input
            id="confirmPassword"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
          />
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? 'Loading...' : 'Reset Password'}
        </button>
      </form>
      <p>
        Back to <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
