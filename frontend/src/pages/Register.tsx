import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';

const minPasswordCharacters = 8;
const maxPasswordBytes = 72;
const maxEmailLength = 255;

function getPasswordByteLength(value: string) {
  return new TextEncoder().encode(value).length;
}

function getPasswordCharacterCount(value: string) {
  return Array.from(value).length;
}

export function Register() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const { register, loading } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    const trimmedEmail = email.trim();

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }
    if (trimmedEmail.length > maxEmailLength) {
      setError('Email must be 255 characters or fewer');
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

    try {
      await register(trimmedEmail, password);
      navigate('/login', {
        state: {
          info: 'If the email is available, the account is ready to sign in.',
        },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    }
  };

  return (
    <div className="auth-container">
      <h1>Register</h1>
      <form onSubmit={handleSubmit}>
        {error && <div className="error">{error}</div>}
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
        <div className="form-group">
          <label htmlFor="password">Password</label>
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
          <label htmlFor="confirmPassword">Confirm Password</label>
          <input
            id="confirmPassword"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
          />
        </div>
        <button type="submit" disabled={loading}>
          {loading ? 'Loading...' : 'Register'}
        </button>
      </form>
      <p>
        Already have an account? <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
