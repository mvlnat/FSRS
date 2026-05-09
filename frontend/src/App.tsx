import { BrowserRouter, Routes, Route, Navigate, Link } from 'react-router-dom';
import { AuthProvider } from './hooks/AuthProvider';
import { useAuth } from './hooks/useAuth';
import { ThemeProvider } from './hooks/ThemeProvider';
import { useTheme } from './hooks/useTheme';
import { DemoProvider } from './hooks/DemoProvider';
import { useDemo } from './hooks/useDemo';
import { initDemoModeFromStorage } from './demo/demoApi';

// Initialize demo mode from storage before app renders
initDemoModeFromStorage();
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { ForgotPassword } from './pages/ForgotPassword';
import { ResetPassword } from './pages/ResetPassword';
import { VerifyEmail } from './pages/VerifyEmail';
import { Decks } from './pages/Decks';
import { DeckEdit } from './pages/DeckEdit';
import { Study } from './pages/Study';
import { About } from './pages/About';
import './App.css';

function AuthRoute({
  children,
  publicOnly = false,
}: {
  children: React.ReactNode;
  publicOnly?: boolean;
}) {
  const { isAuthenticated, loading } = useAuth();
  const { isDemo } = useDemo();

  if (loading) {
    return (
      <div className="loading" role="status" aria-live="polite">
        Loading...
      </div>
    );
  }

  // In demo mode, treat user as authenticated
  const effectivelyAuthenticated = isAuthenticated || isDemo;

  if (publicOnly) {
    return !effectivelyAuthenticated ? <>{children}</> : <Navigate to="/" />;
  }

  return effectivelyAuthenticated ? <>{children}</> : <Navigate to="/login" />;
}

function ThemeToggle() {
  const { theme, effectiveTheme, setTheme } = useTheme();

  const cycleTheme = () => {
    if (theme === 'light') setTheme('dark');
    else if (theme === 'dark') setTheme('system');
    else setTheme('light');
  };

  const getLabel = () => {
    if (theme === 'system') return 'System';
    return theme === 'dark' ? 'Dark' : 'Light';
  };

  return (
    <button
      onClick={cycleTheme}
      className="theme-toggle"
      aria-label={`Theme: ${getLabel()}. Click to change.`}
      title={`Theme: ${getLabel()}`}
    >
      {effectiveTheme === 'dark' ? (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      ) : (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="5" />
          <line x1="12" y1="1" x2="12" y2="3" />
          <line x1="12" y1="21" x2="12" y2="23" />
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
          <line x1="1" y1="12" x2="3" y2="12" />
          <line x1="21" y1="12" x2="23" y2="12" />
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
        </svg>
      )}
      {theme === 'system' && (
        <span className="theme-toggle-badge">A</span>
      )}
    </button>
  );
}

function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const { isDemo, exitDemo } = useDemo();

  const handleLogout = () => {
    if (isDemo) {
      exitDemo();
      window.location.href = '/login';
    } else {
      logout();
    }
  };

  const showUserInfo = user || isDemo;

  return (
    <div className="app">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <header className="header">
        <div className="header-brand">
          <Link to="/" className="header-title">FSRS</Link>
          <nav className="header-nav" aria-label="Primary">
            <Link to="/about" className="header-link">About</Link>
          </nav>
        </div>
        {showUserInfo && (
          <div className="user-info">
            <span className="user-email">{isDemo ? 'Demo User' : user?.email}</span>
            <ThemeToggle />
            <button onClick={handleLogout}>{isDemo ? 'Exit Demo' : 'Logout'}</button>
          </div>
        )}
      </header>
      <main id="main-content" className="main" tabIndex={-1}>
        {children}
      </main>
    </div>
  );
}

function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <BrowserRouter>
          <DemoProvider>
            <Layout>
              <Routes>
            <Route path="/about" element={<About />} />
            <Route
              path="/login"
              element={
                <AuthRoute publicOnly>
                  <Login />
                </AuthRoute>
              }
            />
            <Route
              path="/register"
              element={
                <AuthRoute publicOnly>
                  <Register />
                </AuthRoute>
              }
            />
            <Route
              path="/forgot-password"
              element={
                <AuthRoute publicOnly>
                  <ForgotPassword />
                </AuthRoute>
              }
            />
            <Route
              path="/reset-password"
              element={
                <AuthRoute publicOnly>
                  <ResetPassword />
                </AuthRoute>
              }
            />
            <Route
              path="/verify-email"
              element={
                <AuthRoute publicOnly>
                  <VerifyEmail />
                </AuthRoute>
              }
            />
            <Route
              path="/"
              element={
                <AuthRoute>
                  <Decks />
                </AuthRoute>
              }
            />
            <Route
              path="/decks/:id"
              element={
                <AuthRoute>
                  <DeckEdit />
                </AuthRoute>
              }
            />
            <Route
              path="/study/:deckId"
              element={
                <AuthRoute>
                  <Study />
                </AuthRoute>
              }
            />
              </Routes>
            </Layout>
          </DemoProvider>
        </BrowserRouter>
      </AuthProvider>
    </ThemeProvider>
  );
}

export default App;
