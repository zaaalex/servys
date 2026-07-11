# PROGRESS — статус и параллельная работа

Живой статус-борд, чтобы Dev 1/2/3 работали параллельно и не путались.
**Правила ведения:** обновляй строку своего слоя при изменениях; не редактируй чужие доски.
**Единый источник правды по контрактам — спека §4** (`docs/superpowers/specs/2026-07-11-servys-mvp-design.md`).
ADR-001 — детальное «почему» по бэку/данным; при конфликте со спекой по контрактам — **правит спека**.

_Обновлено: 2026-07-11 · режим **b2c** (Bitrix/b2b отложены)._

---

## Фаза

- **Фаза 0** (заморозка контрактов + скелет) — ✅ готово, скелет бэка в `main`.
- **Фаза 1** (параллельная реализация) — 🔄 идёт.
- **Фаза 2** (интеграция + e2e) — ⏳.

## Уже в `main` (готово)

- Спека: `docs/superpowers/specs/2026-07-11-servys-mvp-design.md`
- ADR: `docs/adr/ADR-001-car-maintenance-mvp.md` (+ `README.md`, `template.md`)
- `CLAUDE.md` — авто-контекст
- **Backend-скелет** (компилируется, тесты `engine/store/api` зелёные, e2e smoke ок):
  `domain/ api/ store/ engine/ sink/` + стабы `recommender/ vin/`.
  Запуск: `cd backend && go run .` (`:8080`), проверка: `curl localhost:8080/api/v1/health`.

## Замороженные контракты (менять только по согласию всех троих)

- **HTTP API** (§4.A): `GET /health`, `GET /me`, `POST /vin/resolve`, `POST /vehicles`,
  `GET /vehicles`, `GET /vehicles/{id}`, `PATCH /vehicles/{id}/odometer`,
  `POST /vehicles/{id}/service-events`, `GET /vehicles/{id}/alerts`. Идентификация — `X-Client-ID`.
- **Go-порты** (§4.B/C): `recommender.Recommender`, `vin.VINProvider`, `sink.Sink`;
  типы `domain.{Tenant,User,Vehicle,Rule,Alert}`.

---

## Доски по разработчикам

### Dev 1 — Backend core · klnkklnk · владеет `backend/{api,store,domain,engine,sink,main.go}`
- [x] Скелет: контракты, chi API (`vehicles`/`alerts`), SQLite+миграции, движок напоминаний, тесты, wiring
- [ ] `POST /vehicles/{id}/service-events` — baseline истории ТО (ADR §S12)
- [ ] `MAINTENANCE_HISTORY_REQUIRED`, когда нет baseline (сейчас упрощение: baseline = 0)
- [ ] Подключить боевые `Recommender`/`VINProvider` от Dev 3 в `main.go` (замена стабов)
- **Статус:** скелет готов и в `main`; жду реализацию Dev 3 для интеграции.

### Dev 2 — Frontend (Vue/TS) · Карина Демченко · владеет `frontend/`
- [ ] ⚠️ **ПЕРЕALIGN:** контракт сменился с `/recommendations` на модель `vehicles`/`alerts` (§4.A).
      Обновить `types/api.ts`, `api/client.ts`, мок.
- [ ] `X-Client-ID`: генерить uuid в `localStorage`, слать в каждом запросе.
- [ ] Экраны: гараж (список авто) · добавить авто (ручной ввод, VIN опц.) · карточка с alerts · обновить пробег.
- [ ] Дёргать: `GET /me`, `GET/POST /vehicles`, `GET /vehicles/{id}/alerts`, `PATCH /odometer`.
- **Работать против:** живого бэка (`go run` в `backend/`) или мока. **Не трогать** `backend/`.

### Dev 3 — Рекомендательный слой + VIN · Aleksandr Zorko · владеет `backend/{recommender,vin,data}`
- [ ] `recommender.Recommender`: читать `data/maintenance_rules.yaml` + LLM (Claude) для моделей вне YAML.
      Заменить `recommender.Stub` (не выдумывать интервалы → пусто = `REGULATION_NOT_FOUND`).
- [ ] `vin.VINProvider`: адаптер Drom (best-effort), заменить `vin.Stub`.
- [ ] Расширять `data/maintenance_rules.yaml`.
- [ ] LLM: свериться со скиллом `claude-api`, ключ через env `ANTHROPIC_API_KEY`.
- **Не трогать** `api/`, `store/`, `main.go`, `frontend/`. Отдаёшь конструкторы — Dev 1 подключает.

---

## Интеграция (фаза 2)

1. Dev 3 отдаёт боевые `Recommender`/`VINProvider` → Dev 1 подключает в `main.go`.
2. Dev 2 переключает фронт с мока на живой API.
3. e2e по одной демо-машине (KIA K3, 95000 км уже работает на стабе).

## Отложено (b2b / позже)

Bitrix (`sink/`+`bitrix/`, `tasks.task.add`), OAuth/Marketplace, CRM-сущности, календарь,
шедулер-рассылка, `ODOMETER_UPDATE_REQUIRED` (30 дней). Порт `Sink` уже заложен — доращивается адаптером.

## Ветки

`main` — интеграционная (сюда мержим рано и часто). Рабочие: `dev1-backend` / `dev2-frontend` / `dev3-recommender`.

## Открытые риски / действия

- ⚠️ **Dev 2 переalign** под `vehicles`/`alerts` — критично, фронт строился на `/recommendations`.
- ⚠️ **ADR-001 разъехался со спекой** (после переписывания Zorko нет портов/`b2b`/имени).
  Контракты берём из **спеки §4**; ADR — за деталями бэка/данных. При расхождении — синхронизируем спекой.
- Роли выше проставлены по авторству коммитов — **поправьте, если распределение другое**.
