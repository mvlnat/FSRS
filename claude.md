# FSRS Project - Development Guidelines

## Workflow Reminders

1. **Start in a worktree and branch** - For code changes, create a dedicated git worktree and branch from `main`
2. **Develop off `main`** - Make and verify changes in that worktree/branch instead of editing directly on `main`
3. **Try to add tests for new work** - Add or update tests for new development and behavior changes whenever practical
4. **Commit branch work** - Commit changes on the branch once the change and tests are in good shape
5. **Always run checks before merge** - Run backend/frontend tests and builds before moving a change forward
6. **Review before merge** - Do code review before moving branch work into `main`
7. **Merge before deploy** - Once the reviewed branch looks good, merge it into `main`
8. **Always build and push from `main`** - After the merged `main` state passes checks, build Docker images and deploy to droplet

## Preferred Git Flow

1. Create a fresh worktree and branch from `main`
```bash
git checkout main
git pull --ff-only
git worktree add ../fsrs-<change-branch> -b <change-branch> main
```

2. Make changes and verify them inside that worktree

3. Try to add or update tests for the new work before finishing the branch

4. Commit the branch work
```bash
git add <files>
git commit -m "<message>"
```

5. Review the branch and fix findings

6. Merge back into `main` once the change looks good
```bash
git checkout main
git merge --ff-only <change-branch>
```

7. Build and deploy from `main`

## Commands

### Run Tests
```bash
devbox run bash -lc 'cd backend && go test ./...'
devbox run bash -lc 'cd frontend && npm test'
devbox run bash -lc 'cd frontend && npm run lint'
devbox run bash -lc 'cd frontend && npm run build'
```

### Build Docker Images (for amd64 server)
```bash
git checkout main
docker build --platform linux/amd64 -t fsrs-backend -f backend/Dockerfile ./backend
docker build --platform linux/amd64 -t fsrs-frontend -f frontend/Dockerfile ./frontend
```

### Deploy to Droplet (Full Steps)

**Step 1: Build for amd64**
```bash
git checkout main
docker build --platform linux/amd64 -t fsrs-backend -f backend/Dockerfile ./backend
docker build --platform linux/amd64 -t fsrs-frontend -f frontend/Dockerfile ./frontend
```

**Step 2: Save and upload**
```bash
docker save fsrs-backend:latest | gzip > /tmp/fsrs-backend.tar.gz
docker save fsrs-frontend:latest | gzip > /tmp/fsrs-frontend.tar.gz
scp /tmp/fsrs-backend.tar.gz /tmp/fsrs-frontend.tar.gz root@161.35.3.230:/root/
```

**Step 3: Load and restart on server**
```bash
ssh root@161.35.3.230 "gunzip -c /root/fsrs-backend.tar.gz | docker load && gunzip -c /root/fsrs-frontend.tar.gz | docker load && cd /root/fsrs && docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d"
```

**Step 4: Verify deployment**
```bash
# Check containers are running with correct images
ssh root@161.35.3.230 "docker ps && docker inspect --format='{{.Name}}: {{.Image}}' fsrs-backend-1 fsrs-frontend-1"

# Clean up old images
ssh root@161.35.3.230 "docker image prune -f"

# Test site responds
curl -s -o /dev/null -w "%{http_code}" https://fsrs.ziyang.li/
```

**One-liner (after building)**
```bash
docker save fsrs-backend:latest | gzip > /tmp/fsrs-backend.tar.gz && docker save fsrs-frontend:latest | gzip > /tmp/fsrs-frontend.tar.gz && scp /tmp/fsrs-backend.tar.gz /tmp/fsrs-frontend.tar.gz root@161.35.3.230:/root/ && ssh root@161.35.3.230 "gunzip -c /root/fsrs-backend.tar.gz | docker load && gunzip -c /root/fsrs-frontend.tar.gz | docker load && cd /root/fsrs && docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d && docker image prune -f"
```

## Server Info

- **Droplet IP**: 161.35.3.230
- **Domain**: fsrs.ziyang.li
- **Low memory (458MB)**: Build images locally, not on server

---

## Code Review Lessons Learned

### Security Fixes Applied

1. **JWT Signing Error Handling** (`handler/auth.go`)
   - Always check error return from `token.SignedString()`
   - Bad: `tokenString, _ := token.SignedString(key)`
   - Good: `tokenString, err := token.SignedString(key); if err != nil { return err }`

2. **JWT Algorithm Validation** (`middleware/auth.go`)
   - Validate signing method to prevent algorithm confusion attacks
   ```go
   if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
       return nil, fmt.Errorf("unexpected signing method")
   }
   ```

3. **XSS Prevention** (`Study.tsx`)
   - Never use `dangerouslySetInnerHTML` with user content
   - Parse markdown into React elements safely instead
   - Use `<code>`, `<strong>`, `<em>` components

4. **X-Forwarded-For Spoofing** (`middleware/ratelimit.go`)
   - Don't trust full X-Forwarded-For header
   - Only use first IP or fall back to RemoteAddr
   ```go
   if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
       ip = strings.TrimSpace(strings.Split(xff, ",")[0])
   }
   ```

5. **Request Body Size Limits** (`handler/deck.go`)
   - Limit request body size to prevent DoS
   ```go
   r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
   ```

6. **Content-Disposition Header Injection** (`handler/deck.go`)
   - Sanitize filenames in Content-Disposition headers
   - Remove quotes, newlines, control characters

7. **Email Validation** (`handler/auth.go`)
   - Use regex to validate email format
   - Enforce minimum password length (8 chars)

### Performance Fixes Applied

1. **N+1 Query Problem** (`repository/deck.go`)
   - Batch fetch stats with single query using LEFT JOIN and GROUP BY
   - Created `ListByUserWithStats()` instead of calling `GetStats()` per deck

2. **Atomic Operations** (`repository/deck.go`)
   - Use transactions for multi-step operations
   - `ImportDeckWithCards()` wraps deck + cards creation in transaction

3. **Batch Tag Fetching** (`repository/tag.go`)
   - `GetTagsForCards()` fetches tags for multiple cards in one query
   - Uses `WHERE card_id = ANY($1)` with array parameter

### React Patterns

1. **Shared Auth State with Context**
   - Problem: Each `useAuth()` call created independent state
   - Solution: Create `AuthProvider` with `AuthContext`
   - Wrap app in provider, all components share same state
   - File must be `.tsx` if it contains JSX

2. **useMemo for Filtering/Sorting**
   ```tsx
   const filteredCards = useMemo(() => {
     let result = [...cards];
     if (searchQuery) { /* filter */ }
     if (filterTagId) { /* filter */ }
     result.sort(/* sort */);
     return result;
   }, [cards, searchQuery, sortBy, filterTagId]);
   ```

3. **useCallback for Event Handlers**
   - Wrap handlers that are dependencies of useEffect
   - Include all used variables in dependency array

### CSS/UI Patterns

1. **Centering Elements**
   - For flex children: use `align-self: center`
   - For block elements: wrap in container with `text-align: center`
   - Safest: wrap in div with explicit centering

2. **Consistent Icon Buttons**
   - Use same class for all icon buttons (buttons and anchors)
   ```css
   .btn-icon, a.btn-icon {
     width: 32px;
     height: 32px;
     display: inline-flex;
     align-items: center;
     justify-content: center;
   }
   ```

3. **Select Dropdowns**
   - Use `appearance: none` and custom background-image for arrow
   - Match padding and border-radius with other inputs

---

## Common Gotchas

1. **Docker restart vs down/up**
   - `restart` may not pick up new images
   - Use `down && up -d` for full image refresh

2. **Browser Caching**
   - Hard refresh (Cmd+Shift+R) after deploys
   - Vite adds hashes to filenames, but browser may cache HTML

3. **TypeScript JSX Files**
   - Files with JSX must have `.tsx` extension
   - Import statements don't need extension

4. **PostgreSQL Arrays**
   - Use `ANY($1)` with `[]uuid.UUID` slice for IN queries
   - pgx handles array parameters automatically

5. **Cookie Auth + CORS**
   - Need `credentials: 'include'` on fetch
   - Need `AllowCredentials: true` in CORS config
   - Cookie `SameSite=Lax` works for same-site navigation
