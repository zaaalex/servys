# PROGRESS — статус и параллельная работа

Живой статус-борд, чтобы Dev 1/2/3 работали параллельно и не путались.
**Правила ведения:** обновляй строку своего слоя; не редактируй чужие доски и чужие пакеты.
**Единый источник правды по контрактам — спека §4** (`docs/superpowers/specs/2026-07-11-servys-mvp-design.md`).
ADR-001 — детальное «почему» по данным/бэку; при конфликте со спекой по контрактам — **правит спека**.

_Обновлено: 2026-07-11 · режим **b2c**. Bitrix — **только b2b**. Режимы b2c/b2b запланированы, **b2b отложен**._

---

## Жёсткое деление областей (не заходить в чужую)

| Dev | Область | Кто | Пакеты/папки (владеет) | НЕ трогает |
|-----|---------|-----|------------------------|-----------|
| **1** | **Go-сервер / платформа** | klnkklnk (мы) | `backend/{api,store,domain*,sink,main.go}`, `go.mod` | `recommender/`, `vin/`, `engine/`, `data/`, `bitrix/`, `frontend/` |
| **2** | **Фронтенд-сервер** | Карина Демченко | `frontend/` | весь `backend/` |
| **3** | **Рекомендательный слой** | Alexandr Zorko | `backend/{vin,recommender,engine,data}` | `api/`, `store/`, `main.go`, `sink/`, `frontend/` |

**Рекомендательный слой** (термин Dev 3) = всё, что превращает авто в план ТО:
парсинг **VIN**, логика по **пробегу**, **выявление** «что и когда обслужить», база знаний/правила, LLM/источники.

`domain/` — общий (стюард Dev 1), заморожен; меняем только по согласию всех троих.

### Шов Dev 1 ↔ Dev 3 (один вход)

Dev 3 отдаёт наружу **один порт**: по `domain.Vehicle` → `[]domain.Alert`, плюс `VINProvider`.
Предлагаемый порт: `recommender.Advisor { Alerts(ctx, domain.Vehicle) ([]domain.Alert, error) }`.
`api` (Dev 1) зовёт только его; `vin`/`recommender`/`engine` целиком внутри Dev 3.
> Dev 1 TODO: отрефакторить скелет с прямого вызова `engine.BuildAlerts` на порт `Advisor`.

---

## Фаза

- **Фаза 0** (контракты + скелет) — ✅ в `main`.
- **Фаза 1** (параллельная реализация) — 🔄 идёт.
- **Фаза 2** (интеграция + e2e) — ⏳.

## Уже в `main`

- Спека, ADR-001 (+ README/template), `CLAUDE.md`, `PROGRESS.md`.
- **Backend-скелет** (компилируется, тесты `engine/store/api` зелёные, e2e smoke ок).
  Запуск: `cd backend && go run .` (`:8080`), проверка: `curl localhost:8080/api/v1/health`.

## Замороженные контракты (§4 спеки)

- **HTTP API**: `GET /health`, `GET /me`, `POST /vin/resolve`, `POST /vehicles`, `GET /vehicles`,
  `GET /vehicles/{id}`, `PATCH /vehicles/{id}/odometer`, `POST /vehicles/{id}/service-events`,
  `GET /vehicles/{id}/alerts`. Идентификация — `X-Client-ID`.
- **Go-порты**: `recommender.Advisor` (+ `Recommender`), `vin.VINProvider`, `sink.Sink`;
  типы `domain.{Tenant,User,Vehicle,Rule,Alert}`.

---

## Доски

### Dev 1 — Go-сервер/платформа · klnkklnk
- [x] Скелет: chi API (`vehicles`/`alerts`), SQLite+миграции, users/vehicles CRUD, wiring, тесты
- [x] Отрефакторить шов: `api` зовёт `recommender.Advisor` (порт задан, стаб-адвайзер, тесты зелёные)
- [x] `POST /vehicles/{id}/service-events` + история ТО прокинута в `Advisor` (baseline для Dev 3)
- [x] CORS (allow-* для standalone-фронта) — в `api`
- [ ] Опц.: `GET /vehicles/{id}/service-events` (история для UI)
- **Статус:** платформа в `main`; жду боевой `Advisor`/`VINProvider` от Dev 3.

### Dev 2 — Фронтенд-сервер (Vue/TS) · Карина Демченко · `frontend/`
- [ ] ⚠️ **ПЕРЕALIGN:** контракт `/recommendations` → `vehicles`/`alerts` (§4.A): `types/api.ts`, `api/client.ts`, мок.
- [ ] `X-Client-ID`: uuid в `localStorage`, слать в каждом запросе.
- [ ] Экраны: гараж · добавить авто (ручной ввод, VIN опц.) · карточка с alerts · обновить пробег.
- **Работать против** живого бэка (`go run` в `backend/`) или мока. **Не трогать** `backend/`.

### Dev 3 — Рекомендательный слой · Alexandr Zorko · `backend/{vin,recommender,engine,data}`
> 🚀 **Kickoff:** `backend/recommender/README.md`. Старт агента: _«Я Dev 3, реализуй по backend/recommender/README.md»_.
> Единый таргет — реализовать порт `recommender.Advisor` (заменить `NewStubAdvisor`), плюс `Recommender`/`VINProvider`.
- [ ] `vin.VINProvider`: парсинг Drom (best-effort), заменить `vin.Stub`.
- [ ] `recommender`: база знаний/правила (`data/`) + LLM-догенерация; отдать порт `Advisor`.
- [ ] `engine`: логика «что и когда обслужить» по пробегу (взять скелет `engine.BuildAlerts`, развить; baseline истории ТО, `MAINTENANCE_HISTORY_REQUIRED`).
- [ ] LLM: **провайдер/модель — зона ответственности Dev 3** (решает и фиксирует сам; Claude/Gemini/др.).
- **Не трогать** `api/`, `store/`, `main.go`, `frontend/`. Отдаёшь порты — Dev 1 подключает в `main.go`.

---

## Отложено (b2b / позже)

Bitrix (`sink/`+`bitrix/`, `tasks.task.add`), OAuth/Marketplace, CRM, календарь, шедулер-рассылка.
Режим b2b запланирован; порт `Sink` заложен — доращивается адаптером, ядро не трогаем.

## Ветки

`main` — интеграционная. Рабочие: `dev1-backend` / `dev2-frontend` / `dev3-recommendations`.

## Открытые риски / действия

- ⚠️ **Dev 2 переalign** под `vehicles`/`alerts` — критично.
- ✅ **LLM — зона ответственности Dev 3** (Claude/Gemini/др. — решает сам, фиксирует в ADR). Не блокер;
  расхождение с прежней записью в спеке снято — доки больше не диктуют провайдера.
- ⚠️ **ADR-001 vs спека:** ADR правит Zorko часто; контракты берём из **спеки §4**, ADR — за деталями данных.
