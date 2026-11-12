# servys backend

Go-бэкенд servys: личный гараж (авто, пробег, ТО, alerts) и вход из Telegram или MAX.
Bitrix и кабинет СТО сняты. Хранилище — SQLite (создаётся и мигрируется автоматически при старте).

## Запуск

```bash
cd backend
go run .            # :8080, БД ./data/app.db
curl localhost:8080/api/v1/health   # {"status":"ok"}
```

Локальный запуск из корня репозитория:

```bash
make dev-backend
```

Вход включается `JWT_SECRET` (см. `.env.example`):

```bash
JWT_SECRET=dev go run .
```

## Переменные окружения

| Переменная | Зачем | Без неё |
|------------|-------|---------|
| `PORT` | порт HTTP | `8080` |
| `DB_PATH` | путь к SQLite | `./data/app.db` |
| `JWT_SECRET` | подпись JWT → включает **auth** | auth-эндпоинты `503` |
| `TELEGRAM_BOT_TOKEN` | вход через Telegram | `/auth/telegram` `503` |
| `LLM_MODE` | `live` ищет отзывы и страницы и извлекает интервал через Ollama. `disabled` и `fixture` не подставляют готовый список | `disabled` |
| `SEARXNG_BASE_URL` / `OLLAMA_BASE_URL` / `OLLAMA_SEARCH_MODEL` / `OLLAMA_MODEL` | поиск и Ollama при `LLM_MODE=live`. Поиск `bonsai-2:27b`, текст `qwen3.5:9b` (ADR-006) | — |
| `VIN_MODE` | `live` (Drom, затем NHTSA) \| `fixture` | `fixture` |
| `VIN_FIXTURE_FALLBACK` | `1` при `VIN_MODE=live` — фикстура после Drom и NHTSA | выключен |

## Слои (пакеты)

`api` · `store` · `domain` · `auth` · `messenger` · `maintenance` · `engine` · `recommender` · `vin`

## API (кратко)

- **Гараж** (Bearer JWT): `GET /api/v1/health`, `POST /vin/resolve`, `POST/GET /vehicles`, `GET/DELETE /vehicles/{id}`,
  `PATCH /vehicles/{id}/odometer`, `POST/GET /vehicles/{id}/service-events`, `POST/GET /vehicles/{id}/history`,
  `POST /vehicles/{id}/history-file`, `GET /vehicles/{id}/alerts`.
- **auth**: `/api/v1/auth/{telegram,max,refresh,logout,me}`. `POST /auth/dev` есть только при `SERVYS_DEV_LOGIN=1`. См. `backend/auth/README.md`.
- **Вебхуки**: `POST /api/v1/telegram/webhook`, `POST /api/v1/max/webhook`.

## Тесты

```bash
go test ./...      # api, auth, engine, recommender, store, vin
```
