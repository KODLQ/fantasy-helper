#!/bin/sh
set -eu

: "${DATABASE_URL:?Set DATABASE_URL before running migrations}"

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)

psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

MIGRATIONS_DIR="${MIGRATIONS_DIR:-${SCRIPT_DIR}/migrations}"

for migration in "${MIGRATIONS_DIR}"/*.up.sql; do
  version=$(basename "${migration}" .up.sql)
  applied=$(psql "${DATABASE_URL}" -Atc "SELECT version FROM schema_migrations WHERE version = '${version}'")
  if [ -n "${applied}" ]; then
    echo "Skipping ${version}"
    continue
  fi
  echo "Applying ${version}"
  psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "${migration}"
  psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations (version) VALUES ('${version}')"
done
