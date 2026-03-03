# db_for_work_bot

Telegram knowledge-base bot built with Go and PostgreSQL.

## Features

- Access control with one-time code activation per user.
- Category and article browsing via inline keyboards.
- Webhook mode for production and polling mode for local/dev.
- Automated DB migration on container startup.

## Architecture

- `cmd/bot/main.go`: app entrypoint, mode selection, graceful shutdown.
- `internal/config/config.go`: env configuration loader.
- `internal/db/db.go`: DB connection pool setup.
- `internal/db/queries.go`: SQL query layer.
- `internal/telegram/handlers.go`: update handling and access flow.
- `internal/telegram/keyboards.go`: keyboard builders.
- `migrations/001_init.sql`: schema + seed data.

## Environment

Copy `.env.example` to `.env` and set:

- `BOT_TOKEN`
- `ACCESS_CODE`
- `ADMIN_USER_ID`
- `WEBHOOK_URL` (required only for webhook mode)
- `WEBHOOK_SECRET` (optional but recommended in webhook mode)

Default `.env.example` is prepared for Docker Compose (`DATABASE_URL` points to `db:5432`).

## Quick Start (Docker)

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

If you run bot directly from host (`go run`), use:

- `DATABASE_URL=postgres://kb:kb@127.0.0.1:5433/kb?sslmode=disable`

## Automated Ubuntu Deploy

```bash
git clone https://github.com/Levis-Artur/db_for_work_bot.git
cd db_for_work_bot
cp .env.example .env
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

`scripts/deploy.sh` installs missing dependencies, ensures Docker + Compose, validates `.env`, then starts `db`, `migrate`, and `bot`.

## Run Modes

- Webhook mode: set `WEBHOOK_URL=https://your-domain/tg/webhook`
- Optionally set `WEBHOOK_SECRET` and configure the same secret in your reverse proxy forwarding.
- Polling mode: leave `WEBHOOK_URL` empty

## Admin panel

- Command: `/admin`
- Access: only users where `users.is_active = true` and `users.role IN ('admin', 'owner')`
- Non-admin response: `Немає доступу`
- Admin capabilities in Telegram:
  - Manage categories: add, rename, enable/disable, move up/down
  - Manage articles: pick category, add article, edit title/body, publish/unpublish, move up/down
  - Cancel current admin input state via `Скасувати` (`adm:cancel`) or `/cancel`

## Content Management

Open PostgreSQL shell:

```bash
docker compose exec db psql -U kb -d kb
```

Examples:

```sql
INSERT INTO categories(name, sort_order) VALUES ('Topic 4', 40);

INSERT INTO articles(category_id, title, body, sort_order, is_published)
VALUES (1, 'New article', '<b>Text</b><br>Description...', 20, true);

UPDATE articles SET is_published = false WHERE id = 1;
```

## Validation

```bash
go build ./...
docker compose config
```
