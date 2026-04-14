import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import type { Deck, CardWithState } from '../types';
import * as api from '../api/client';

type Tab = 'settings' | 'cards';

export function DeckEdit() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [deck, setDeck] = useState<Deck | null>(null);
  const [cards, setCards] = useState<CardWithState[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [activeTab, setActiveTab] = useState<Tab>('cards');

  // Deck settings state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [savingDeck, setSavingDeck] = useState(false);

  // Add card state
  const [showAddCard, setShowAddCard] = useState(false);
  const [newFront, setNewFront] = useState('');
  const [newBack, setNewBack] = useState('');
  const [newLink, setNewLink] = useState('');

  // Edit card state
  const [editingCard, setEditingCard] = useState<string | null>(null);
  const [editFront, setEditFront] = useState('');
  const [editBack, setEditBack] = useState('');
  const [editLink, setEditLink] = useState('');

  useEffect(() => {
    if (id) loadDeck();
  }, [id]);

  const loadDeck = async () => {
    if (!id) return;
    try {
      const [deckData, cardsData] = await Promise.all([
        api.getDeck(id),
        api.getCards(id),
      ]);
      setDeck(deckData);
      setCards(cardsData);
      setName(deckData.name);
      setDescription(deckData.description);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load deck');
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateDeck = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setSavingDeck(true);
    try {
      await api.updateDeck(id, name, description);
      loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update deck');
    } finally {
      setSavingDeck(false);
    }
  };

  const handleAddCard = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      await api.createCard(id, newFront, newBack, newLink);
      setNewFront('');
      setNewBack('');
      setNewLink('');
      setShowAddCard(false);
      loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add card');
    }
  };

  const handleEditCard = async (cardId: string) => {
    try {
      await api.updateCard(cardId, editFront, editBack, editLink);
      setEditingCard(null);
      loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update card');
    }
  };

  const handleDeleteCard = async (cardId: string) => {
    if (!confirm('Delete this card?')) return;
    try {
      await api.deleteCard(cardId);
      loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete card');
    }
  };

  const handleDeleteDeck = async () => {
    if (!id) return;
    if (!confirm('Are you sure you want to delete this deck? This action cannot be undone.')) return;
    try {
      await api.deleteDeck(id);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete deck');
    }
  };

  const startEditing = (card: CardWithState) => {
    setEditingCard(card.id);
    setEditFront(card.front);
    setEditBack(card.back);
    setEditLink(card.link || '');
  };

  const getFirstLine = (text: string): string => {
    const firstLine = text.split('\n')[0].trim();
    // Remove markdown formatting for preview
    return firstLine
      .replace(/```.*/, '') // Remove code block start
      .replace(/`([^`]+)`/g, '$1') // Remove inline code backticks
      .replace(/\*\*([^*]+)\*\*/g, '$1') // Remove bold
      .replace(/\*([^*]+)\*/g, '$1') // Remove italic
      .slice(0, 100) + (firstLine.length > 100 ? '...' : '');
  };

  if (loading) return <div className="deck-edit-container">Loading...</div>;
  if (!deck) return <div className="deck-edit-container">Deck not found</div>;

  return (
    <div className="deck-edit-container">
      <div className="deck-edit-header">
        <button onClick={() => navigate('/')} className="back-btn">
          Back to Decks
        </button>
        <h1>{deck.name}</h1>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="tabs">
        <button
          className={`tab ${activeTab === 'cards' ? 'active' : ''}`}
          onClick={() => setActiveTab('cards')}
        >
          Cards ({cards.length})
        </button>
        <button
          className={`tab ${activeTab === 'settings' ? 'active' : ''}`}
          onClick={() => setActiveTab('settings')}
        >
          Settings
        </button>
      </div>

      {activeTab === 'settings' && (
        <>
          <form onSubmit={handleUpdateDeck} className="deck-form">
            <div className="form-group">
              <label htmlFor="name">Deck Name</label>
              <input
                id="name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="form-group">
              <label htmlFor="description">Description</label>
              <textarea
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={4}
              />
            </div>
            <button type="submit" disabled={savingDeck}>
              {savingDeck ? 'Saving...' : 'Save Changes'}
            </button>
          </form>

          <div className="danger-zone">
            <h3>Danger Zone</h3>
            <p>Once you delete a deck, there is no going back. Please be certain.</p>
            <button onClick={handleDeleteDeck} className="btn-danger">
              Delete Deck
            </button>
          </div>
        </>
      )}

      {activeTab === 'cards' && (
        <div className="cards-section">
          <div className="cards-header">
            <button onClick={() => setShowAddCard(!showAddCard)}>
              {showAddCard ? 'Cancel' : 'Add Card'}
            </button>
          </div>

          {showAddCard && (
            <form onSubmit={handleAddCard} className="add-card-form">
              <p className="form-hint">
                Supports markdown: **bold**, *italic*, `inline code`, and code blocks with ```
              </p>
              <div className="card-form-grid">
                <div className="form-group">
                  <label htmlFor="front">Front</label>
                  <textarea
                    id="front"
                    value={newFront}
                    onChange={(e) => setNewFront(e.target.value)}
                    required
                    rows={6}
                    placeholder="Question or prompt..."
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="back">Back</label>
                  <textarea
                    id="back"
                    value={newBack}
                    onChange={(e) => setNewBack(e.target.value)}
                    required
                    rows={6}
                    placeholder="Answer..."
                  />
                </div>
              </div>
              <div className="form-group">
                <label htmlFor="link">Link (optional)</label>
                <input
                  id="link"
                  type="url"
                  value={newLink}
                  onChange={(e) => setNewLink(e.target.value)}
                  placeholder="https://..."
                />
              </div>
              <button type="submit">Add Card</button>
            </form>
          )}

          <div className="cards-list">
            {cards.length === 0 ? (
              <p className="no-cards">No cards yet. Add your first card!</p>
            ) : (
              cards.map((card) => (
                <div key={card.id} className="card-item">
                  {editingCard === card.id ? (
                    <div className="card-edit">
                      <p className="form-hint">
                        Supports markdown: **bold**, *italic*, `inline code`, and code blocks with ```
                      </p>
                      <div className="card-form-grid">
                        <div className="form-group">
                          <label>Front</label>
                          <textarea
                            value={editFront}
                            onChange={(e) => setEditFront(e.target.value)}
                            rows={6}
                          />
                        </div>
                        <div className="form-group">
                          <label>Back</label>
                          <textarea
                            value={editBack}
                            onChange={(e) => setEditBack(e.target.value)}
                            rows={6}
                          />
                        </div>
                      </div>
                      <div className="form-group">
                        <label>Link (optional)</label>
                        <input
                          type="url"
                          value={editLink}
                          onChange={(e) => setEditLink(e.target.value)}
                          placeholder="https://..."
                        />
                      </div>
                      <div className="card-edit-actions">
                        <button onClick={() => handleEditCard(card.id)}>Save</button>
                        <button onClick={() => setEditingCard(null)} className="btn-secondary">Cancel</button>
                      </div>
                    </div>
                  ) : (
                    <div className="card-row">
                      <div className="card-preview" onClick={() => startEditing(card)}>
                        <span className="card-preview-text">{getFirstLine(card.front)}</span>
                        {card.state && (
                          <span className="card-preview-meta">
                            {card.state.reps > 0 ? `${card.state.reps} reps` : 'New'}
                          </span>
                        )}
                      </div>
                      <div className="card-row-actions">
                        {card.link && (
                          <a href={card.link} target="_blank" rel="noopener noreferrer" className="btn-icon" title="Open link">
                            ↗
                          </a>
                        )}
                        <button onClick={() => startEditing(card)} className="btn-icon" title="Edit">
                          ✎
                        </button>
                        <button onClick={() => handleDeleteCard(card.id)} className="btn-icon btn-icon-danger" title="Delete">
                          ✕
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
