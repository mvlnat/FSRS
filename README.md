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

Use a normal branch-based workflow in `/Users/ziyangli/Coding/fsrs`. Keep `main` clean for release verification and deployment, and do day-to-day work on short-lived branches created from the latest `origin/main`.

Expected flow:

1. Update `/Users/ziyangli/Coding/fsrs` from `origin/main` and create a new branch there.
2. Make changes and run the backend/frontend checks in this repo.
3. Commit the branch work.
4. Push the branch or otherwise make it available for review.
5. Merge the verified branch into `main`.
6. Release from the verified `main` state in `/Users/ziyangli/Coding/fsrs`.
7. Build production Docker images locally only. Do not build on the droplet.
8. Transfer only the release artifacts needed by production, load the prebuilt images on the droplet, and restart the stack from `docker-compose.prod.yml`.
9. Clean up the droplet after deploy so it remains runtime-only instead of becoming a copy of the development workspace.

Example branch flow:

```bash
cd /Users/ziyangli/Coding/fsrs
git fetch origin
git switch main
git pull --ff-only
git switch -c <change-branch>
```

Example merge flow once the branch is reviewed:

```bash
cd /Users/ziyangli/Coding/fsrs
git fetch origin
git switch main
git pull --ff-only
git merge --ff-only <change-branch>
```

## Production Notes

- `docker-compose.yml` assumes HTTPS termination and mounted certificates at `./certs/fullchain.pem` and `./certs/privkey.pem`.
- `docker-compose.prod.yml` is the droplet-targeted stack and expects Let's Encrypt certs at `/etc/letsencrypt/live/fsrs.ziyang.li/`.
- The backend now refuses to start in production unless `SECURE_COOKIES=true`.
- The droplet is a runtime host, not a build host. Keep builds local and publish prebuilt Docker images to the droplet.
- After a successful deploy, `/root/fsrs` should normally contain only `.env`, `docker-compose.prod.yml`, and `nginx.conf`. Source trees, `node_modules`, temporary tar archives, local cert copies, and other dev/release leftovers should be removed.
