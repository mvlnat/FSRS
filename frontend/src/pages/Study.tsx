import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
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

  const renderInlineFormatting = (text: string, keyPrefix: string): ReactNode[] => {
    // Parse inline formatting into React elements (no dangerouslySetInnerHTML)
    const result: ReactNode[] = [];
    let remaining = text;
    let partIndex = 0;

    while (remaining.length > 0) {
      // Find the earliest match
      const codeMatch = remaining.match(/`([^`]+)`/);
      const boldMatch = remaining.match(/\*\*([^*]+)\*\*/);
      const italicMatch = remaining.match(/\*([^*]+)\*/);

      const matches = [
        codeMatch ? { type: 'code', match: codeMatch, index: codeMatch.index! } : null,
        boldMatch ? { type: 'bold', match: boldMatch, index: boldMatch.index! } : null,
        italicMatch ? { type: 'italic', match: italicMatch, index: italicMatch.index! } : null,
      ].filter(Boolean).sort((a, b) => a!.index - b!.index);

      if (matches.length === 0) {
        // No more matches, add remaining text
        if (remaining) result.push(remaining);
        break;
      }

      const earliest = matches[0]!;

      // Add text before the match
      if (earliest.index > 0) {
        result.push(remaining.slice(0, earliest.index));
      }

      // Add the formatted element
      const content = earliest.match[1];
      const key = `${keyPrefix}-${partIndex++}`;

      if (earliest.type === 'code') {
        result.push(<code key={key} className="inline-code">{content}</code>);
      } else if (earliest.type === 'bold') {
        result.push(<strong key={key}>{content}</strong>);
      } else if (earliest.type === 'italic') {
        result.push(<em key={key}>{content}</em>);
      }

      remaining = remaining.slice(earliest.index + earliest.match[0].length);
    }

    return result;
  };

  const renderMarkdown = (text: string) => {
    // Simple markdown rendering for code blocks and basic formatting
    const lines = text.split('\n');
    const elements: ReactNode[] = [];
    let inCodeBlock = false;
    let codeContent: string[] = [];
    let codeLanguage = '';

    lines.forEach((line, i) => {
      if (line.startsWith('```')) {
        if (inCodeBlock) {
          // End code block
          elements.push(
            <pre key={`code-${i}`} className="code-block">
              <code className={codeLanguage ? `language-${codeLanguage}` : ''}>
                {codeContent.join('\n')}
              </code>
            </pre>
          );
          codeContent = [];
          codeLanguage = '';
          inCodeBlock = false;
        } else {
          // Start code block
          inCodeBlock = true;
          codeLanguage = line.slice(3).trim();
        }
      } else if (inCodeBlock) {
        codeContent.push(line);
      } else {
        // Handle inline code and bold/italic safely
        elements.push(
          <p key={i}>{renderInlineFormatting(line, `line-${i}`)}</p>
        );
      }
    });

    // Handle unclosed code block
    if (inCodeBlock && codeContent.length > 0) {
      elements.push(
        <pre key="code-final" className="code-block">
          <code>{codeContent.join('\n')}</code>
        </pre>
      );
    }

    return elements;
  };

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
              <div className="flashcard-text">{renderMarkdown(currentCard.front)}</div>
            </div>

            {showBack && (
              <div className="flashcard-back">
                <h3>Answer</h3>
                <div className="flashcard-text">{renderMarkdown(currentCard.back)}</div>
                {currentCard.link && (
                  <a href={currentCard.link} target="_blank" rel="noopener noreferrer" className="card-link-btn">
                    Open Link
                  </a>
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
