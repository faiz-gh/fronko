#!/bin/sh
# Writes the browser's runtime settings, so one image works with any backend URL.
# API_URL empty (the default) keeps API calls same-origin via this container's proxy.
set -e

escaped=$(printf '%s' "${API_URL:-}" | sed 's/[\\"]/\\&/g')
printf 'window.__FRONKO_CONFIG__ = { apiUrl: "%s" };\n' "$escaped" > /usr/share/nginx/html/config.js
