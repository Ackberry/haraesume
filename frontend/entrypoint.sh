#!/bin/sh
set -e

envsubst '${PORT} ${BACKEND_URL}' \
  < /etc/nginx/templates/default.conf.template \
  > /etc/nginx/conf.d/default.conf

rm -f /etc/nginx/conf.d/default.conf.bak 2>/dev/null

exec nginx -g 'daemon off;'
