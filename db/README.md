# Database migrations

Migrations are ordered SQL files and are intended to run against PostgreSQL. Set `DATABASE_URL` before running `sh db/migrate.sh`.

The backend also verifies and creates the schema at startup, then records the four versioned migrations in `schema_migrations`. To verify the migration set twice, run `sh db/verify.sh` from the repository root or run it inside the Compose database container:

```sh
docker compose exec -T db sh -c 'DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@localhost:5432/$POSTGRES_DB?sslmode=disable" sh /migrations/verify.sh'
```

The PostgreSQL repository upserts by stable FPL source IDs and wraps snapshot, squad, and sync metadata writes in transactions.
