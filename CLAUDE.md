# servys — контекст для Claude Code

Сервис превентивного обслуживания авто: по авто (VIN опц. / ручной ввод) и пробегу подсказывает,
что и когда обслужить по найденным отзывам и страницам. Ядро — **Go**, фронт — **Vue (TypeScript)**,
standalone веб-приложение. Продукт — **личный гараж**. Bitrix24 и кабинет СТО сняты.

**Делаем: фронт + бэк + рекомендательный слой.**

**Прежде чем что-либо делать — прочитай:**
- `PROGRESS.md` — статус-борд и параллельные задачи (обнови свою строку!)
- `docs/superpowers/specs/2026-07-11-servys-mvp-design.md` — спека, **источник правды по контрактам**
- `docs/adr/ADR-001-car-maintenance-mvp.md` — детальные решения бэка/данных (при конфликте по контрактам правит спека)

## Режим

- **Личный гараж** — частник. Вход только из мини-приложения Telegram и MAX. Бот на `/start` открывает гараж. Машину задают VIN или годом и маркой. История обслуживания — текстом или файлом. Варианты работ считает движок. Поиск страниц — `bonsai-2:27b`, текст страницы — `qwen3.5:9b` (ADR-006). На хосте с запасом 15 ГБ они включаются по очереди.

## Раскладка

```
backend/     Go — api/ store/ domain/ auth/ messenger/ maintenance/ main.go
             recommender/ vin/ engine/ data/
frontend/    Vue SPA (TypeScript), standalone
```

## Кто чем владеет (не заходи в чужой слой)

| Dev | Владеет | НЕ трогает |
|-----|---------|-----------|
| 1 (Go-сервер / платформа) | `backend/`: `api/`, `store/`, `domain/` (стюард), `auth/`, `messenger/`, `maintenance/`, `main.go` | `recommender/`, `vin/`, `engine/`, `data/`, `frontend/` |
| 2 (фронтенд-сервер)  | `frontend/` | весь `backend/` |
| 3 (рекомендательный слой) | `backend/`: `recommender/`, `vin/`, `engine/`, `data/` | `api/`, `store/`, `main.go`, `frontend/` |

Скажи, кто ты (Dev 1/2/3) — работаю в твоём слое.

## Замороженные контракты (менять только по общему согласию)

- **HTTP API** — спека §4.A: Bearer, `/auth/*`, `/vehicles`, `/vin/resolve`, `/alerts`, `/history`, вебхуки Telegram и MAX. JSON snake_case. Источник авто: `manual` | `drom` | `nhtsa`.
- **Порты** `recommender.Advisor`, `recommender.Recommender`, `vin.VINProvider` (Drom, затем NHTSA), типы `domain.{Account,Vehicle,Rule,Alert}` — спека §4.B.

## Правила работы

- Ветка на человека: `dev1-backend` / `dev2-frontend` / `dev3-recommendations`. Мержим в `main` часто.
- `backend/main.go`, `backend/domain/` правит только Dev 1.
- Скоуп — личный гараж: авто, пробег+история, движок напоминаний, alerts в приложении,
  интервалы только из найденных страниц при `LLM_MODE=live`. Bitrix и кабинет СТО удалены, не возвращать.
- LLM — локальная Ollama, без ключа. Поиск — `bonsai-2:27b`, текст страницы — `qwen3.5:9b` (ADR-006). Вместе в 15 ГБ не входят, поэтому включаются по очереди. Ключ облачной модели в код и на фронт не класть.
- Архитектурные решения фиксируем в `docs/adr/` (см. `docs/adr/README.md`).
