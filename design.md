# FSRS - Spaced Repetition Web App

## Tech Stack

- **Frontend**: TypeScript, React, Vite
- **Backend**: Go (Chi router, pgx, go-fsrs)
- **Database**: PostgreSQL
- **Hosting**: DigitalOcean Droplet (Docker Compose)

## Features

### Core
- User authentication (signup/login with JWT in httpOnly cookies)
- Create/edit/delete flashcard decks
- Create/edit/delete cards within decks
- Study session with FSRS algorithm
- Track review history and progress
- Import/export decks (JSON)

### Cards
- Front/back text with markdown support
- Fenced code blocks with language labels, line numbers, and lightweight syntax highlighting
- Optional link field
- Editing a card resets its FSRS schedule and review history
- Tags for categorization
- Search by front/back content
- Sort by date, alphabetical, review count
- Filter by tag

### Study
- Keyboard shortcuts (Space to flip, 1-4 for ratings)
- Progress bar
- Session statistics
- Dashboard study snapshot with rolling last 24 hours, rolling last 7 days, average rating, and retention
- Short learning steps stay in the study session and return when due
- When the visible queue runs out, the session pulls any newly due cards without requiring a manual refresh
- Review submissions are accepted only for due cards, with per-card locking to prevent duplicate reviews from double clicks or key repeats

---

## Project Structure

```text
fsrs/
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   │   └── client.ts        # API functions
│   │   ├── hooks/
│   │   │   ├── AuthProvider.tsx      # Auth provider
│   │   │   ├── AuthProvider.test.tsx
│   │   │   ├── auth-context.ts       # Auth context types and object
│   │   │   └── useAuth.tsx           # Auth hook
│   │   ├── pages/
│   │   │   ├── Login.tsx
│   │   │   ├── Register.tsx
│   │   │   ├── Decks.tsx        # Deck list with stats
│   │   │   ├── DeckEdit.tsx          # Cards, tags, settings
│   │   │   ├── DeckEdit.test.tsx
│   │   │   ├── Study.tsx             # Flashcard review
│   │   │   └── Study.test.tsx
│   │   ├── types/
│   │   │   └── index.ts
│   │   ├── test/
│   │   │   └── setup.ts        # Vitest + Testing Library setup
│   │   ├── App.tsx              # Routes & layout
│   │   └── App.css              # All styles
│   ├── package.json
│   └── Dockerfile
│
├── backend/
│   ├── cmd/server/
│   │   └── main.go              # Entry point, routes, migrations
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── auth.go
│   │   │   ├── deck.go
│   │   │   ├── card.go
│   │   │   ├── study.go
│   │   │   └── tag.go
│   │   ├── model/
│   │   │   ├── user.go
│   │   │   ├── deck.go
│   │   │   ├── card.go
│   │   │   ├── tag.go
│   │   │   └── review.go
│   │   ├── repository/
│   │   │   ├── postgres.go
│   │   │   ├── user.go
│   │   │   ├── deck.go
│   │   │   ├── card.go
│   │   │   └── tag.go
│   │   ├── service/
│   │   │   └── fsrs.go          # FSRS algorithm wrapper
│   │   └── middleware/
│   │       ├── auth.go          # JWT validation
│   │       └── ratelimit.go     # Rate limiting
│   ├── go.mod
│   └── Dockerfile
│
├── docker-compose.yml
├── nginx.conf
├── claude.md                    # Development guidelines
└── design.md                    # This file
```

---

## Database Schema

```sql
-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Decks
CREATE TABLE decks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_decks_user_id ON decks(user_id);

-- Cards
CREATE TABLE cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    link TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_cards_deck_id ON cards(deck_id);

-- Card States (FSRS scheduling data)
-- state: 0=New, 1=Learning, 2=Review, 3=Relearning
CREATE TABLE card_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL UNIQUE REFERENCES cards(id) ON DELETE CASCADE,
    due TIMESTAMPTZ NOT NULL,
    stability FLOAT DEFAULT 0,
    difficulty FLOAT DEFAULT 0,
    elapsed_days INT DEFAULT 0,
    scheduled_days INT DEFAULT 0,
    reps INT DEFAULT 0,
    lapses INT DEFAULT 0,
    state INT DEFAULT 0,
    last_review TIMESTAMPTZ
);
CREATE INDEX idx_card_states_due ON card_states(due);

-- Reviews (history log)
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 4),
    reviewed_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_reviews_card_id ON reviews(card_id);

-- Tags (per-deck)
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(deck_id, name)
);
CREATE INDEX idx_tags_deck_id ON tags(deck_id);

-- Card-Tag Association (many-to-many)
CREATE TABLE card_tags (
    card_id UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (card_id, tag_id)
);
CREATE INDEX idx_card_tags_card_id ON card_tags(card_id);
CREATE INDEX idx_card_tags_tag_id ON card_tags(tag_id);
```

---

## API Endpoints

```http
Auth (JWT in httpOnly cookie)
------------------------------
POST   /api/auth/register      { email, password }
POST   /api/auth/login         { email, password }
POST   /api/auth/logout
GET    /api/auth/me

Decks
------------------------------
GET    /api/decks              List with stats (batch query)
POST   /api/decks              { name, description }
POST   /api/decks/import       { name, description, cards[] }
GET    /api/decks/:id
PUT    /api/decks/:id          { name, description }
DELETE /api/decks/:id
GET    /api/decks/:id/stats
GET    /api/decks/:id/export

Cards
------------------------------
GET    /api/decks/:id/cards    List with state and tags
POST   /api/decks/:id/cards    { front, back, link }
GET    /api/cards/:id
PUT    /api/cards/:id          { front, back, link }
DELETE /api/cards/:id

Tags
------------------------------
GET    /api/decks/:deckId/tags
POST   /api/decks/:deckId/tags { name }
DELETE /api/tags/:tagId
PUT    /api/cards/:cardId/tags { tag_ids[] }

Study
------------------------------
GET    /api/study/stats
GET    /api/study/:deckId      Get due cards
POST   /api/study/:cardId/review { rating: 1-4 }
```

---

## Architecture Patterns

### Backend

**Repository Pattern**
- Each entity has a repository (UserRepository, DeckRepository, etc.)
- Repositories handle all database operations
- Handlers call repositories, not direct SQL

**Handler Structure**
```go
func (h *Handler) Method(w http.ResponseWriter, r *http.Request) {
    // 1. Get user from context (auth middleware)
    // 2. Parse request (URL params, body)
    // 3. Validate input
    // 4. Check authorization (ownership)
    // 5. Call repository
    // 6. Return JSON response
}
```

**Authorization Pattern**
- All resources checked for ownership
- Get resource → check if user owns parent deck → proceed or 403

### Frontend

**Auth Context Pattern**
```tsx
// useAuth.tsx
const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }) {
  const [state, setState] = useState({ user, loading, error });
  // login, register, logout functions
  return <AuthContext.Provider value={...}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  return useContext(AuthContext);
}

// App.tsx
<AuthProvider>
  <BrowserRouter>
    <Layout>
      <Routes>...</Routes>
    </Layout>
  </BrowserRouter>
</AuthProvider>
```

**Filtering/Sorting Pattern**
```tsx
const filteredItems = useMemo(() => {
  let result = [...items];
  if (searchQuery) result = result.filter(...);
  if (filterTag) result = result.filter(...);
  result.sort(...);
  return result;
}, [items, searchQuery, filterTag, sortBy]);
```

---

## Security Measures

### Authentication
- JWT in httpOnly cookies (XSS protection)
- SameSite=Lax (CSRF protection)
- 7-day expiration
- Bcrypt password hashing (cost=10)
- Email validation (regex)
- Password minimum 8 characters

### API Security
- Rate limiting on auth endpoints (10 req/min)
- Request body size limits (10MB)
- JWT algorithm validation (prevent alg:none)
- Rate limiter only trusts proxy-set `X-Real-IP` when `TRUST_PROXY_HEADERS=true`
- Card tag updates only accept tags from the same deck as the card
- Content-Disposition header sanitization
- All queries parameterized (SQL injection prevention)

### CORS
- Whitelist allowed origins
- AllowCredentials for cookies
- Configurable via CORS_ORIGINS env var

---

## UI/UX Design

### Style System (Apple-inspired)
- System font stack
- Rounded corners (12-20px)
- Subtle shadows
- Blur backdrop on header
- Smooth transitions

### Components
- **Buttons**: Pill-shaped, hover scale
- **Inputs**: Rounded, focus ring
- **Cards**: White background, shadow on hover
- **Tabs**: Segmented control style
- **Icons**: Consistent 32x32 icon buttons

### Responsive
- Mobile-first approach
- Flex wrap for button groups
- Stack layout on small screens

---

## Deployment

### Docker Images
- Multi-stage builds (builder → runtime)
- Alpine base images
- Platform: linux/amd64

### Production Stack
- Nginx reverse proxy (SSL termination)
- PostgreSQL container
- Backend serves API on :8080
- Frontend as static files

### Environment Variables
```dotenv
DATABASE_URL=postgres://...
JWT_SECRET=<random 32 bytes>
SECURE_COOKIES=true
TRUST_PROXY_HEADERS=true
CORS_ORIGINS=https://fsrs.ziyang.li
```

---

## Testing

```bash
# Backend unit tests
cd backend && go test ./internal/...

# Frontend unit tests
cd frontend && npm run test

# Frontend type check
cd frontend && npm run build

# Optional backend integration tests (requires local fsrs_test Postgres)
cd backend && go test -tags=integration ./internal/handler/...
```

---

## Future Enhancements

- [ ] Deck sharing (public decks)
- [ ] Multiple card types (cloze, image)
- [ ] Spaced repetition statistics graphs
- [ ] Mobile app (React Native)
- [ ] Offline support (PWA)
- [ ] AI-generated cards from text
