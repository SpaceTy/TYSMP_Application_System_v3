## Postgres setup

Bring up the database:

```sh
docker compose up -d postgres
```

On first start, SQL files in `Database/postgres/init/` bootstrap the schema and triggers.

Connect:

```sh
psql "$DATABASE_URL"
```

Set actor for auditing in a session:

```sql
SET application.actor = 'discord-bot';
```

Listen for events (bot):

```sql
LISTEN app_events; -- payload is JSON string
```

