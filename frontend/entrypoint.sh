#!/bin/sh
set -e

# Extract DNS resolver from the container's resolv.conf for nginx.
export DNS_RESOLVER=$(awk '/^nameserver/{print $2; exit}' /etc/resolv.conf)
if [ -z "$DNS_RESOLVER" ]; then
  DNS_RESOLVER="8.8.8.8"
fi

envsubst '${PORT} ${BACKEND_URL} ${DNS_RESOLVER}' \
  < /etc/nginx/templates/default.conf.template \
  > /etc/nginx/conf.d/default.conf

rm -f /etc/nginx/conf.d/default.conf.bak 2>/dev/null

exec nginx -g 'daemon off;'
