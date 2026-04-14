import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, AuthProvider } from './hooks/useAuth';
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { Decks } from './pages/Decks';
import { DeckEdit } from './pages/DeckEdit';
import { Study } from './pages/Study';
import './App.css';

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth();

  if (loading) {
    return <div className="loading">Loading...</div>;
  }

  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />;
}

function PublicRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth();

  if (loading) {
    return <div className="loading">Loading...</div>;
  }

  return !isAuthenticated ? <>{children}</> : <Navigate to="/" />;
}

function Layout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();

  return (
    <div className="app">
      <header className="header">
        <h1>FSRS</h1>
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
            <Route
              path="/login"
              element={
                <PublicRoute>
                  <Login />
                </PublicRoute>
              }
            />
            <Route
              path="/register"
              element={
                <PublicRoute>
                  <Register />
                </PublicRoute>
              }
            />
            <Route
              path="/"
              element={
                <PrivateRoute>
                  <Decks />
                </PrivateRoute>
              }
            />
            <Route
              path="/decks/:id"
              element={
                <PrivateRoute>
                  <DeckEdit />
                </PrivateRoute>
              }
            />
            <Route
              path="/study/:deckId"
              element={
                <PrivateRoute>
                  <Study />
                </PrivateRoute>
              }
            />
          </Routes>
        </Layout>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
