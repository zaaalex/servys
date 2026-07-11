# servys backend

Go-бэкенд servys: b2c-платформа обслуживания авто + b2b-движок удержания для СТО + единый auth.
Хранилище — SQLite (создаётся и мигрируется автоматически при старте).

## Запуск

```bash
cd backend
go run .            # :8080, БД ./data/app.db
curl localhost:8080/api/v1/health   # {"status":"ok"}
```

Полный набор фич — задать env (см. `.env.example`):

```bash
APP_SECRET_KEY=dev JWT_SECRET=dev ADMIN_TOKEN=adm B2B_SCAN_INTERVAL=10m go run .
```

## Переменные окружения

| Переменная | Зачем | Без неё |
|------------|-------|---------|
| `PORT` | порт HTTP | `8080` |
| `DB_PATH` | путь к SQLite | `./data/app.db` |
| `APP_SECRET_KEY` | шифрование вебхуков СТО → включает **b2b** | b2b-эндпоинты `503` |
| `JWT_SECRET` | подпись JWT → включает **auth** | auth-эндпоинты `503` |
| `ADMIN_TOKEN` | операторские действия (`scan-all`) | `scan-all` `503` |
| `TELEGRAM_BOT_TOKEN` | вход через Telegram | `/auth/telegram` `503` |
| `B2B_SCAN_INTERVAL` | период автоскана СТО (напр. `10m`) | шедулер выключен |

## Слои (пакеты)

`api` · `store` (SQLite, WAL) · `domain` · `engine` (движок ТО) · `recommender`+`vin` (рекомендации — Dev 3, пока заглушки) · `sink`+`bitrix` (интеграция Bitrix) · `b2b` · `auth` · `crypto`

## API (кратко)

- **b2c** (идентификация `X-Client-ID`): `GET /api/v1/health`, `/me`, `POST/GET /vehicles`, `GET /vehicles/{id}`,
  `PATCH /vehicles/{id}/odometer`, `POST/GET /vehicles/{id}/service-events`, `GET /vehicles/{id}/alerts`.
- **auth**: `/api/v1/auth/{register,login,telegram,refresh,logout,me,switch}` → `backend/auth/README.md`.
- **b2b** (Bearer + per-СТО; `scan-all` — `X-Admin-Token`): `/api/v1/b2b/service-centers[...]`, `/api/v1/b2b/scan-all`
  → `backend/b2b/README.md`.

## Тесты

```bash
go test ./...      # api, auth, b2b, bitrix, crypto, engine, store
```
