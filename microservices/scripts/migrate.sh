#!/bin/sh
# Applies each microservice's migrations against the shared MySQL instance.
#
# Each service owns its own tables and its own migration version table
# (x-migrations-table), so they do not clash even though they share one DB.
# Order matters: transaction-service has foreign keys into wallets/transactions.
set -e

DB_DSN_BASE="${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:${DB_PORT})/${DB_NAME}"

run_migrations() {
	svc="$1"
	echo "==> applying ${svc} migrations"
	migrate -path "/migrations/${svc}/db/migrations" \
		-database "mysql://${DB_DSN_BASE}?x-migrations-table=${svc}_migrations" \
		up
}

# Dependency order: users -> wallets -> transactions/ledger -> refresh tokens
run_migrations user-service
run_migrations wallet-service
run_migrations transaction-service
run_migrations auth-service

echo "==> all migrations applied"
