# FSRS

Spaced-repetition app with a Go backend, React/Vite frontend, and Docker deployment.

## Devbox Test Commands

The reliable way to run toolchains from this repo is to enter the repo's devbox environment first, or invoke commands through `devbox run` with a shell.

### Option 1: open a devbox shell

```bash
cd /Users/ziyangli/Coding/fsrs
devbox shell

cd backend
go test ./...

cd ../frontend
npm test
npm run lint
npm run build
```

### Option 2: run from the repo root without opening a shell

```bash
cd /Users/ziyangli/Coding/fsrs
devbox run bash -lc 'cd backend && go test ./...'
devbox run bash -lc 'cd frontend && npm test'
devbox run bash -lc 'cd frontend && npm run lint'
devbox run bash -lc 'cd frontend && npm run build'
```

### Optional integration tests

These require a PostgreSQL test database listening on `localhost:5432` and reachable as `fsrs_test`.

```bash
cd /Users/ziyangli/Coding/fsrs
devbox run bash -lc 'cd backend && go test -tags=integration ./internal/handler/...'
```

### Start a disposable PostgreSQL test instance with Docker

The integration tests expect:

- host: `localhost`
- port: `5432`
- user: `fsrs`
- password: `fsrs`
- database: `fsrs_test`

Start the test database:

```bash
docker run --name fsrs-test-postgres \
  -e POSTGRES_USER=fsrs \
  -e POSTGRES_PASSWORD=fsrs \
  -e POSTGRES_DB=fsrs_test \
  -p 5432:5432 \
  -d postgres:16
```

Wait until Postgres is ready:

```bash
docker exec fsrs-test-postgres pg_isready -U fsrs -d fsrs_test
```

Run the integration tests:

```bash
cd /Users/ziyangli/Coding/fsrs
devbox run bash -lc 'cd backend && go test -count=1 -tags=integration ./internal/handler/...'
```

If you prefer not to use the default connection settings, override them with `TEST_DATABASE_URL`:

```bash
cd /Users/ziyangli/Coding/fsrs
TEST_DATABASE_URL='postgres://fsrs:fsrs@localhost:5432/fsrs_test?sslmode=disable' \
  devbox run bash -lc 'cd backend && go test -count=1 -tags=integration ./internal/handler/...'
```

Stop and remove the disposable database when done:

```bash
docker rm -f fsrs-test-postgres
```

## Preferred Development Workflow

Use separate sibling clones for concurrent work. Do not use `git worktree` for agent or programmer isolation here because worktrees share the same repository refs and can race each other.

Recommended local layout:

- `/Users/ziyangli/Coding/fsrs`: canonical `main` checkout used for merge, release verification, and deployment
- `/Users/ziyangli/Coding/fsrs2`: isolated bug-fix development clone
- `/Users/ziyangli/Coding/fsrs3`: isolated new-feature development clone

If `fsrs2` or `fsrs3` are already busy, create another sibling clone with the next free `fsrs#` name instead of sharing an active checkout.

One-time setup for the extra clones:

```bash
cd /Users/ziyangli/Coding
git clone git@github.com:mvlnat/FSRS.git fsrs2
git clone git@github.com:mvlnat/FSRS.git fsrs3
```

Example for adding another isolated clone later:

```bash
cd /Users/ziyangli/Coding
git clone git@github.com:mvlnat/FSRS.git fsrs4
```

Expected flow:

1. Pick the correct isolated clone for the work: use `fsrs2` for bug fixes and `fsrs3` for new features. If those are already in use, create a new sibling clone such as `fsrs4`.
2. Update it from `origin/main` and create a new branch there.
3. Make changes and run the backend/frontend checks inside that clone only.
4. Commit the branch work in that clone.
5. Push the branch or otherwise make it available for review.
6. Merge the verified branch into `main` from the canonical `/Users/ziyangli/Coding/fsrs` clone.
7. Release from the verified `main` state in `/Users/ziyangli/Coding/fsrs`.
8. Build production Docker images locally only. Do not build on the droplet.
9. Transfer only the release artifacts needed by production, load the prebuilt images on the droplet, and restart the stack from the canonical `docker-compose.prod.yml`.
10. Clean up the droplet after deploy so it remains runtime-only instead of becoming a copy of the development workspace.

Example isolated branch flow:

```bash
cd /Users/ziyangli/Coding/fsrs2
git fetch origin
git switch main
git pull --ff-only
git switch -c <change-branch>
```

For new feature work, start in `fsrs3` instead:

```bash
cd /Users/ziyangli/Coding/fsrs3
git fetch origin
git switch main
git pull --ff-only
git switch -c <feature-branch>
```

Example merge flow from the canonical `main` clone:

```bash
cd /Users/ziyangli/Coding/fsrs
git fetch origin
git switch main
git pull --ff-only
git merge --ff-only origin/<change-branch>
```

If a branch has not been pushed yet, the canonical clone can fetch it directly from a sibling clone without sharing refs:

```bash
cd /Users/ziyangli/Coding/fsrs
git fetch ../fsrs2 <change-branch>
git merge --ff-only FETCH_HEAD
```

## Production Notes

- `docker-compose.yml` assumes HTTPS termination and mounted certificates at `./certs/fullchain.pem` and `./certs/privkey.pem`.
- `docker-compose.prod.yml` is the droplet-targeted stack and expects Let's Encrypt certs at `/etc/letsencrypt/live/fsrs.ziyang.li/`.
- The backend now refuses to start in production unless `SECURE_COOKIES=true`.
- The droplet is a runtime host, not a build host. Keep builds local and publish prebuilt Docker images to the droplet.
- After a successful deploy, `/root/fsrs` should normally contain only `.env`, `docker-compose.prod.yml`, and `nginx.conf`. Source trees, `node_modules`, temporary tar archives, local cert copies, and other dev/release leftovers should be removed.
