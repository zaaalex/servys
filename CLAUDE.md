# servys — контекст для Claude Code

Приложение для автовладельцев: по марке/модели/году и пробегу подсказывает, что и когда
обслужить (регламент + типовые поломки). Представление — приложение **Bitrix24**, ядро — **Go**.

**Прежде чем что-либо делать — прочитай спеку (источник правды):**
`docs/superpowers/specs/2026-07-11-servys-mvp-design.md`

## Раскладка

```
backend/     Go-модуль: api/ store/ main.go domain/ recommender/ bitrix/
frontend/    Vue SPA, iframe-приложение Bitrix24
```

## Кто чем владеет (не заходи в чужой слой)

| Dev | Владеет | НЕ трогает |
|-----|---------|-----------|
| 1 (backend)      | `backend/api/`, `backend/store/`, `backend/main.go`, `backend/domain/` | `recommender/`, `bitrix/`, `frontend/` |
| 2 (frontend)     | `frontend/` | `backend/` |
| 3 (integrations) | `backend/recommender/`, `backend/bitrix/` | `backend/api/`, `backend/main.go`, `frontend/` |

Скажи, кто ты (Dev 1/2/3) — и я работаю строго в твоём слое по твоему чек-листу.

## Замороженные контракты (менять только по общему согласию)

- **HTTP API:** `POST /api/v1/recommendations`, `GET /api/v1/health` — форматы в спеке, §4.A.
- **Go-интерфейс** `recommender.Recommender` + типы `domain.{Car,Item,Result}` — спека, §4.B.
- Пока API не поднят, фронт работает по `frontend/mock/recommendations.json`.

## Правила работы

- Ветка на человека: `dev1-backend` / `dev2-frontend` / `dev3-integrations`. Мержим в `main` часто.
- `backend/main.go` и `backend/domain/` правит только Dev 1.
- Скоуп — жёсткий MVP (спека §8). Не тащим отложенные фичи.
- LLM = Claude API; перед реализацией `recommender/` свериться со скиллом `claude-api`
  (модель, ключ через env `ANTHROPIC_API_KEY`).
