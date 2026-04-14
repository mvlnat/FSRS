import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import Markdown from 'react-markdown';
import type { CardWithState, Rating } from '../types';
import { RATING_LABELS } from '../types';
import * as api from '../api/client';

export function Study() {
  const { deckId } = useParams<{ deckId: string }>();
  const navigate = useNavigate();
  const [cards, setCards] = useState<CardWithState[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [showBack, setShowBack] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [completed, setCompleted] = useState(0);
  const [totalInSession, setTotalInSession] = useState(0);

  const loadCards = useCallback(async (isInitial = false) => {
    if (!deckId) return;
    try {
      const data = await api.getDueCards(deckId);
      setCards(data);
      setCurrentIndex(0);
      setShowBack(false);
      if (isInitial && data.length > 0) {
        setTotalInSession(data.length);
        setCompleted(0);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load cards');
    } finally {
      setLoading(false);
    }
  }, [deckId]);

  useEffect(() => {
    if (deckId) loadCards(true);
  }, [deckId, loadCards]);

  const handleRating = useCallback(async (rating: Rating) => {
    const card = cards[currentIndex];
    if (!card) return;

    try {
      await api.reviewCard(card.id, rating);
      setCompleted(c => c + 1);

      // Move to next card
      if (currentIndex < cards.length - 1) {
        setCurrentIndex(i => i + 1);
        setShowBack(false);
      } else {
        // Session complete - reload to get any new due cards
        setLoading(true);
        await loadCards();
        setLoading(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit review');
    }
  }, [cards, currentIndex, loadCards]);

  // Keyboard shortcuts
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    // Ignore if user is typing in an input
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
      return;
    }

    const currentCard = cards[currentIndex];
    if (!currentCard || loading) return;

    if (e.code === 'Space') {
      e.preventDefault();
      if (!showBack) {
        setShowBack(true);
      }
    } else if (showBack && e.key >= '1' && e.key <= '4') {
      e.preventDefault();
      handleRating(parseInt(e.key) as Rating);
    }
  }, [cards, currentIndex, showBack, loading, handleRating]);

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  if (loading) return <div className="study-container">Loading...</div>;

  const currentCard = cards[currentIndex];
  const isComplete = !currentCard || cards.length === 0;

  return (
    <div className="study-container">
      <button onClick={() => navigate('/')} className="back-btn">
        Back to Decks
      </button>

      {error && <div className="error">{error}</div>}

      {totalInSession > 0 && (
        <div className="progress-bar-container">
          <div
            className="progress-bar"
            style={{ width: `${(completed / totalInSession) * 100}%` }}
          />
        </div>
      )}

      <div className="study-progress">
        <span>Completed: {completed}/{totalInSession}</span>
        <span>Remaining: {cards.length - currentIndex}</span>
      </div>

      {isComplete ? (
        <div className="study-complete">
          <h2>Session Complete!</h2>
          <p>You've reviewed all due cards.</p>
          <button onClick={() => navigate('/')}>Back to Decks</button>
        </div>
      ) : (
        <div className="flashcard">
          <div className="flashcard-content">
            <div className="flashcard-front">
              <h3>Question</h3>
              <div className="flashcard-text"><Markdown>{currentCard.front}</Markdown></div>
            </div>

            {showBack && (
              <div className="flashcard-back">
                <h3>Answer</h3>
                <div className="flashcard-text"><Markdown>{currentCard.back}</Markdown></div>
                {currentCard.link && (
                  <div className="card-link-wrapper">
                    <a href={currentCard.link} target="_blank" rel="noopener noreferrer" className="card-link-btn">
                      Open Link
                    </a>
                  </div>
                )}
              </div>
            )}
          </div>

          {!showBack ? (
            <button onClick={() => setShowBack(true)} className="show-answer-btn">
              Show Answer <span className="shortcut-hint">(Space)</span>
            </button>
          ) : (
            <div className="rating-buttons">
              <p>How well did you know this?</p>
              <div className="ratings">
                {([1, 2, 3, 4] as Rating[]).map((rating) => (
                  <button
                    key={rating}
                    onClick={() => handleRating(rating)}
                    className={`rating-btn rating-${rating}`}
                  >
                    <span className="shortcut-hint">{rating}</span> {RATING_LABELS[rating]}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
