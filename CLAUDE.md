# servys — контекст для Claude Code

Сервис превентивного обслуживания авто: по авто (VIN опц. / ручной ввод) и пробегу подсказывает,
что и когда обслужить (регламент + типовые поломки). Ядро — **Go**, фронт — **Vue (TypeScript)**,
standalone веб-приложение.

**Изначально делаем: фронт + бэк + рекомендационный слой (b2c). Bitrix — только на уровне b2b, отложен.**

**Прежде чем что-либо делать — прочитай:**
- `PROGRESS.md` — статус-борд и параллельные задачи (обнови свою строку!)
- `docs/superpowers/specs/2026-07-11-servys-mvp-design.md` — спека, **источник правды по контрактам**
- `docs/adr/ADR-001-car-maintenance-mvp.md` — детальные решения бэка/данных (при конфликте по контрактам правит спека)

## Два режима (tenant type)

- **b2c** *(делаем сейчас)* — частник, всё в веб-приложении, без Bitrix.
- **b2b** *(запланирован, отложен)* — автосервис/дилер: та же основа **+** Bitrix24 (задачи
  `tasks.task.add`, далее CRM). Bitrix осмыслен только тут (задача на сотрудника сервиса).

## Раскладка

```
backend/     Go — Dev1: api/ store/ domain/ sink/ bitrix/ main.go  ·  Dev3: recommender/ vin/ engine/ data/
frontend/    Vue SPA (TypeScript), standalone
```

## Кто чем владеет (не заходи в чужой слой)

| Dev | Владеет | НЕ трогает |
|-----|---------|-----------|
| 1 (Go-сервер / платформа + интеграции) | `backend/`: `api/`, `store/`, `domain/` (стюард), `sink/` (порт), `bitrix/`, `main.go` | `recommender/`, `vin/`, `engine/`, `data/`, `frontend/` |
| 2 (фронтенд-сервер)  | `frontend/` | весь `backend/` |
| 3 (рекомендательный слой) | `backend/`: `recommender/`, `vin/`, `engine/`, `data/` | `api/`, `store/`, `sink/`, `main.go`, `frontend/` |

Bitrix-коннектор (`backend/bitrix/`) — Dev 1, за портом `Sink` (вебхук, без OAuth); активируется на b2b.
Скажи, кто ты (Dev 1/2/3) — работаю в твоём слое.

## Замороженные контракты (менять только по общему согласию)

- **HTTP API** (модель `vehicles`/`alerts`): `/me`, `/vin/resolve`, `/vehicles`, `/vehicles/{id}/odometer`,
  `/vehicles/{id}/service-events`, `/vehicles/{id}/alerts`, `/health` — спека §4.A.
  ⚠️ Заменил прежний `/recommendations` — фронт Dev 2 нужно переalign'ить.
- **Порты** `recommender.Advisor` (Vehicle→Alerts, шов Dev1↔Dev3) + `recommender.Recommender` (правила), `vin.VINProvider` (Drom),
  типы `domain.{Tenant,User,Vehicle,Rule,Alert}` — спека §4.B.
- **Порт `sink.Sink`** — b2b/отложено, спека §4.C.

## Правила работы

- Ветка на человека: `dev1-backend` / `dev2-frontend` / `dev3-recommendations`. Мержим в `main` часто.
- `backend/main.go`, `backend/domain/`, `backend/sink/` (порт) правит только Dev 1.
- Скоуп — жёсткий MVP b2c (спека §9): гараж, пробег+история, движок напоминаний, alerts в приложении,
  гибрид YAML+LLM. Bitrix/b2b — отложено.
- LLM — **зона ответственности Dev 3** (провайдер/модель: Claude/Gemini/др. — на его выбор);
  ключ через env, только на бэке, никогда в коде/на фронте.
- Архитектурные решения фиксируем в `docs/adr/` (см. `docs/adr/README.md`).
