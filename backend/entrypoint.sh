#!/bin/sh
set -e

if [ -n "$GOOGLE_APPLICATION_CREDENTIALS_JSON" ]; then
  mkdir -p /app/secrets
  printf '%s' "$GOOGLE_APPLICATION_CREDENTIALS_JSON" > /app/secrets/service-account.json
  export GOOGLE_APPLICATION_CREDENTIALS=/app/secrets/service-account.json
  echo "entrypoint: wrote credentials to $GOOGLE_APPLICATION_CREDENTIALS ($(wc -c < /app/secrets/service-account.json) bytes)"
else
  echo "entrypoint: GOOGLE_APPLICATION_CREDENTIALS_JSON is NOT set"
fi

exec "$@"
