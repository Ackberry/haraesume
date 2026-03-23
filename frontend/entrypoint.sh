#!/bin/sh
set -e

# Extract DNS resolver from the container's resolv.conf for nginx.
# Prefer an IPv4 nameserver since nginx can't parse bare IPv6 addresses.
export DNS_RESOLVER=$(awk '/^nameserver/{print $2}' /etc/resolv.conf | grep -v ':' | head -1)
if [ -z "$DNS_RESOLVER" ]; then
  # No IPv4 nameserver found — wrap IPv6 in brackets for nginx.
  IPV6=$(awk '/^nameserver/{print $2; exit}' /etc/resolv.conf)
  if [ -n "$IPV6" ]; then
    DNS_RESOLVER="[$IPV6]"
  else
    DNS_RESOLVER="8.8.8.8"
  fi
fi

envsubst '${PORT} ${BACKEND_URL} ${DNS_RESOLVER}' \
  < /etc/nginx/templates/default.conf.template \
  > /etc/nginx/conf.d/default.conf

rm -f /etc/nginx/conf.d/default.conf.bak 2>/dev/null

exec nginx -g 'daemon off;'
