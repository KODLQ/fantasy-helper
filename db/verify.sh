#!/bin/sh
set -eu

: "${DATABASE_URL:?Set DATABASE_URL before verifying migrations}"

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
MIGRATIONS_DIR="${MIGRATIONS_DIR:-${SCRIPT_DIR}/migrations}"
MIGRATIONS_DIR="${MIGRATIONS_DIR}" sh "${SCRIPT_DIR}/migrate.sh"
MIGRATIONS_DIR="${MIGRATIONS_DIR}" sh "${SCRIPT_DIR}/migrate.sh"

expected=4
applied=$(psql "${DATABASE_URL}" -Atc "SELECT COUNT(*) FROM schema_migrations")
if [ "${applied}" -lt "${expected}" ]; then
	 echo "Expected at least ${expected} migrations, found ${applied}" >&2
	 exit 1
fi

psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -c "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('seasons','players','fixtures','sync_runs','squad_plans') ORDER BY table_name"
echo "Migration verification passed (${applied} migrations applied)."
