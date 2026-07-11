# servys — контекст для Claude Code

Сервис превентивного обслуживания авто: по марке/модели/году и пробегу подсказывает, что и когда
обслужить (регламент + типовые поломки). Ядро — **Go**. Основное представление — **standalone
веб-приложение на Vue (TypeScript)**. Bitrix24 — **интеграционный канал** (уведомления → в перспективе CRM),
а не UI.

**Прежде чем что-либо делать — прочитай спеку (источник правды):**
`docs/superpowers/specs/2026-07-11-servys-mvp-design.md`

## Два режима (tenant type)

- **b2c** — частник: работает через Vue, опц. напоминания в Bitrix.
- **b2b** — автосервис/дилер: тот же Vue-процесс **+** CRM-движок удержания в их Bitrix24.
  b2b — аддитивное расширение b2c, а не отдельная система.

## Раскладка

```
backend/     Go-модуль: api/ store/ sink/ main.go domain/ recommender/ bitrix/
frontend/    Vue SPA (TypeScript), standalone веб-приложение
```

## Кто чем владеет (не заходи в чужой слой)

| Dev | Владеет | НЕ трогает |
|-----|---------|-----------|
| 1 (backend)      | `backend/api/`, `backend/store/`, `backend/sink/` (порт), `backend/main.go`, `backend/domain/` | `recommender/`, `bitrix/`, `frontend/` |
| 2 (frontend)     | `frontend/` | `backend/` |
| 3 (integrations) | `backend/recommender/`, `backend/bitrix/` | `backend/api/`, `backend/main.go`, `frontend/` |

Скажи, кто ты (Dev 1/2/3) — и я работаю строго в твоём слое по твоему чек-листу.

## Замороженные контракты (менять только по общему согласию)

- **HTTP API (pull, общий для b2c/b2b):** `POST /api/v1/recommendations`, `GET /api/v1/health` — форматы в спеке §4.A.
- **Порт `recommender.Recommender`** + типы `domain.{Tenant,Car,Item,Result}` — спека §4.B.
- **Порт `sink.Sink`** (push-каналы: IM → календарь → CRM), DTO `sink.Reminder` — спека §4.C.
- Пока API не поднят, фронт работает по `frontend/mock/recommendations.json`.

## Правила работы

- Ветка на человека: `dev1-backend` / `dev2-frontend` / `dev3-integrations`. Мержим в `main` часто.
- `backend/main.go`, `backend/domain/`, `backend/sink/` (порт) правит только Dev 1.
- Скоуп — жёсткий MVP (спека §8): b2c + рекомендации + демо-уведомление в Bitrix. CRM/b2b — v3, аддитивно.
- LLM = Claude API; перед реализацией `recommender/` свериться со скиллом `claude-api`
  (модель, ключ через env `ANTHROPIC_API_KEY`).
