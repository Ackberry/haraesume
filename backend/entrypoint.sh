#!/bin/sh
set -e

if [ -n "$GOOGLE_APPLICATION_CREDENTIALS_JSON" ]; then
  mkdir -p /app/secrets
  printf '%s' "$GOOGLE_APPLICATION_CREDENTIALS_JSON" > /app/secrets/service-account.json
  export GOOGLE_APPLICATION_CREDENTIALS=/app/secrets/service-account.json
fi

exec "$@"
