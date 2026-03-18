#!/bin/sh
set -e

DB_HOST="${DB_HOST:-10.0.10.114:5432}"
DB_USER="${DB_USER}"
DB_PASSWORD="${DB_PASSWORD}"

for service in auth subscription checker scheduler notify settings; do
    echo "==> Migrating: $service"
    migrate \
        -path "/services/${service}/migrations" \
        -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}/${service}?sslmode=disable" \
        up
done

echo "==> All migrations complete"
