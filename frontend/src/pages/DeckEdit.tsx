import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import type { Deck, CardWithState, Tag } from '../types';
import * as api from '../api/client';
import { getCardTitle, normalizeCardTitle } from '../utils/cards';
import { normalizeOptionalExternalLink } from '../utils/links';

type Tab = 'settings' | 'cards';
type SortOption = 'newest' | 'oldest' | 'alpha' | 'mostReviews' | 'leastReviews';

const CARDS_TAB_ID = 'deck-edit-tab-cards';
const SETTINGS_TAB_ID = 'deck-edit-tab-settings';
const CARDS_PANEL_ID = 'deck-edit-panel-cards';
const SETTINGS_PANEL_ID = 'deck-edit-panel-settings';

export function DeckEdit() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [deck, setDeck] = useState<Deck | null>(null);
  const [cards, setCards] = useState<CardWithState[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [activeTab, setActiveTab] = useState<Tab>('cards');
  const cardsTabRef = useRef<HTMLButtonElement | null>(null);
  const settingsTabRef = useRef<HTMLButtonElement | null>(null);

  // Search, sort, and filter state
  const [searchQuery, setSearchQuery] = useState('');
  const [sortBy, setSortBy] = useState<SortOption>('newest');
  const [filterTagId, setFilterTagId] = useState<string>('');

  // Deck settings state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [fuzzEnabled, setFuzzEnabled] = useState(false);
  const [newCardFrontTemplate, setNewCardFrontTemplate] = useState('');
  const [newCardBackTemplate, setNewCardBackTemplate] = useState('');
  const [savingDeck, setSavingDeck] = useState(false);

  // Tag management state
  const [newTagName, setNewTagName] = useState('');

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
  const [editTagIds, setEditTagIds] = useState<string[]>([]);

  const newCardDuplicates = useMemo(() => {
    const normalizedTitle = normalizeCardTitle(newFront);
    if (!normalizedTitle) {
      return [];
    }

    return cards.filter((card) => normalizeCardTitle(card.front) === normalizedTitle);
  }, [cards, newFront]);

  const editCardDuplicates = useMemo(() => {
    const normalizedTitle = normalizeCardTitle(editFront);
    if (!normalizedTitle || !editingCard) {
      return [];
    }

    return cards.filter(
      (card) => card.id !== editingCard && normalizeCardTitle(card.front) === normalizedTitle,
    );
  }, [cards, editingCard, editFront]);

  // Filtered and sorted cards
  const filteredCards = useMemo(() => {
    let result = [...cards];

    // Filter by search query
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter(card =>
        card.front.toLowerCase().includes(query) ||
        card.back.toLowerCase().includes(query)
      );
    }

    // Filter by tag
    if (filterTagId) {
      result = result.filter(card =>
        card.tags?.some(tag => tag.id === filterTagId)
      );
    }

    // Sort
    result.sort((a, b) => {
      switch (sortBy) {
        case 'oldest':
          return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
        case 'newest':
          return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
        case 'alpha': {
          const titleCompare = normalizeCardTitle(a.front).localeCompare(normalizeCardTitle(b.front));
          if (titleCompare !== 0) {
            return titleCompare;
          }
          return a.front.localeCompare(b.front);
        }
        case 'mostReviews':
          return (b.state?.reps || 0) - (a.state?.reps || 0);
        case 'leastReviews':
          return (a.state?.reps || 0) - (b.state?.reps || 0);
        default:
          return 0;
      }
    });

    return result;
  }, [cards, searchQuery, sortBy, filterTagId]);

  const loadDeck = useCallback(async () => {
    if (!id) return;
    try {
      const [deckData, cardsData, tagsData] = await Promise.all([
        api.getDeck(id),
        api.getCards(id),
        api.getTags(id),
      ]);
      setDeck(deckData);
      setCards(cardsData);
      setTags(tagsData);
      setName(deckData.name);
      setDescription(deckData.description);
      setFuzzEnabled(deckData.fuzz_enabled);
      setNewCardFrontTemplate(deckData.new_card_front_template ?? '');
      setNewCardBackTemplate(deckData.new_card_back_template ?? '');
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load deck');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    if (id) {
      void loadDeck();
    }
  }, [id, loadDeck]);

  const handleUpdateDeck = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError('Deck name is required');
      return;
    }

    setSavingDeck(true);
    try {
      await api.updateDeck(
        id,
        trimmedName,
        description,
        fuzzEnabled,
        newCardFrontTemplate,
        newCardBackTemplate,
      );
      await loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update deck');
    } finally {
      setSavingDeck(false);
    }
  };

  const handleAddCardToggle = () => {
    if (showAddCard) {
      setShowAddCard(false);
      return;
    }

    setNewFront(deck?.new_card_front_template ?? '');
    setNewBack(deck?.new_card_back_template ?? '');
    setNewLink('');
    setShowAddCard(true);
  };

  const handleAddCard = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;

    const normalizedLink = normalizeOptionalExternalLink(newLink);
    if (newLink.trim() && !normalizedLink) {
      setError('Link must be a valid http or https URL');
      return;
    }

    try {
      await api.createCard(id, newFront, newBack, normalizedLink ?? '');
      setNewFront('');
      setNewBack('');
      setNewLink('');
      setShowAddCard(false);
      await loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add card');
    }
  };

  const handleEditCard = async (cardId: string) => {
    const normalizedLink = normalizeOptionalExternalLink(editLink);
    if (editLink.trim() && !normalizedLink) {
      setError('Link must be a valid http or https URL');
      return;
    }

    try {
      await api.updateCard(cardId, editFront, editBack, normalizedLink ?? '', editTagIds);
      setEditingCard(null);
      await loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update card');
    }
  };

  const handleAddTag = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !newTagName.trim()) return;
    try {
      await api.createTag(id, newTagName.trim());
      setNewTagName('');
      await loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create tag');
    }
  };

  const handleDeleteTag = async (tagId: string) => {
    if (!confirm('Delete this tag? It will be removed from all cards.')) return;
    try {
      await api.deleteTag(tagId);
      if (filterTagId === tagId) setFilterTagId('');
      await loadDeck();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete tag');
    }
  };

  const toggleEditTag = (tagId: string) => {
    setEditTagIds(prev =>
      prev.includes(tagId)
        ? prev.filter(id => id !== tagId)
        : [...prev, tagId]
    );
  };

  const handleDeleteCard = async (cardId: string) => {
    if (!confirm('Delete this card?')) return;
    try {
      await api.deleteCard(cardId);
      await loadDeck();
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
    setEditTagIds(card.tags?.map(t => t.id) || []);
  };

  const focusTab = (tab: Tab) => {
    if (tab === 'cards') {
      cardsTabRef.current?.focus();
      return;
    }

    settingsTabRef.current?.focus();
  };

  const handleTabKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, tab: Tab) => {
    let nextTab: Tab | null = null;

    switch (event.key) {
      case 'ArrowLeft':
      case 'ArrowUp':
        nextTab = tab === 'cards' ? 'settings' : 'cards';
        break;
      case 'ArrowRight':
      case 'ArrowDown':
        nextTab = tab === 'cards' ? 'settings' : 'cards';
        break;
      case 'Home':
        nextTab = 'cards';
        break;
      case 'End':
        nextTab = 'settings';
        break;
      default:
        return;
    }

    event.preventDefault();
    setActiveTab(nextTab);
    focusTab(nextTab);
  };

  if (loading) {
    return (
      <div className="deck-edit-container" role="status" aria-live="polite">
        Loading...
      </div>
    );
  }
  if (!deck) {
    return (
      <div className="deck-edit-container">
        <div className="deck-edit-header">
          <button onClick={() => navigate('/')} className="back-btn">
            Back to Decks
          </button>
          <h1>{error ? 'Unable to Load Deck' : 'Deck not found'}</h1>
        </div>

        {error ? (
          <>
            <div className="error" role="alert">{error}</div>
            <button onClick={() => void loadDeck()} className="btn-secondary">
              Retry
            </button>
          </>
        ) : (
          <p>This deck could not be found.</p>
        )}
      </div>
    );
  }

  return (
    <div className="deck-edit-container">
      <div className="deck-edit-header">
        <button onClick={() => navigate('/')} className="back-btn">
          Back to Decks
        </button>
        <h1>{deck.name}</h1>
      </div>

      {error && <div className="error" role="alert">{error}</div>}

      <div className="tabs" role="tablist" aria-label="Deck editor sections">
        <button
          type="button"
          ref={cardsTabRef}
          id={CARDS_TAB_ID}
          role="tab"
          className={`tab ${activeTab === 'cards' ? 'active' : ''}`}
          aria-selected={activeTab === 'cards'}
          aria-controls={CARDS_PANEL_ID}
          tabIndex={activeTab === 'cards' ? 0 : -1}
          onClick={() => setActiveTab('cards')}
          onKeyDown={(event) => handleTabKeyDown(event, 'cards')}
        >
          Cards ({cards.length})
        </button>
        <button
          type="button"
          ref={settingsTabRef}
          id={SETTINGS_TAB_ID}
          role="tab"
          className={`tab ${activeTab === 'settings' ? 'active' : ''}`}
          aria-selected={activeTab === 'settings'}
          aria-controls={SETTINGS_PANEL_ID}
          tabIndex={activeTab === 'settings' ? 0 : -1}
          onClick={() => setActiveTab('settings')}
          onKeyDown={(event) => handleTabKeyDown(event, 'settings')}
        >
          Settings
        </button>
      </div>

      <section
        id={SETTINGS_PANEL_ID}
        role="tabpanel"
        aria-labelledby={SETTINGS_TAB_ID}
        hidden={activeTab !== 'settings'}
      >
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
            <div className="deck-template-section">
              <h3>New Card Templates</h3>
              <div className="card-form-grid">
                <div className="form-group">
                  <label htmlFor="new-card-front-template">Front Template</label>
                  <textarea
                    id="new-card-front-template"
                    value={newCardFrontTemplate}
                    onChange={(e) => setNewCardFrontTemplate(e.target.value)}
                    rows={6}
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="new-card-back-template">Back Template</label>
                  <textarea
                    id="new-card-back-template"
                    value={newCardBackTemplate}
                    onChange={(e) => setNewCardBackTemplate(e.target.value)}
                    rows={6}
                  />
                </div>
              </div>
            </div>
            <div className="deck-setting-toggle">
              <label htmlFor="fuzz-enabled" className="deck-setting-toggle-label">
                <span className="deck-setting-toggle-copy">
                  <span className="deck-setting-toggle-title">Enable Fuzz For Long-Term Reviews</span>
                  <span className="deck-setting-toggle-description">
                    Slightly randomize day-based review intervals to spread future workload. Short learning steps are not affected.
                  </span>
                </span>
                <input
                  id="fuzz-enabled"
                  type="checkbox"
                  checked={fuzzEnabled}
                  onChange={(e) => setFuzzEnabled(e.target.checked)}
                />
              </label>
            </div>
            <button type="submit" disabled={savingDeck || !name.trim()}>
              {savingDeck ? 'Saving...' : 'Save Changes'}
            </button>
          </form>

          <div className="tags-section">
            <h3>Tags</h3>
            <p>Create tags to categorize your cards. Tags can be assigned to cards when editing them.</p>
            <form onSubmit={handleAddTag} className="add-tag-form">
              <label className="sr-only" htmlFor="new-tag-name">New tag name</label>
              <input
                id="new-tag-name"
                type="text"
                value={newTagName}
                onChange={(e) => setNewTagName(e.target.value)}
                placeholder="New tag name..."
                maxLength={100}
              />
              <button type="submit" disabled={!newTagName.trim()}>Add Tag</button>
            </form>
            {tags.length > 0 ? (
              <div className="tags-list">
                {tags.map(tag => (
                  <div key={tag.id} className="tag-item">
                    <span className="tag-name">{tag.name}</span>
                    <button
                      type="button"
                      onClick={() => handleDeleteTag(tag.id)}
                      className="btn-icon btn-icon-danger"
                      aria-label={`Delete tag ${tag.name}`}
                      title="Delete tag"
                    >
                      ✕
                    </button>
                  </div>
                ))}
              </div>
            ) : (
              <p className="no-tags">No tags yet. Create your first tag above.</p>
            )}
          </div>

          <div className="danger-zone">
            <h3>Danger Zone</h3>
            <p>Once you delete a deck, there is no going back. Please be certain.</p>
            <button onClick={handleDeleteDeck} className="btn-danger">
              Delete Deck
            </button>
          </div>
        </>
      </section>

      <section
        id={CARDS_PANEL_ID}
        role="tabpanel"
        aria-labelledby={CARDS_TAB_ID}
        hidden={activeTab !== 'cards'}
      >
        <div className="cards-section">
          <div className="cards-toolbar">
            <div className="search-box">
              <label className="sr-only" htmlFor="card-search">Search cards</label>
              <input
                id="card-search"
                type="text"
                placeholder="Search cards..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="search-input"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => setSearchQuery('')}
                  className="search-clear"
                  aria-label="Clear search"
                  title="Clear search"
                >
                  ✕
                </button>
              )}
            </div>
            <div className="cards-toolbar-right">
              {tags.length > 0 && (
                <>
                  <label className="sr-only" htmlFor="card-tag-filter">Filter cards by tag</label>
                  <select
                    id="card-tag-filter"
                    value={filterTagId}
                    onChange={(e) => setFilterTagId(e.target.value)}
                    className="tag-filter-select"
                  >
                    <option value="">All Tags</option>
                    {tags.map(tag => (
                      <option key={tag.id} value={tag.id}>{tag.name}</option>
                    ))}
                  </select>
                </>
              )}
              <label className="sr-only" htmlFor="card-sort">Sort cards</label>
              <select
                id="card-sort"
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value as SortOption)}
                className="sort-select"
              >
                <option value="newest">Newest First</option>
                <option value="oldest">Oldest First</option>
                <option value="alpha">Alphabetical</option>
                <option value="mostReviews">Most Reviews</option>
                <option value="leastReviews">Least Reviews</option>
              </select>
              <button onClick={handleAddCardToggle}>
                {showAddCard ? 'Cancel' : 'Add Card'}
              </button>
            </div>
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
                  {newCardDuplicates.length > 0 && (
                    <div className="duplicate-warning" role="status" aria-live="polite">
                      <strong className="duplicate-warning-title">Possible duplicate in this deck</strong>
                      <p className="duplicate-warning-copy">
                        {newCardDuplicates.length === 1
                          ? '1 existing card has the same first-line title.'
                          : `${newCardDuplicates.length} existing cards have the same first-line title.`}
                      </p>
                      <ul className="duplicate-warning-list">
                        {newCardDuplicates.map((card) => (
                          <li key={card.id}>
                            <span className="duplicate-warning-card">{getCardTitle(card.front)}</span>
                            {getCardTitle(card.back) && (
                              <span className="duplicate-warning-card-meta">
                                {' '}
                                - {getCardTitle(card.back)}
                              </span>
                            )}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
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
            ) : filteredCards.length === 0 ? (
              <p className="no-cards">No cards match your search.</p>
            ) : (
              filteredCards.map((card) => {
                const safeLink = normalizeOptionalExternalLink(card.link);
                const cardTitle = getCardTitle(card.front);
                const editFrontId = `edit-front-${card.id}`;
                const editBackId = `edit-back-${card.id}`;
                const editLinkId = `edit-link-${card.id}`;
                const tagsGroupLabelId = `edit-tags-${card.id}`;

                return (
                  <div key={card.id} className="card-item">
                    {editingCard === card.id ? (
                      <div className="card-edit">
                        <p className="form-hint">
                          Supports markdown: **bold**, *italic*, `inline code`, and code blocks with ```
                        </p>
                        <div className="card-form-grid">
                          <div className="form-group">
                            <label htmlFor={editFrontId}>Front</label>
                            <textarea
                              id={editFrontId}
                              value={editFront}
                              onChange={(e) => setEditFront(e.target.value)}
                              rows={6}
                            />
                            {editCardDuplicates.length > 0 && (
                              <div className="duplicate-warning" role="status" aria-live="polite">
                                <strong className="duplicate-warning-title">Possible duplicate in this deck</strong>
                                <p className="duplicate-warning-copy">
                                  {editCardDuplicates.length === 1
                                    ? '1 existing card has the same first-line title.'
                                    : `${editCardDuplicates.length} existing cards have the same first-line title.`}
                                </p>
                                <ul className="duplicate-warning-list">
                                  {editCardDuplicates.map((duplicateCard) => (
                                    <li key={duplicateCard.id}>
                                      <span className="duplicate-warning-card">
                                        {getCardTitle(duplicateCard.front)}
                                      </span>
                                      {getCardTitle(duplicateCard.back) && (
                                        <span className="duplicate-warning-card-meta">
                                          {' '}
                                          - {getCardTitle(duplicateCard.back)}
                                        </span>
                                      )}
                                    </li>
                                  ))}
                                </ul>
                              </div>
                            )}
                          </div>
                          <div className="form-group">
                            <label htmlFor={editBackId}>Back</label>
                            <textarea
                              id={editBackId}
                              value={editBack}
                              onChange={(e) => setEditBack(e.target.value)}
                              rows={6}
                            />
                          </div>
                        </div>
                        <div className="form-group">
                          <label htmlFor={editLinkId}>Link (optional)</label>
                          <input
                            id={editLinkId}
                            type="url"
                            value={editLink}
                            onChange={(e) => setEditLink(e.target.value)}
                            placeholder="https://..."
                          />
                        </div>
                        {tags.length > 0 && (
                          <div className="form-group">
                            <p id={tagsGroupLabelId} className="form-label">Tags</p>
                            <div className="tag-selector" role="group" aria-labelledby={tagsGroupLabelId}>
                              {tags.map(tag => (
                                <button
                                  key={tag.id}
                                  type="button"
                                  className={`tag-btn ${editTagIds.includes(tag.id) ? 'active' : ''}`}
                                  onClick={() => toggleEditTag(tag.id)}
                                >
                                  {tag.name}
                                </button>
                              ))}
                            </div>
                          </div>
                        )}
                        <div className="card-edit-actions">
                          <button onClick={() => handleEditCard(card.id)}>Save</button>
                          <button onClick={() => setEditingCard(null)} className="btn-secondary">Cancel</button>
                        </div>
                      </div>
                    ) : (
                      <div className="card-row">
                        <button type="button" className="card-preview" onClick={() => startEditing(card)}>
                          <div className="card-preview-main">
                            <span className="card-preview-text">{cardTitle}</span>
                            {card.tags && card.tags.length > 0 && (
                              <div className="card-tags">
                                {card.tags.map(tag => (
                                  <span key={tag.id} className="card-tag">{tag.name}</span>
                                ))}
                              </div>
                            )}
                          </div>
                          {card.state && (
                            <span className="card-preview-meta">
                              {card.state.reps > 0 ? `${card.state.reps} reps` : 'New'}
                            </span>
                          )}
                        </button>
                        <div className="card-row-actions">
                          {safeLink && (
                            <a
                              href={safeLink}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="btn-icon"
                              aria-label={`Open link for ${cardTitle}`}
                              title="Open link"
                            >
                              ↗
                            </a>
                          )}
                          <button
                            type="button"
                            onClick={() => startEditing(card)}
                            className="btn-icon"
                            aria-label={`Edit card ${cardTitle}`}
                            title="Edit"
                          >
                            ✎
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDeleteCard(card.id)}
                            className="btn-icon btn-icon-danger"
                            aria-label={`Delete card ${cardTitle}`}
                            title="Delete"
                          >
                            ✕
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
