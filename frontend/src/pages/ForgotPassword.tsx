import { useState } from 'react';
import { Link } from 'react-router-dom';
import * as api from '../api/client';

const maxEmailLength = 255;

export function ForgotPassword() {
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setSubmitting(true);

    try {
      const response = await api.requestPasswordReset(email.trim());
      setSuccess(response.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Password reset request failed');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-container">
      <h1>Forgot Password</h1>
      <form onSubmit={handleSubmit}>
        {success && (
          <div className="success" role="status" aria-live="polite">
            {success}
          </div>
        )}
        {error && <div className="error" role="alert">{error}</div>}
        <div className="form-group">
          <label htmlFor="email">Email</label>
          <input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            maxLength={maxEmailLength}
          />
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? 'Loading...' : 'Send Reset Link'}
        </button>
      </form>
      <p>
        Remembered your password? <Link to="/login">Back to Login</Link>
      </p>
    </div>
  );
}
