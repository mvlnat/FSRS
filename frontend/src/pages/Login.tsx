import { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { useDemo } from '../hooks/useDemo';

export function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { login, loading } = useAuth();
  const { enterDemo } = useDemo();
  const navigate = useNavigate();
  const location = useLocation();
  const info = typeof location.state === 'object' && location.state !== null && 'info' in location.state
    ? String(location.state.info)
    : '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    try {
      await login(email, password);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    }
  };

  const handleTryDemo = () => {
    enterDemo();
    navigate('/');
  };

  return (
    <div className="auth-container">
      <h1>Login</h1>
      <form onSubmit={handleSubmit}>
        {info && (
          <div className="success" role="status" aria-live="polite">
            {info}
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
          />
        </div>
        <button type="submit" disabled={loading}>
          {loading ? 'Loading...' : 'Login'}
        </button>
      </form>
      <p>
        <Link to="/forgot-password">Forgot your password?</Link>
      </p>
      <p>
        Don&apos;t have an account? <Link to="/register">Register</Link>
      </p>
      <div className="demo-divider">
        <span>or</span>
      </div>
      <button type="button" onClick={handleTryDemo} className="btn-demo">
        Try Demo
      </button>
      <p className="demo-hint">
        Explore the app with sample flashcards. No account needed.
      </p>
    </div>
  );
}
