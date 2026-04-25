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
7. Build production Docker images locally only. Do not build on the production VM.
8. Transfer only the release artifacts needed by production, load the prebuilt images on the VM, and restart the stack from `docker-compose.prod.yml`.
9. Clean up the production VM after deploy so it remains runtime-only instead of becoming a copy of the development workspace.

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

### One-command Release Script

The repo now includes [`scripts/release-prod.sh`](./scripts/release-prod.sh) to run the end-to-end release flow:

- commit current changes if the worktree is dirty
- push the current branch
- fast-forward merge into `main`
- run backend/frontend checks
- push `main`
- build amd64 Docker images locally
- upload image archives plus `docker-compose.prod.yml` and `nginx.conf`
- restart the production stack on `root@5.78.201.47`
- verify the remote containers and the live site

Example:

```bash
cd /Users/ziyangli/Coding/fsrs
scripts/release-prod.sh --message "Add email verification and password reset"
```

Useful flags:

- `--skip-tests` to skip backend/frontend checks
- `--skip-verify` to skip the remote/site verification step
- `--host`, `--app-dir`, and `--domain` to override the production defaults

## Production Notes

- FSRS owns the VM's public nginx edge because its `nginx` service binds host ports `80` and `443`.
- That edge must stay a shared config for `fsrs.ziyang.li`, `store.ziyang.li`, and `random.ziyang.li`. Store and RandomPick do not bind public ports; their containers join the external Docker network `shared-edge` as `store-edge` and `randompick-edge`.
- Do not deploy an FSRS-only `nginx.conf`. A future FSRS release uploads `nginx.conf` to `/root/fsrs`, so removing the store/randompick server blocks will break those sites.
- `docker-compose.yml` assumes HTTPS termination and mounted certificates at `./certs/fullchain.pem` and `./certs/privkey.pem`.
- `docker-compose.prod.yml` is the production-targeted stack and mounts `/etc/letsencrypt` read-only so the shared edge can serve each hostname's certificate.
- The backend now refuses to start in production unless `SECURE_COOKIES=true`.
- The production VM is a runtime host, not a build host. Keep builds local and publish prebuilt Docker images to the VM.
- After a successful deploy, `/root/fsrs` should normally contain only `.env`, `docker-compose.prod.yml`, and `nginx.conf`. Source trees, `node_modules`, temporary tar archives, local cert copies, and other dev/release leftovers should be removed.
- Current production host: `root@5.78.201.47`

## Production Bootstrap

Use this once on a fresh production VM before the normal image upload/restart flow:

```bash
PROD_HOST=root@5.78.201.47

ssh "$PROD_HOST" "apt-get update && apt-get install -y docker.io docker-compose-v2 certbot"
ssh "$PROD_HOST" "certbot certonly --standalone --non-interactive --agree-tos --register-unsafely-without-email -d fsrs.ziyang.li"
ssh "$PROD_HOST" "docker network create shared-edge >/dev/null 2>&1 || true"
ssh "$PROD_HOST" "mkdir -p /root/fsrs && chmod 700 /root/fsrs"
scp docker-compose.prod.yml nginx.conf "$PROD_HOST":/root/fsrs/
scp /path/to/prod.env "$PROD_HOST":/root/fsrs/.env
ssh "$PROD_HOST" "chmod 600 /root/fsrs/.env"
```

Because the production certificate uses Certbot's `standalone` mode, renewals must temporarily free host port `80`. Install renewal hooks on the VM so Certbot stops `fsrs-nginx` before renewal and restarts it afterward:

```bash
ssh "$PROD_HOST" "mkdir -p /etc/letsencrypt/renewal-hooks/pre /etc/letsencrypt/renewal-hooks/post"
scp ./scripts/letsencrypt-stop-fsrs-nginx.sh "$PROD_HOST":/etc/letsencrypt/renewal-hooks/pre/stop-fsrs-nginx.sh
scp ./scripts/letsencrypt-start-fsrs-nginx.sh "$PROD_HOST":/etc/letsencrypt/renewal-hooks/post/start-fsrs-nginx.sh
ssh "$PROD_HOST" "chmod 755 /etc/letsencrypt/renewal-hooks/pre/stop-fsrs-nginx.sh /etc/letsencrypt/renewal-hooks/post/start-fsrs-nginx.sh"
ssh "$PROD_HOST" "certbot renew --dry-run --no-random-sleep-on-renew"
```
