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

## Preferred Development Workflow

Use a separate git branch for code changes. The expected flow is:

1. Start from `main` and create a new branch.
2. Make changes on that branch.
3. Run the backend/frontend checks.
4. Commit the branch work.
5. Merge the verified branch into `main`.
6. Build Docker images and deploy to the droplet from the merged `main` state.

Example:

```bash
cd /Users/ziyangli/Coding/fsrs
git checkout main
git pull --ff-only
git checkout -b <change-branch>
```

## Production Notes

- `docker-compose.yml` assumes HTTPS termination and mounted certificates at `./certs/fullchain.pem` and `./certs/privkey.pem`.
- `docker-compose.prod.yml` is the droplet-targeted stack and expects Let's Encrypt certs at `/etc/letsencrypt/live/fsrs.ziyang.li/`.
- The backend now refuses to start in production unless `SECURE_COOKIES=true`.
