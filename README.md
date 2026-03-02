# db_for_work_bot

Telegram knowledge base bot on Go + PostgreSQL.

## Quick Server Deploy

```bash
git clone https://github.com/Levis-Artur/db_for_work_bot.git
cd db_for_work_bot
cp .env.example .env
```

Fill `.env`:

- `BOT_TOKEN`
- `ACCESS_CODE`
- `ADMIN_USER_ID`
- `WEBHOOK_URL` (for webhook mode) or leave empty (polling mode)

Run deploy:

```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

Script does:

1. Installs base dependencies (`curl`, `git`).
2. Installs Docker if missing.
3. Ensures Docker Compose is available.
4. Validates `.env`.
5. Builds and starts `db`, `migrate`, `bot`.

## Local Run (Docker)

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

## Services

- `db`: PostgreSQL
- `migrate`: applies `migrations/001_init.sql`
- `bot`: Telegram bot

## Useful Commands

```bash
docker compose logs -f bot
docker compose logs -f db
docker compose restart bot
docker compose down
```

## Content Management

Open SQL console:

```bash
docker compose exec db psql -U kb -d kb
```

Examples:

```sql
INSERT INTO categories(name, sort_order) VALUES ('Тема 4', 40);

INSERT INTO articles(category_id, title, body, sort_order, is_published)
VALUES (1, 'Нова стаття', '<b>Текст</b><br>Опис...', 20, true);

UPDATE articles SET is_published = false WHERE id = 1;
```
