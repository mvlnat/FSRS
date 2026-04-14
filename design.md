making a web app for spaced repetition learning

- frontend: typescript react
- backend: golang
- database: postgres
- hosting: vm

## MVP Features

- User authentication (signup/login)
- Create/edit/delete flashcard decks
- Create/edit/delete cards within decks
- Study session with FSRS algorithm (using go-fsrs library)
- Track review history and progress

## Project Structure

```
fsrs/
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Auth/
│   │   │   ├── Deck/
│   │   │   ├── Card/
│   │   │   └── Study/
│   │   ├── pages/
│   │   │   ├── Home.tsx
│   │   │   ├── Login.tsx
│   │   │   ├── Register.tsx
│   │   │   ├── Decks.tsx
│   │   │   ├── DeckEdit.tsx
│   │   │   └── Study.tsx
│   │   ├── hooks/
│   │   ├── api/
│   │   ├── types/
│   │   └── App.tsx
│   ├── package.json
│   └── vite.config.ts
│
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── auth.go
│   │   │   ├── auth_test.go
│   │   │   ├── deck.go
│   │   │   ├── card.go
│   │   │   ├── study.go
│   │   │   ├── security_test.go
│   │   │   └── integration_test.go
│   │   ├── model/
│   │   │   ├── user.go
│   │   │   ├── deck.go
│   │   │   ├── card.go
│   │   │   └── review.go
│   │   ├── repository/
│   │   ├── service/
│   │   │   ├── fsrs.go
│   │   │   └── fsrs_test.go
│   │   └── middleware/
│   │       ├── auth.go
│   │       ├── ratelimit.go
│   │       └── ratelimit_test.go
│   ├── migrations/
│   ├── go.mod
│   └── go.sum
│
├── docker-compose.yml
├── Dockerfile.frontend
├── Dockerfile.backend
├── nginx.conf
└── .env.example
```

## Database Schema

```sql
-- users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- decks
CREATE TABLE decks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_decks_user_id ON decks(user_id);

-- cards
CREATE TABLE cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_cards_deck_id ON cards(deck_id);

-- card_states (FSRS scheduling data)
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

-- reviews (history log)
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 4),
    reviewed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_reviews_card_id ON reviews(card_id);
```

## API Endpoints

```
Auth (JWT in httpOnly cookie)
------------------------------
POST   /api/auth/register
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me

Decks
------------------------------
GET    /api/decks
POST   /api/decks
POST   /api/decks/import         (import deck from JSON)
GET    /api/decks/:id
PUT    /api/decks/:id
DELETE /api/decks/:id
GET    /api/decks/:id/stats      (cards total, due today, new)
GET    /api/decks/:id/export     (export deck as JSON)

Cards
------------------------------
GET    /api/decks/:id/cards
POST   /api/decks/:id/cards
GET    /api/cards/:id
PUT    /api/cards/:id
DELETE /api/cards/:id

Study
------------------------------
GET    /api/study/stats            (user study statistics)
GET    /api/study/:deckId          (get due cards)
POST   /api/study/:cardId/review   (submit rating 1-4)
```

## Testing

```bash
# Run unit tests
cd backend && go test ./internal/...

# Run integration tests (requires test DB)
TEST_DATABASE_URL=postgres://... go test -tags=integration ./internal/handler/...

# Run frontend type check
cd frontend && npm run build
```

## Auth Strategy

- JWT stored in httpOnly cookie (sameSite=lax for localhost compatibility)
- Token expires in 7 days
- Middleware extracts user from token, rejects if invalid/expired
- Password hashed with bcrypt (cost=10)

## VM Deployment

**Stack**
- Docker Compose for orchestration
- Nginx reverse proxy (SSL termination)
- PostgreSQL container
- Backend serves API on :8080
- Frontend built as static files served by Nginx

**Config**
- Environment variables via .env file
- Secrets: DB password, JWT secret
- Let's Encrypt for SSL via certbot
- `nslookup fsrs.ziyang.li`
- `sudo certbot --nginx -d fsrs.ziyang.li`

**Operations**
- Systemd service for auto-restart
- Daily pg_dump backup to object storage
- Nginx access logs for monitoring

## Production Polish

**UI/UX**
- Keyboard shortcuts (spacebar to flip, 1-4 for ratings)
- Mobile responsive design
- Progress bar during study sessions

**Features**
- Import/export decks (JSON)
- Study statistics (retention rate, cards reviewed)

**Security**
- Rate limiting on auth endpoints (10 req/min per IP)
- Input sanitization (React auto-escapes XSS)
- Parameterized SQL queries (prevents SQL injection)
- Password hashing with bcrypt
- JWT in httpOnly cookies (prevents XSS token theft)
- CORS restricted to allowed origins
- All sensitive routes require authentication middleware

## Security Checklist

**Secrets Management**
- [ ] Generate strong JWT_SECRET: `openssl rand -base64 32`
- [ ] Generate strong DB password: `openssl rand -base64 32`
- [ ] Never commit .env to git (already in .gitignore)
- [ ] Rotate secrets periodically

**Database Security**
- [ ] Use SSL for DB connections: `sslmode=require`
- [ ] Create dedicated DB user with minimal privileges
- [ ] Regular backups with `pg_dump`
- [ ] Store backups encrypted

**VM/Server Security**
- [ ] SSH key-only authentication (disable password login)
- [ ] Firewall: only allow 22 (SSH), 80, 443
- [ ] Keep system packages updated
- [ ] Use fail2ban for SSH protection

**API Security**
- [x] Rate limiting on auth endpoints
- [x] JWT expiration (7 days for refresh)
- [x] httpOnly cookies (XSS protection)
- [x] CORS whitelist
- [x] Input validation on all endpoints
- [x] Authorization checks on all protected resources
