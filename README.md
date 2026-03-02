# db_for_work_bot

Telegram knowledge base bot on Go with PostgreSQL.

## Stack

- Go
- PostgreSQL
- Docker Compose

## Project Structure

```text
cmd/bot/main.go
internal/config/config.go
internal/db/db.go
internal/db/queries.go
internal/telegram/handlers.go
internal/telegram/keyboards.go
migrations/001_init.sql
docker-compose.yml
.env.example
```

## Local Run

1. Copy env:

```powershell
Copy-Item .env.example .env
```

2. Fill `.env` with real values (`BOT_TOKEN`, `ACCESS_CODE`, `ADMIN_USER_ID`).

3. Start DB:

```powershell
docker compose up -d db
```

4. Apply migration:

```powershell
Get-Content migrations\001_init.sql | docker compose exec -T db psql -U kb -d kb -v ON_ERROR_STOP=1
```

5. Run bot:

```powershell
go run ./cmd/bot/main.go
```

## Build Check

```powershell
go build ./...
```

## First Commit And Push

```powershell
git init
git add .
git commit -m "chore: initial project setup"
git branch -M main
git remote add origin <your-github-repo-url>
git push -u origin main
```
