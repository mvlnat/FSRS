#!/bin/sh
set -eu

cd /root/fsrs
docker compose -f docker-compose.prod.yml up -d nginx
