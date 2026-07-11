# PROGRESS — статус и параллельная работа

Живой статус-борд, чтобы Dev 1/2/3 работали параллельно и не путались.
**Правила ведения:** обновляй строку своего слоя; не редактируй чужие доски и чужие пакеты.
**Единый источник правды по контрактам — спека §4** (`docs/superpowers/specs/2026-07-11-servys-mvp-design.md`).
ADR-001 — детальное «почему» по данным/бэку; при конфликте со спекой по контрактам — **правит спека**.

_Обновлено: 2026-07-11 · b2c работает e2e · b2b-бэкенд + auth готовы (Dev 1) · b2b/auth-фронт — 🔄 у агента · рекомендации (Dev 3) — заглушки · гараж: удаление авто + сворачивание панели + точки срочности (Dev 2, +`DELETE /vehicles/{id}` в §4.A — согласовать)._

---

## Жёсткое деление областей (не заходить в чужую)

| Dev | Область | Кто | Пакеты/папки (владеет) | НЕ трогает |
|-----|---------|-----|------------------------|-----------|
| **1** | **Go-сервер / платформа + интеграции** | klnkklnk (мы) | `backend/{api,store,domain*,sink,bitrix,main.go}`, `go.mod` | `recommender/`, `vin/`, `engine/`, `data/`, `frontend/` |
| **2** | **Фронтенд-сервер** | Карина Демченко | `frontend/` | весь `backend/` |
| **3** | **Рекомендательный слой** | Alexandr Zorko | `backend/{vin,recommender,engine,data}` | `api/`, `store/`, `main.go`, `sink/`, `bitrix/`, `frontend/` |

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

- **HTTP API**: `GET /health`, `POST /vin/resolve`, `POST /vehicles`, `GET /vehicles`,
  `GET /vehicles/{id}`, `DELETE /vehicles/{id}`, `PATCH /vehicles/{id}/odometer`, `POST /vehicles/{id}/service-events`,
  `GET /vehicles/{id}/alerts`. **Идентификация — Bearer JWT (аккаунт)**: b2c за requireAuth, скоуп по `account_id` (гостевой `X-Client-ID` убран).
  - `DELETE /vehicles/{id}` — добавлен по фиче «удаление из гаража» (запрос Dev 2). Каскад по истории пробега/ТО; `204` при успехе, чужое/несуществующее → `404`. Реализация в `api/`+`store/` (Dev 1). ⚠️ согласовать пост-фактум.
- **Go-порты**: `recommender.Advisor` (+ `Recommender`), `vin.VINProvider`, `sink.Sink`;
  типы `domain.{Tenant,User,Vehicle,Rule,Alert}`.

---

## Доски

### Dev 1 — Go-сервер/платформа · klnkklnk
- [x] Скелет: chi API (`vehicles`/`alerts`), SQLite+миграции, users/vehicles CRUD, wiring, тесты
- [x] Отрефакторить шов: `api` зовёт `recommender.Advisor` (порт задан, стаб-адвайзер, тесты зелёные)
- [x] `POST /vehicles/{id}/service-events` + история ТО прокинута в `Advisor` (baseline для Dev 3)
- [x] CORS (allow-* для standalone-фронта) — в `api`
- [x] `GET /vehicles/{id}/service-events` (журнал ТО для UI)
- [x] Bitrix-коннектор (вебхук, без OAuth) за портом `Sink` — `backend/bitrix` (тесты + live-прогон на портале)
- [x] **B2B-слой** (движок удержания для СТО): connect/list/scan + шедулер, чтение автопарка из CRM, ретеншн-дела (идемпотентно). См. `backend/b2b/README.md`
- [x] **Auth**: единый аккаунт + точки входа (email/Telegram) + JWT access/refresh + переключение b2c/b2b. См. `backend/auth/README.md`
- [x] **Гейт b2b-эндпоинтов**: per-СТО по аккаунту (Bearer + membership), `scan-all` — по `X-Admin-Token`
- [x] **b2c привязан к аккаунту**: b2c-эндпоинты за Bearer, авто скоупятся по `account_id` (X-Client-ID убран) — вход с любой точки → гараж везде
- [x] `DELETE /vehicles/{id}` (+ `store.DeleteVehicle`, каскад пробег/ТО) — под фичу удаления из гаража (запрос Dev 2). ⚠️ вписан в замороженный §4.A постфактум — на ревью/согласование Dev 1.
- **Статус:** b2c-платформа (account-based), Bitrix-коннектор, b2b-слой (+шедулер), auth+гейт — готовы. Жду боевой `Advisor`/`VINProvider` от Dev 3.

### Dev 2 — Фронтенд-сервер (Vue/TS) · Карина Демченко · `frontend/`
- [x] Переalign на контракт `vehicles`/`alerts` (§4.A)
- [x] Экраны b2c: гараж · добавление авто · рекомендации (карусель + 3D-сцена) · обновление пробега
- [x] Удаление машины из гаража (инлайн-подтверждение) — `client.deleteVehicle` → `DELETE /vehicles/{id}` + `useGarage.removeVehicle`
- [x] Сворачивание/разворачивание панели «Мой гараж» (плавно, состояние в localStorage)
- [x] Переименование «Регламент» → «Что пора обслужить»
- [x] Красная точка срочности (`useFleetAlerts`): в списке гаража, у активной машины возле «Что пора обслужить», на свёрнутом рельсе — при `OVERDUE`/`DUE`. Ждёт непустой `Advisor` (Dev 3)
- [ ] 🔄 **Интеграция auth + b2b (наш агент → ветка `integrate/frontend-b2b-auth`)**: приложение за логином,
      b2c-гараж Карины **на аккаунте (Bearer, не X-Client-ID)** + раздел «Кабинет СТО» (b2b) с переключением контекста.
      Собирается агентом с headless-проверкой; в `main` мержим после ревью.
- **Координация:** интеграцию агентской ветки с фронтом Карины свести вручную. **Не трогать** `backend/`.

### Dev 3 — Рекомендательный слой · Alexandr Zorko · `backend/{vin,recommender,engine,data}`
> ⚠️ **Статус:** ADR готов, но в коде `recommender`/`vin`/`engine` — заглушки Dev 1. Это **критический путь** к «настоящему» демо: сейчас любое авто получает одни и те же 3 демо-правила.
> 🚀 **Kickoff:** `backend/recommender/README.md`. Старт агента: _«Я Dev 3, реализуй по backend/recommender/README.md»_.
> Единый таргет — реализовать порт `recommender.Advisor` (заменить `NewStubAdvisor`), плюс `Recommender`/`VINProvider`.
- [ ] `vin.VINProvider`: парсинг Drom (best-effort), заменить `vin.Stub`.
- [ ] `recommender`: база знаний/правила (`data/`) + LLM-догенерация; отдать порт `Advisor`.
- [ ] `engine`: логика «что и когда обслужить» по пробегу (взять скелет `engine.BuildAlerts`, развить; baseline истории ТО, `MAINTENANCE_HISTORY_REQUIRED`).
- [ ] LLM: **провайдер/модель — зона ответственности Dev 3** (решает и фиксирует сам; Claude/Gemini/др.).
- **Не трогать** `api/`, `store/`, `main.go`, `frontend/`. Отдаёшь порты — Dev 1 подключает в `main.go`.

---

## B2B-слой — ✅ минимальный готов (Dev 1)

Движок удержания для СТО/дилеров на входящем вебхуке (без OAuth). Сценарии и эндпоинты — в
`backend/b2b/README.md`:
- `POST /api/v1/b2b/service-centers` — подключить СТО (вебхук валидируется + шифруется);
- `GET /api/v1/b2b/service-centers` — список;
- `POST /api/v1/b2b/service-centers/{id}/scan` — скан одного СТО → ретеншн-дела (идемпотентно);
- `POST /api/v1/b2b/scan-all` — скан всех СТО разом;
- **шедулер** — периодический автоскан всех СТО (env `B2B_SCAN_INTERVAL`, напр. `10m`).
- Ядро рекомендаций — то же (`recommender.Advisor`). Включается при `APP_SECRET_KEY`.

**Отложено:** OAuth/Marketplace и вход через Bitrix, привязка нескольких точек входа к аккаунту (linking),
RS256/JWKS, перевод b2c с гостевого X-Client-ID на аккаунт, обратная синхронизация из CRM,
смарт-процессы/сделки, календарь.
Режим b2b запланирован; порт `Sink` заложен — доращивается адаптером, ядро не трогаем.

## Ветки

`main` — интеграционная. Рабочие: `dev1-backend` / `dev2-frontend` / `dev3-recommendations`.

## Открытые риски / действия

- ⚠️ **Dev 2 переalign** под `vehicles`/`alerts` — критично.
- ✅ **LLM — зона ответственности Dev 3** (Claude/Gemini/др. — решает сам, фиксирует в ADR). Не блокер;
  расхождение с прежней записью в спеке снято — доки больше не диктуют провайдера.
- ⚠️ **ADR-001 vs спека:** ADR правит Zorko часто; контракты берём из **спеки §4**, ADR — за деталями данных.
