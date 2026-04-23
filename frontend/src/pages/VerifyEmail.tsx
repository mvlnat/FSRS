import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import * as api from '../api/client';

export function VerifyEmail() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token')?.trim() ?? '';
  const [message, setMessage] = useState(token ? 'Verifying your email...' : '');
  const [error, setError] = useState(token ? '' : 'Verification token is missing');

  useEffect(() => {
    if (!token) {
      return;
    }

    let cancelled = false;

    async function verify() {
      try {
        const response = await api.confirmEmailVerification(token);
        if (!cancelled) {
          setMessage(response.message);
          setError('');
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Email verification failed');
          setMessage('');
        }
      }
    }

    void verify();

    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <div className="auth-container">
      <h1>Verify Email</h1>
      {message && <div className="success">{message}</div>}
      {error && <div className="error">{error}</div>}
      <p>
        Continue to <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
