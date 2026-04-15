import { Children, isValidElement, useCallback, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import Markdown from 'react-markdown';
import type { Components } from 'react-markdown';
import type { CardState, CardWithState, Rating } from '../types';
import { RATING_LABELS } from '../types';
import * as api from '../api/client';

type HighlightTokenType = 'plain' | 'comment' | 'string' | 'keyword' | 'number' | 'function' | 'property' | 'operator';

type HighlightToken = {
  type: HighlightTokenType;
  value: string;
};

type CodeElementProps = {
  className?: string;
  children?: ReactNode;
};

type PendingReview = {
  card: CardWithState;
  dueAt: number;
};

const LANGUAGE_ALIASES: Record<string, string> = {
  js: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  sh: 'bash',
  shell: 'bash',
  zsh: 'bash',
  yml: 'yaml',
  env: 'dotenv',
  plaintext: 'text',
};

const LANGUAGE_LABELS: Record<string, string> = {
  bash: 'Bash',
  dotenv: 'ENV',
  go: 'Go',
  javascript: 'JavaScript',
  json: 'JSON',
  sql: 'SQL',
  text: 'Code',
  typescript: 'TypeScript',
  yaml: 'YAML',
};

const LANGUAGE_KEYWORDS: Record<string, Set<string>> = {
  bash: new Set(['case', 'do', 'done', 'elif', 'else', 'esac', 'export', 'fi', 'for', 'function', 'if', 'in', 'local', 'then', 'until', 'while']),
  dotenv: new Set(['export']),
  go: new Set(['break', 'case', 'chan', 'const', 'continue', 'default', 'defer', 'else', 'fallthrough', 'for', 'func', 'go', 'if', 'import', 'interface', 'map', 'nil', 'package', 'range', 'return', 'select', 'struct', 'switch', 'type', 'var']),
  javascript: new Set(['async', 'await', 'break', 'case', 'catch', 'class', 'const', 'continue', 'default', 'else', 'export', 'extends', 'false', 'finally', 'for', 'from', 'function', 'if', 'import', 'let', 'new', 'null', 'return', 'switch', 'throw', 'true', 'try', 'typeof', 'undefined', 'while']),
  json: new Set(['false', 'null', 'true']),
  sql: new Set(['all', 'and', 'as', 'asc', 'between', 'by', 'case', 'create', 'delete', 'desc', 'distinct', 'drop', 'else', 'end', 'from', 'group', 'having', 'in', 'inner', 'insert', 'into', 'join', 'left', 'limit', 'not', 'null', 'on', 'or', 'order', 'right', 'select', 'set', 'table', 'then', 'union', 'update', 'values', 'when', 'where']),
  text: new Set(),
  typescript: new Set(['as', 'async', 'await', 'break', 'case', 'catch', 'class', 'const', 'continue', 'default', 'else', 'enum', 'export', 'extends', 'false', 'finally', 'for', 'from', 'function', 'if', 'implements', 'import', 'interface', 'let', 'new', 'null', 'readonly', 'return', 'switch', 'throw', 'true', 'try', 'type', 'typeof', 'undefined', 'while']),
  yaml: new Set(['false', 'no', 'null', 'off', 'on', 'true', 'yes']),
};

const IDENTIFIER_PATTERN = /\b[A-Za-z_][A-Za-z0-9_$-]*\b/y;
const NUMBER_PATTERN = /\b(?:0x[0-9a-fA-F]+|\d+(?:\.\d+)?)\b/y;
const OPERATOR_PATTERN = /=>|==={0,1}|!==|&&|\|\||\?\?|\+=|-=|\*=|\/=|:=|[-+*/%<>!=|&~^]+/y;
const JSON_KEY_PATTERN = /"(?:\\.|[^"\\])*"(?=\s*:)/y;
const DOTENV_KEY_PATTERN = /[A-Za-z_][A-Za-z0-9_]*(?=\s*=)/y;
const YAML_KEY_PATTERN = /[A-Za-z_][A-Za-z0-9_-]*(?=\s*:)/y;
const STRING_PATTERNS = [
  /"(?:\\.|[^"\\])*"/y,
  /'(?:\\.|[^'\\])*'/y,
  /`(?:\\.|[^`\\])*`/y,
];

function normalizeLanguage(language?: string): string {
  if (!language) return 'text';
  return LANGUAGE_ALIASES[language.toLowerCase()] ?? language.toLowerCase();
}

function getLanguageLabel(language?: string): string {
  const normalized = normalizeLanguage(language);
  return LANGUAGE_LABELS[normalized] ?? normalized.toUpperCase();
}

function flattenText(node: ReactNode): string {
  return Children.toArray(node)
    .map((child) => {
      if (typeof child === 'string' || typeof child === 'number') {
        return String(child);
      }

      if (isValidElement<{ children?: ReactNode }>(child)) {
        return flattenText(child.props.children);
      }

      return '';
    })
    .join('');
}

function matchPattern(pattern: RegExp, value: string, index: number): string | null {
  pattern.lastIndex = index;
  const match = pattern.exec(value);
  if (!match || match.index !== index) {
    return null;
  }

  return match[0];
}

function matchAnyPattern(patterns: RegExp[], value: string, index: number): string | null {
  for (const pattern of patterns) {
    const match = matchPattern(pattern, value, index);
    if (match) {
      return match;
    }
  }

  return null;
}

function matchComment(language: string, value: string, index: number): string | null {
  const previousChar = index > 0 ? value[index - 1] : ' ';

  if ((language === 'bash' || language === 'dotenv' || language === 'yaml') && value[index] === '#' && /\s/.test(previousChar)) {
    return value.slice(index);
  }

  if (language === 'sql' && value.startsWith('--', index) && /\s/.test(previousChar)) {
    return value.slice(index);
  }

  if ((language === 'go' || language === 'javascript' || language === 'typescript') && value.startsWith('//', index)) {
    return value.slice(index);
  }

  return matchPattern(/\/\*.*?\*\//y, value, index);
}

function matchProperty(language: string, value: string, index: number): string | null {
  if (language === 'json') {
    return matchPattern(JSON_KEY_PATTERN, value, index);
  }

  if (language === 'dotenv') {
    return matchPattern(DOTENV_KEY_PATTERN, value, index);
  }

  if (language === 'yaml') {
    return matchPattern(YAML_KEY_PATTERN, value, index);
  }

  return null;
}

function looksLikeFunction(language: string, line: string, index: number, token: string): boolean {
  if (language === 'bash') {
    return line.slice(0, index).trim() === '';
  }

  return /^\s*\(/.test(line.slice(index + token.length));
}

function highlightCodeLine(line: string, language?: string): HighlightToken[] {
  const normalizedLanguage = normalizeLanguage(language);
  const keywordSet = LANGUAGE_KEYWORDS[normalizedLanguage] ?? LANGUAGE_KEYWORDS.text;
  const tokens: HighlightToken[] = [];

  const pushToken = (type: HighlightTokenType, value: string) => {
    if (!value) return;

    const previous = tokens[tokens.length - 1];
    if (previous && previous.type === type) {
      previous.value += value;
      return;
    }

    tokens.push({ type, value });
  };

  let index = 0;
  while (index < line.length) {
    const comment = matchComment(normalizedLanguage, line, index);
    if (comment) {
      pushToken('comment', comment);
      break;
    }

    const property = matchProperty(normalizedLanguage, line, index);
    if (property) {
      pushToken('property', property);
      index += property.length;
      continue;
    }

    const stringValue = matchAnyPattern(STRING_PATTERNS, line, index);
    if (stringValue) {
      pushToken('string', stringValue);
      index += stringValue.length;
      continue;
    }

    const numberValue = matchPattern(NUMBER_PATTERN, line, index);
    if (numberValue) {
      pushToken('number', numberValue);
      index += numberValue.length;
      continue;
    }

    const operatorValue = matchPattern(OPERATOR_PATTERN, line, index);
    if (operatorValue) {
      pushToken('operator', operatorValue);
      index += operatorValue.length;
      continue;
    }

    const identifier = matchPattern(IDENTIFIER_PATTERN, line, index);
    if (identifier) {
      const lowerIdentifier = identifier.toLowerCase();

      if (keywordSet.has(lowerIdentifier)) {
        pushToken('keyword', identifier);
      } else if (looksLikeFunction(normalizedLanguage, line, index, identifier)) {
        pushToken('function', identifier);
      } else {
        pushToken('plain', identifier);
      }

      index += identifier.length;
      continue;
    }

    pushToken('plain', line[index]);
    index += 1;
  }

  return tokens;
}

async function copyToClipboard(value: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    const textArea = document.createElement('textarea');
    textArea.value = value;
    textArea.setAttribute('readonly', '');
    textArea.style.position = 'absolute';
    textArea.style.left = '-9999px';
    document.body.appendChild(textArea);
    textArea.select();

    const copied = document.execCommand('copy');
    document.body.removeChild(textArea);
    return copied;
  }
}

function CodeBlock({ code, language }: { code: string; language?: string }) {
  const [copied, setCopied] = useState(false);
  const lines = code.split('\n');

  useEffect(() => {
    if (!copied) return;

    const timeoutId = window.setTimeout(() => setCopied(false), 1500);
    return () => window.clearTimeout(timeoutId);
  }, [copied]);

  const handleCopy = useCallback(async () => {
    const didCopy = await copyToClipboard(code);
    if (didCopy) {
      setCopied(true);
    }
  }, [code]);

  return (
    <div className="code-block">
      <div className="code-block-header">
        <div className="code-block-dots" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <span className="code-block-language">{getLanguageLabel(language)}</span>
        <button type="button" className="code-block-copy" onClick={handleCopy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <div className="code-block-body">
        <code>
          {lines.map((line, index) => (
            <span key={`${index}-${line}`} className="code-line">
              <span className="code-line-number" aria-hidden="true">
                {index + 1}
              </span>
              <span className="code-line-content">
                {highlightCodeLine(line, language).map((token, tokenIndex) => (
                  <span
                    key={`${token.type}-${tokenIndex}`}
                    className={token.type === 'plain' ? undefined : `code-token code-token-${token.type}`}
                  >
                    {token.value || ' '}
                  </span>
                ))}
                {line.length === 0 && ' '}
              </span>
            </span>
          ))}
        </code>
      </div>
    </div>
  );
}

const markdownComponents: Components = {
  pre({ children }) {
    const [codeNode] = Children.toArray(children);

    if (isValidElement<CodeElementProps>(codeNode)) {
      return (
        <CodeBlock
          code={flattenText(codeNode.props.children).replace(/\n$/, '')}
          language={codeNode.props.className?.replace('language-', '')}
        />
      );
    }

    return <pre>{children}</pre>;
  },
};

function shouldRequeueLearningCard(state: CardState): boolean {
  return state.scheduled_days === 0 && (state.state === 1 || state.state === 3);
}

function enqueuePendingReview(queue: PendingReview[], entry: PendingReview): PendingReview[] {
  return [...queue, entry].sort((a, b) => a.dueAt - b.dueAt);
}

function formatCountdown(ms: number): string {
  const totalSeconds = Math.max(1, Math.ceil(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  if (minutes === 0) {
    return `${seconds}s`;
  }

  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

export function Study() {
  const { deckId } = useParams<{ deckId: string }>();
  const navigate = useNavigate();
  const [cards, setCards] = useState<CardWithState[]>([]);
  const [pendingReviews, setPendingReviews] = useState<PendingReview[]>([]);
  const [showBack, setShowBack] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [completed, setCompleted] = useState(0);
  const [totalInSession, setTotalInSession] = useState(0);
  const [now, setNow] = useState(Date.now());

  const loadCards = useCallback(async (isInitial = false) => {
    if (!deckId) return;
    try {
      const data = await api.getDueCards(deckId);
      setCards(data);
      setShowBack(false);
      if (isInitial && data.length > 0) {
        setTotalInSession(data.length);
        setCompleted(0);
      }
      if (isInitial) {
        setPendingReviews([]);
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

  useEffect(() => {
    if (pendingReviews.length === 0) return;

    const intervalId = window.setInterval(() => {
      const currentTime = Date.now();
      setNow(currentTime);

      setPendingReviews((queue) => {
        const dueNow = queue.filter((entry) => entry.dueAt <= currentTime);
        if (dueNow.length === 0) {
          return queue;
        }

        setCards((currentCards) => {
          const existingIds = new Set(currentCards.map((card) => card.id));
          const newlyDueCards = dueNow
            .map((entry) => entry.card)
            .filter((card) => !existingIds.has(card.id));

          return newlyDueCards.length > 0 ? [...currentCards, ...newlyDueCards] : currentCards;
        });

        return queue.filter((entry) => entry.dueAt > currentTime);
      });
    }, 1000);

    return () => window.clearInterval(intervalId);
  }, [pendingReviews.length]);

  const handleRating = useCallback(async (rating: Rating) => {
    const [card, ...remainingCards] = cards;
    if (!card) return;

    try {
      const nextState = await api.reviewCard(card.id, rating);
      setCompleted(c => c + 1);
      setCards(remainingCards);
      setShowBack(false);
      setError('');

      if (shouldRequeueLearningCard(nextState)) {
        setPendingReviews((queue) => enqueuePendingReview(queue, {
          card: { ...card, state: nextState },
          dueAt: new Date(nextState.due).getTime(),
        }));
        setTotalInSession((count) => count + 1);
      }

      if (remainingCards.length === 0 && !shouldRequeueLearningCard(nextState)) {
        const freshDueCards = await api.getDueCards(deckId!);
        setCards(freshDueCards);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit review');
    }
  }, [cards, deckId]);

  // Keyboard shortcuts
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    // Ignore if user is typing in an input
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
      return;
    }

    const currentCard = cards[0];
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
  }, [cards, showBack, loading, handleRating]);

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  if (loading) return <div className="study-container">Loading...</div>;

  const currentCard = cards[0];
  const nextPendingReview = pendingReviews[0];
  const isWaiting = !currentCard && pendingReviews.length > 0;
  const isComplete = !currentCard && pendingReviews.length === 0;
  const nextReviewCountdown = nextPendingReview ? formatCountdown(nextPendingReview.dueAt - now) : null;

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
        <span>Due Now: {cards.length}</span>
        <span>Learning Queue: {pendingReviews.length}</span>
      </div>

      {isWaiting ? (
        <div className="study-waiting">
          <h2>Next Review Soon</h2>
          <p>
            This card is in a short learning step and will return in {nextReviewCountdown}.
          </p>
        </div>
      ) : isComplete ? (
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
              <div className="flashcard-text">
                <Markdown components={markdownComponents}>{currentCard.front}</Markdown>
              </div>
            </div>

            {showBack && (
              <div className="flashcard-back">
                <h3>Answer</h3>
                <div className="flashcard-text">
                  <Markdown components={markdownComponents}>{currentCard.back}</Markdown>
                </div>
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
