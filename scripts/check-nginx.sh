#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v nginx >/dev/null 2>&1; then
  echo "nginx is not available in the current environment" >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is not available in the current environment" >&2
  exit 1
fi

nginx_root="$(cd "$(dirname "$(command -v nginx)")/.." && pwd)"
mime_types_path="$nginx_root/conf/mime.types"

if [[ ! -f "$mime_types_path" ]]; then
  echo "could not locate nginx mime.types at $mime_types_path" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
mkdir -p "$tmpdir/logs"

cert_path="$tmpdir/fullchain.pem"
key_path="$tmpdir/privkey.pem"

openssl req \
  -x509 \
  -nodes \
  -newkey rsa:2048 \
  -keyout "$key_path" \
  -out "$cert_path" \
  -days 1 \
  -subj "/CN=localhost" >/dev/null 2>&1

main_conf="$tmpdir/nginx.conf"
sed \
  -e "s|include /etc/nginx/mime.types;|include $mime_types_path;|" \
  -e "s|access_log /var/log/nginx/access.log;|access_log $tmpdir/access.log;|" \
  -e "s|error_log /var/log/nginx/error.log;|error_log $tmpdir/error.log;|" \
  -e "s|listen 80 default_server;|listen 8080 default_server;|" \
  -e "s|listen 80;|listen 8080;|" \
  -e "s|listen 443 ssl default_server;|listen 8443 ssl default_server;|" \
  -e "s|listen 443 ssl;|listen 8443 ssl;|" \
  -e "s|server backend:8080;|server 127.0.0.1:8080;|" \
  -e "s|server frontend:80;|server 127.0.0.1:8081;|" \
  -e "s|/etc/nginx/certs/fullchain.pem|$cert_path|" \
  -e "s|/etc/nginx/certs/privkey.pem|$key_path|" \
  "$repo_root/nginx.conf" >"$main_conf"

frontend_conf="$tmpdir/frontend.nginx.conf"
sed \
  -e "s|listen 80;|listen 8081;|" \
  "$repo_root/frontend/nginx.conf" >"$frontend_conf"

frontend_wrapper="$tmpdir/frontend.wrapper.conf"
cat >"$frontend_wrapper" <<EOF
events {
    worker_connections 1024;
}

http {
    include $mime_types_path;
    default_type application/octet-stream;
    access_log off;
    error_log stderr;
    include $frontend_conf;
}
EOF

echo "Validating $repo_root/nginx.conf"
nginx -t -e stderr -p "$tmpdir" -c "$main_conf" -g "pid $tmpdir/nginx.pid;"

echo "Validating $repo_root/frontend/nginx.conf"
nginx -t -e stderr -p "$tmpdir" -c "$frontend_wrapper" -g "pid $tmpdir/frontend.nginx.pid;"

echo "Nginx config validation passed"
