import { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import type { DeckWithStats } from '../types';
import * as api from '../api/client';

export function Decks() {
  const [decks, setDecks] = useState<DeckWithStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDescription, setNewDescription] = useState('');
  const [stats, setStats] = useState<api.StudyStats | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    loadDecks();
    loadStats();
  }, []);

  const loadDecks = async () => {
    try {
      const data = await api.getDecks();
      setDecks(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load decks');
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const data = await api.getStudyStats();
      setStats(data);
    } catch (err) {
      // Stats are not critical, silently ignore
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.createDeck(newName, newDescription);
      setNewName('');
      setNewDescription('');
      setShowCreate(false);
      loadDecks();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create deck');
    }
  };

  const handleExport = async (id: string, name: string) => {
    try {
      const data = await api.exportDeck(id);
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${name}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to export deck');
    }
  };

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    try {
      const text = await file.text();
      const data = JSON.parse(text) as api.DeckExport;

      if (!data.name || !Array.isArray(data.cards)) {
        throw new Error('Invalid deck format');
      }

      await api.importDeck(data);
      loadDecks();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import deck');
    } finally {
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div className="decks-container">
      <div className="decks-header">
        <h1>My Decks</h1>
        <div className="header-actions">
          <input
            ref={fileInputRef}
            type="file"
            accept=".json"
            onChange={handleImport}
            style={{ display: 'none' }}
          />
          <button onClick={() => fileInputRef.current?.click()} className="btn-import">
            Import
          </button>
          <button onClick={() => setShowCreate(!showCreate)}>
            {showCreate ? 'Cancel' : 'New Deck'}
          </button>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      {stats && stats.totalReviews > 0 && (
        <div className="study-stats-card">
          <h2>Study Statistics</h2>
          <div className="stats-grid">
            <div className="stat-item">
              <span className="stat-value">{stats.reviewsToday}</span>
              <span className="stat-label">Reviews Today</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{stats.reviewsThisWeek}</span>
              <span className="stat-label">This Week</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{stats.totalReviews}</span>
              <span className="stat-label">Total Reviews</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{stats.retentionRate.toFixed(0)}%</span>
              <span className="stat-label">Retention Rate</span>
            </div>
          </div>
        </div>
      )}

      {showCreate && (
        <form onSubmit={handleCreate} className="create-deck-form">
          <div className="form-group">
            <label htmlFor="name">Name</label>
            <input
              id="name"
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              required
            />
          </div>
          <div className="form-group">
            <label htmlFor="description">Description</label>
            <textarea
              id="description"
              value={newDescription}
              onChange={(e) => setNewDescription(e.target.value)}
            />
          </div>
          <button type="submit">Create Deck</button>
        </form>
      )}

      <div className="decks-list">
        {decks.length === 0 ? (
          <p>No decks yet. Create your first deck to get started!</p>
        ) : (
          decks.map((deck) => (
            <div key={deck.id} className="deck-card">
              <div className="deck-info">
                <h3>{deck.name}</h3>
                {deck.description && <p>{deck.description}</p>}
                <div className="deck-stats">
                  <span>Total: {deck.stats.total}</span>
                  <span>New: {deck.stats.new}</span>
                  <span>Due: {deck.stats.due}</span>
                  <span>Learning: {deck.stats.learning}</span>
                </div>
              </div>
              <div className="deck-actions">
                <Link to={`/study/${deck.id}`} className="btn-study">
                  Study
                </Link>
                <Link to={`/decks/${deck.id}`} className="btn-edit">
                  Edit
                </Link>
                <button onClick={() => handleExport(deck.id, deck.name)} className="btn-export">
                  Export
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
