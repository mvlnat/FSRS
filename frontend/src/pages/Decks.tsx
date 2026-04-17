import { useState, useEffect, useMemo, useRef } from 'react';
import { Link } from 'react-router-dom';
import type { DeckWithStats, DueCalendarDay } from '../types';
import * as api from '../api/client';

const WEEKDAY_LABELS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

const monthFormatter = new Intl.DateTimeFormat('en-US', {
  month: 'long',
  year: 'numeric',
});

const fullDateFormatter = new Intl.DateTimeFormat('en-US', {
  weekday: 'long',
  month: 'long',
  day: 'numeric',
  year: 'numeric',
});

type CalendarCell = {
  date: Date;
  dateKey: string;
  isCurrentMonth: boolean;
  isToday: boolean;
  total: number;
};

function padDatePart(value: number): string {
  return String(value).padStart(2, '0');
}

function getDateKey(date: Date): string {
  return `${date.getFullYear()}-${padDatePart(date.getMonth() + 1)}-${padDatePart(date.getDate())}`;
}

function parseDateKey(value: string): Date {
  const [year, month, day] = value.split('-').map(Number);
  return new Date(year, month - 1, day);
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

function addMonths(date: Date, months: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function getCalendarRangeStart(date: Date): Date {
  const firstOfMonth = startOfMonth(date);
  return addDays(firstOfMonth, -firstOfMonth.getDay());
}

function getCalendarRangeEnd(date: Date): Date {
  const lastOfMonth = new Date(date.getFullYear(), date.getMonth() + 1, 0);
  return addDays(lastOfMonth, 6 - lastOfMonth.getDay());
}

function buildCalendarCells(visibleMonth: Date, scheduleByDate: Map<string, DueCalendarDay>): CalendarCell[] {
  const rangeStart = getCalendarRangeStart(visibleMonth);
  const rangeEnd = getCalendarRangeEnd(visibleMonth);
  const todayKey = getDateKey(new Date());
  const cells: CalendarCell[] = [];

  for (let cursor = rangeStart; cursor <= rangeEnd; cursor = addDays(cursor, 1)) {
    const dateKey = getDateKey(cursor);
    cells.push({
      date: cursor,
      dateKey,
      isCurrentMonth: cursor.getMonth() === visibleMonth.getMonth(),
      isToday: dateKey === todayKey,
      total: scheduleByDate.get(dateKey)?.total ?? 0,
    });
  }

  return cells;
}

function getCalendarTone(total: number, maxTotal: number): 'empty' | 'low' | 'medium' | 'high' | 'peak' {
  if (total <= 0 || maxTotal <= 0) {
    return 'empty';
  }

  const ratio = total / maxTotal;
  if (ratio >= 0.8) {
    return 'peak';
  }
  if (ratio >= 0.55) {
    return 'high';
  }
  if (ratio >= 0.3) {
    return 'medium';
  }
  return 'low';
}

function formatDueCount(total: number): string {
  return `${total} card${total === 1 ? '' : 's'} due`;
}

function formatReviewCount(value: number): string {
  return value.toLocaleString();
}

function formatAvgRating(value: number): string {
  if (value <= 0) {
    return '—';
  }

  return `${value.toFixed(1)}/4`;
}

function getStudyStatsSummary(stats: api.StudyStats): string {
  const reviewNoun = stats.totalReviews === 1 ? 'review' : 'reviews';
  const formattedTotal = formatReviewCount(stats.totalReviews);

  if (stats.totalReviews === stats.reviewsLast24Hours) {
    return `All ${formattedTotal} recorded ${reviewNoun} happened in the last 24 hours.`;
  }

  if (stats.totalReviews === stats.reviewsLast7Days) {
    return `All ${formattedTotal} recorded ${reviewNoun} are from the last 7 days.`;
  }

  return `${formattedTotal} recorded ${reviewNoun} so far.`;
}

export function Decks() {
  const [decks, setDecks] = useState<DeckWithStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDescription, setNewDescription] = useState('');
  const [stats, setStats] = useState<api.StudyStats | null>(null);
  const [dueCalendar, setDueCalendar] = useState<DueCalendarDay[]>([]);
  const [dueCalendarLoading, setDueCalendarLoading] = useState(true);
  const [dueCalendarError, setDueCalendarError] = useState('');
  const [visibleMonth, setVisibleMonth] = useState(() => startOfMonth(new Date()));
  const [selectedDateKey, setSelectedDateKey] = useState(() => getDateKey(new Date()));
  const fileInputRef = useRef<HTMLInputElement>(null);
  const browserTimeZone = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', []);

  const scheduleRange = useMemo(() => {
    const start = getCalendarRangeStart(visibleMonth);
    const end = getCalendarRangeEnd(visibleMonth);

    return {
      start: getDateKey(start),
      end: getDateKey(end),
    };
  }, [visibleMonth]);

  const scheduleByDate = useMemo(
    () => new Map(dueCalendar.map((day) => [day.date, day])),
    [dueCalendar],
  );

  const calendarCells = useMemo(
    () => buildCalendarCells(visibleMonth, scheduleByDate),
    [visibleMonth, scheduleByDate],
  );

  const maxDueCount = useMemo(
    () => calendarCells.reduce((currentMax, cell) => Math.max(currentMax, cell.total), 0),
    [calendarCells],
  );

  const selectedDay = scheduleByDate.get(selectedDateKey);
  const selectedDate = parseDateKey(selectedDateKey);
  const currentMonthTotal = useMemo(
    () => dueCalendar.reduce((total, day) => {
      const date = parseDateKey(day.date);
      return date.getMonth() === visibleMonth.getMonth() && date.getFullYear() === visibleMonth.getFullYear()
        ? total + day.total
        : total;
    }, 0),
    [dueCalendar, visibleMonth],
  );

  useEffect(() => {
    loadDecks();
    loadStats();
  }, []);

  useEffect(() => {
    let cancelled = false;

    async function loadDueCalendar() {
      try {
        setDueCalendarLoading(true);
        const data = await api.getDueCalendar(scheduleRange.start, scheduleRange.end, browserTimeZone);
        if (cancelled) {
          return;
        }
        setDueCalendar(data);
        setDueCalendarError('');
      } catch (err) {
        if (cancelled) {
          return;
        }
        setDueCalendarError(err instanceof Error ? err.message : 'Failed to load due calendar');
      } finally {
        if (!cancelled) {
          setDueCalendarLoading(false);
        }
      }
    }

    void loadDueCalendar();

    return () => {
      cancelled = true;
    };
  }, [browserTimeZone, scheduleRange.end, scheduleRange.start]);

  const loadDecks = async () => {
    try {
      const data = await api.getDecks();
      setDecks(data);
      setError('');
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
    } catch {
      // Stats are not critical, silently ignore
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedName = newName.trim();
    if (!trimmedName) {
      setError('Deck name is required');
      return;
    }

    try {
      await api.createDeck(trimmedName, newDescription);
      setNewName('');
      setNewDescription('');
      setShowCreate(false);
      await loadDecks();
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
      await loadDecks();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import deck');
    } finally {
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const handleMonthChange = (months: number) => {
    const nextMonth = addMonths(visibleMonth, months);
    setVisibleMonth(nextMonth);
    setSelectedDateKey(getDateKey(nextMonth));
  };

  const handleToday = () => {
    const today = startOfMonth(new Date());
    setVisibleMonth(today);
    setSelectedDateKey(getDateKey(new Date()));
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
          <div className="study-stats-header">
            <div>
              <h2>Study Statistics</h2>
              <p className="study-stats-subtitle">Recent activity and answer quality</p>
            </div>
            <p className="study-stats-summary">{getStudyStatsSummary(stats)}</p>
          </div>
          <div className="stats-grid">
            <div className="stat-item">
              <span className="stat-value">{formatReviewCount(stats.reviewsLast24Hours)}</span>
              <span className="stat-label">Last 24 Hours</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{formatReviewCount(stats.reviewsLast7Days)}</span>
              <span className="stat-label">Last 7 Days</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{formatAvgRating(stats.avgRating)}</span>
              <span className="stat-label">Average Rating</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{stats.retentionRate.toFixed(0)}%</span>
              <span className="stat-label">Good or Easy</span>
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
          <button type="submit" disabled={!newName.trim()}>
            Create Deck
          </button>
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

      <section className="due-calendar-card" aria-labelledby="due-calendar-title">
        <div className="due-calendar-header">
          <div>
            <h2 id="due-calendar-title">Due Calendar</h2>
            <p className="due-calendar-subtitle">Scheduled cards across all decks</p>
          </div>
          <div className="due-calendar-header-actions">
            <p className="due-calendar-summary">{formatDueCount(currentMonthTotal)} this month</p>
            <div className="due-calendar-nav">
              <button
                type="button"
                className="btn-ghost due-calendar-nav-btn"
                onClick={() => handleMonthChange(-1)}
                aria-label="Show previous month"
              >
                Prev
              </button>
              <span className="due-calendar-month">{monthFormatter.format(visibleMonth)}</span>
              <button
                type="button"
                className="btn-ghost due-calendar-nav-btn"
                onClick={() => handleMonthChange(1)}
                aria-label="Show next month"
              >
                Next
              </button>
              <button
                type="button"
                className="btn-secondary due-calendar-today-btn"
                onClick={handleToday}
              >
                Today
              </button>
            </div>
          </div>
        </div>

        {dueCalendarError && <div className="error">{dueCalendarError}</div>}

        <div className="due-calendar-layout">
          <div className="due-calendar-grid" aria-busy={dueCalendarLoading}>
            <div className="due-calendar-weekdays" aria-hidden="true">
              {WEEKDAY_LABELS.map((label) => (
                <span key={label}>{label}</span>
              ))}
            </div>
            <div className="due-calendar-days">
              {calendarCells.map((cell) => (
                <button
                  key={cell.dateKey}
                  type="button"
                  className={`due-calendar-day${cell.isCurrentMonth ? '' : ' is-outside-month'}${cell.isToday ? ' is-today' : ''}${selectedDateKey === cell.dateKey ? ' is-selected' : ''}`}
                  data-tone={getCalendarTone(cell.total, maxDueCount)}
                  onClick={() => setSelectedDateKey(cell.dateKey)}
                  aria-label={`View due cards for ${cell.dateKey}`}
                  aria-pressed={selectedDateKey === cell.dateKey}
                >
                  <span className="due-calendar-day-number">{cell.date.getDate()}</span>
                  <span className="due-calendar-day-count" aria-hidden="true">
                    {cell.total > 0 ? formatDueCount(cell.total) : ''}
                  </span>
                </button>
              ))}
            </div>
          </div>

          <aside className="due-calendar-detail">
            <p className="due-calendar-detail-label">Selected Day</p>
            <h3>{fullDateFormatter.format(selectedDate)}</h3>
            {dueCalendarLoading ? (
              <p className="due-calendar-empty">Loading due cards…</p>
            ) : selectedDay ? (
              <>
                <p className="due-calendar-detail-total">{formatDueCount(selectedDay.total)}</p>
                <div className="due-calendar-deck-list">
                  {selectedDay.decks.map((deck) => (
                    <Link
                      key={deck.deck_id}
                      to={`/decks/${deck.deck_id}`}
                      className="due-calendar-deck-item"
                      aria-label={`${deck.deck_name}, ${formatDueCount(deck.count)}`}
                    >
                      <span>{deck.deck_name}</span>
                      <strong>{deck.count}</strong>
                    </Link>
                  ))}
                </div>
              </>
            ) : (
              <p className="due-calendar-empty">No cards are scheduled for this day.</p>
            )}
          </aside>
        </div>
      </section>
    </div>
  );
}
