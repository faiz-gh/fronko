#!/bin/sh
set -e

if [ -z "$DATABASE_URL" ]; then
    echo "DATABASE_URL is not set" >&2
    exit 1
fi

echo "Running database migrations..."
migrate -path /app/migrations -database "$DATABASE_URL" up

echo "Starting backend..."
exec "$@"
