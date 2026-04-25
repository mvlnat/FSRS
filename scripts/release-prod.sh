#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/release-prod.sh [options]

Commits the current work (if needed), pushes the branch, fast-forward merges it
into main, runs release checks, builds amd64 Docker images locally, uploads the
images and production config to the VM, restarts the production stack, and
verifies the deploy.

Options:
  -m, --message MSG   Commit message to use when the working tree is dirty
  --host HOST         Production SSH host (default: root@5.78.201.47)
  --app-dir DIR       Remote app directory (default: /root/fsrs)
  --domain DOMAIN     Domain to verify after deploy (default: fsrs.ziyang.li)
  --skip-tests        Skip backend/frontend checks before build
  --skip-verify       Skip post-deploy verification checks
  -h, --help          Show this help text

Shared edge:
  This repo owns the VM's host 80/443 nginx container. Its nginx.conf must stay
  as the combined shared-edge config for fsrs.ziyang.li, store.ziyang.li, and
  random.ziyang.li. Do not replace it with an FSRS-only config during deploy.
EOF
}

log() {
  printf '[release] %s\n' "$*"
}

die() {
  printf '[release] ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

run() {
  log "$*"
  "$@"
}

remote_run() {
  local script=$1

  log "ssh $PROD_HOST <remote script>"
  ssh "$PROD_HOST" "bash -lc $(printf '%q' "$script")"
}

COMMIT_MESSAGE=""
PROD_HOST="root@5.78.201.47"
APP_DIR="/root/fsrs"
DOMAIN="fsrs.ziyang.li"
SKIP_TESTS=0
SKIP_VERIFY=0

while (($# > 0)); do
  case "$1" in
    -m|--message)
      (($# >= 2)) || die "missing value for $1"
      COMMIT_MESSAGE=$2
      shift 2
      ;;
    --host)
      (($# >= 2)) || die "missing value for $1"
      PROD_HOST=$2
      shift 2
      ;;
    --app-dir)
      (($# >= 2)) || die "missing value for $1"
      APP_DIR=$2
      shift 2
      ;;
    --domain)
      (($# >= 2)) || die "missing value for $1"
      DOMAIN=$2
      shift 2
      ;;
    --skip-tests)
      SKIP_TESTS=1
      shift
      ;;
    --skip-verify)
      SKIP_VERIFY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

need_cmd git
need_cmd docker
need_cmd ssh
need_cmd scp
need_cmd curl
need_cmd devbox

REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || die "run this script inside the git repository"
cd "$REPO_ROOT"

ORIGINAL_BRANCH=$(git branch --show-current)
[[ -n "$ORIGINAL_BRANCH" ]] || die "detached HEAD is not supported"

TMP_DIR=$(mktemp -d /tmp/fsrs-release.XXXXXX)

cleanup() {
  rm -rf "$TMP_DIR"
}

trap cleanup EXIT

if [[ -n "$(git status --porcelain)" ]]; then
  [[ -n "$COMMIT_MESSAGE" ]] || die "working tree is dirty; pass --message to create the release commit"

  log "staging current changes on $ORIGINAL_BRANCH"
  run git add -A

  if [[ -n "$(git diff --cached --name-only)" ]]; then
    run git commit -m "$COMMIT_MESSAGE"
  else
    log "nothing was staged after git add -A; skipping commit"
  fi
else
  log "working tree is clean; skipping commit"
fi

if [[ "$ORIGINAL_BRANCH" != "main" ]]; then
  run git push -u origin "$ORIGINAL_BRANCH"
fi

run git fetch origin
run git switch main
run git pull --ff-only origin main

if [[ "$ORIGINAL_BRANCH" != "main" ]]; then
  run git merge --ff-only "$ORIGINAL_BRANCH"
fi

if ((SKIP_TESTS == 0)); then
  run devbox run bash -lc "cd backend && go test ./..."
  run devbox run bash -lc "cd frontend && npm test"
  run devbox run bash -lc "cd frontend && npm run lint"
  run devbox run bash -lc "cd frontend && npm run build"
else
  log "skipping backend/frontend checks"
fi

run git push origin main

SHORT_SHA=$(git rev-parse --short HEAD)
BACKEND_IMAGE="fsrs-backend:latest"
FRONTEND_IMAGE="fsrs-frontend:latest"
BACKEND_ARCHIVE="$TMP_DIR/fsrs-backend-${SHORT_SHA}.tar.gz"
FRONTEND_ARCHIVE="$TMP_DIR/fsrs-frontend-${SHORT_SHA}.tar.gz"
REMOTE_BACKEND_ARCHIVE="$APP_DIR/$(basename "$BACKEND_ARCHIVE")"
REMOTE_FRONTEND_ARCHIVE="$APP_DIR/$(basename "$FRONTEND_ARCHIVE")"

run docker build --platform linux/amd64 -t "$BACKEND_IMAGE" -t "fsrs-backend:${SHORT_SHA}" -f backend/Dockerfile ./backend
run docker build --platform linux/amd64 -t "$FRONTEND_IMAGE" -t "fsrs-frontend:${SHORT_SHA}" -f frontend/Dockerfile ./frontend

log "saving release images to $TMP_DIR"
docker save "$BACKEND_IMAGE" | gzip > "$BACKEND_ARCHIVE"
docker save "$FRONTEND_IMAGE" | gzip > "$FRONTEND_ARCHIVE"

remote_run "
set -euo pipefail
mkdir -p $(printf '%q' "$APP_DIR")
test -f $(printf '%q' "$APP_DIR/.env")
"

run scp \
  docker-compose.prod.yml \
  nginx.conf \
  "$BACKEND_ARCHIVE" \
  "$FRONTEND_ARCHIVE" \
  "$PROD_HOST:$APP_DIR/"

remote_run "
set -euo pipefail
gunzip -c $(printf '%q' "$REMOTE_BACKEND_ARCHIVE") | docker load
gunzip -c $(printf '%q' "$REMOTE_FRONTEND_ARCHIVE") | docker load
cd $(printf '%q' "$APP_DIR")
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d
rm -f $(printf '%q' "$REMOTE_BACKEND_ARCHIVE") $(printf '%q' "$REMOTE_FRONTEND_ARCHIVE")
docker image prune -f
"

if ((SKIP_VERIFY == 0)); then
  remote_run "
set -euo pipefail
cd $(printf '%q' "$APP_DIR")
docker compose -f docker-compose.prod.yml ps
docker ps --format '{{.Names}} {{.Image}} {{.Status}}' --filter name=fsrs-
"

  HTTP_STATUS=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "https://${DOMAIN}/")
  case "$HTTP_STATUS" in
    2*|3*)
      log "verified https://${DOMAIN}/ returned HTTP ${HTTP_STATUS}"
      ;;
    *)
      die "deployment verification failed: https://${DOMAIN}/ returned HTTP ${HTTP_STATUS}"
      ;;
  esac
else
  log "skipping post-deploy verification"
fi

log "release complete"
log "deployed commit: $(git rev-parse HEAD)"
