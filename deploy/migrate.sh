#!/bin/sh
set -e

# Install golang-migrate if not present
if ! command -v migrate > /dev/null 2>&1; then
    echo "Installing golang-migrate..."
    wget -qO- https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz \
        | tar xz -C /usr/local/bin migrate
fi

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
