#!/bin/sh
# Applies each microservice's migrations against its own database.
#
# One database per service:
#   user-service        -> sudowallet_user
#   wallet-service      -> sudowallet_wallet
#   transaction-service -> sudowallet_transaction
#   auth-service        -> sudowallet_auth
#
# The databases are created by scripts/init-databases.sql on first MySQL boot.
# payment-service owns no tables and is skipped here.
set -e

DB_DSN_BASE="${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:${DB_PORT})"

run_migrations() {
	svc="$1"
	db_name="$2"
	echo "==> applying ${svc} migrations to ${db_name}"
	migrate -path "/migrations/${svc}/db/migrations" \
		-database "mysql://${DB_DSN_BASE}/${db_name}?x-migrations-table=${svc}_migrations" \
		up
}

# Dependency order: users -> wallets -> transactions/ledger -> refresh tokens
run_migrations user-service sudowallet_user
run_migrations wallet-service sudowallet_wallet
run_migrations transaction-service sudowallet_transaction
run_migrations auth-service sudowallet_auth

echo "==> all migrations applied"