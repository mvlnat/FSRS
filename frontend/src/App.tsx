import { BrowserRouter, Routes, Route, Navigate, Link } from 'react-router-dom';
import { AuthProvider } from './hooks/AuthProvider';
import { useAuth } from './hooks/useAuth';
import { Login } from './pages/Login';
import { Register } from './pages/Register';
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

  if (loading) {
    return <div className="loading">Loading...</div>;
  }

  if (publicOnly) {
    return !isAuthenticated ? <>{children}</> : <Navigate to="/" />;
  }

  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />;
}

function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();

  return (
    <div className="app">
      <header className="header">
        <div className="header-brand">
          <Link to="/" className="header-title">FSRS</Link>
          <nav className="header-nav" aria-label="Primary">
            <Link to="/about" className="header-link">About</Link>
          </nav>
        </div>
        {user && (
          <div className="user-info">
            <span>{user.email}</span>
            <button onClick={logout}>Logout</button>
          </div>
        )}
      </header>
      <main className="main">{children}</main>
    </div>
  );
}

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
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
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
