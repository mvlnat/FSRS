# FSRS Project - Development Guidelines

## Workflow Reminders

1. **Always git commit** - Commit changes after completing features or fixes
2. **Always run unit tests** - Run tests before committing
3. **Always build and push** - After tests pass, build Docker images and deploy to droplet

## Commands

### Run Tests
```bash
eval "$(devbox shellenv)" && cd backend && go test ./internal/...
```

### Build Docker Images (for amd64 server)
```bash
docker build --platform linux/amd64 -t fsrs-backend -f backend/Dockerfile ./backend
docker build --platform linux/amd64 -t fsrs-frontend -f frontend/Dockerfile ./frontend
```

### Deploy to Droplet
```bash
# Save images
docker save fsrs-backend | gzip > /tmp/fsrs-backend.tar.gz
docker save fsrs-frontend | gzip > /tmp/fsrs-frontend.tar.gz

# Upload to server
scp /tmp/fsrs-backend.tar.gz /tmp/fsrs-frontend.tar.gz root@161.35.3.230:/root/

# Load and restart on server
ssh root@161.35.3.230 "gunzip -c /root/fsrs-backend.tar.gz | docker load && gunzip -c /root/fsrs-frontend.tar.gz | docker load && cd /root/fsrs && docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d"
```

## Server Info

- **Droplet IP**: 161.35.3.230
- **Domain**: fsrs.ziyang.li
- **Low memory (458MB)**: Build images locally, not on server
