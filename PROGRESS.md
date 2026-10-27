# PROGRESS — статус и параллельная работа

Живой статус-борд, чтобы Dev 1/2/3 работали параллельно и не путались.
**Правила ведения:** обновляй строку своего слоя; не редактируй чужие доски и чужие пакеты.
**Единый источник правды по контрактам — спека §4** (`docs/superpowers/specs/2026-07-11-servys-mvp-design.md`).
ADR-001 — детальное «почему» по данным/бэку; при конфликте со спекой по контрактам — **правит спека**.

_Обновлено: 2026-10-26 · личный гараж в Telegram и MAX. Вход только из мессенджера. Экраны — бумажная колонка в духе trainys, снизу Гараж, Книжка и Профиль справа. Книжка хранит, что и когда меняли. Интервалы из раздела документации, который совпал с годом, рынком и мощностью. Форумы дочитываются в фоне и не подменяют этот раздел. Готового профиля нет. Модель на немецком VPS — qwen2.5:0.5b (ADR-003)._

---

## Жёсткое деление областей (не заходить в чужую)

| Dev | Область | Кто | Пакеты/папки (владеет) | НЕ трогает |
|-----|---------|-----|------------------------|-----------|
| **1** | **Go-сервер / платформа** | klnkklnk (мы) | `backend/{api,store,domain*,auth,messenger,maintenance,main.go}`, `go.mod` | `recommender/`, `vin/`, `engine/`, `data/`, `frontend/` |
| **2** | **Фронтенд-сервер** | Карина Демченко | `frontend/` | весь `backend/` |
| **3** | **Рекомендательный слой** | Alexandr Zorko | `backend/{vin,recommender,engine,data}` | `api/`, `store/`, `main.go`, `frontend/` |

**Рекомендательный слой** (термин Dev 3) = всё, что превращает авто в план ТО:
парсинг **VIN**, логика по **пробегу**, **выявление** «что и когда обслужить», база знаний/правила, LLM/источники.

`domain/` — общий (стюард Dev 1), заморожен; меняем только по согласию всех троих.

### Шов Dev 1 ↔ Dev 3 (один вход)

Dev 3 отдаёт наружу порт `Advisor`: по машине и истории работ → `[]domain.Alert`, плюс `VINProvider`.

```go
Alerts(ctx, domain.Vehicle, history []domain.ServiceEvent) ([]domain.Alert, error)
```

`api` зовёт только его. `main` подключает `recommender.NewFromEnv` и `vin.NewFromEnv`.

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

- **HTTP API**: `GET /health`, `POST /auth/telegram`, `POST /auth/max`, `POST /auth/refresh`, `POST /auth/logout`, `GET /auth/me`, `POST /vin/resolve`, `POST /vehicles`, `GET /vehicles`,
  `GET /vehicles/{id}`, `DELETE /vehicles/{id}`, `PATCH /vehicles/{id}/odometer`, `POST|GET /vehicles/{id}/service-events`,
  `POST|GET /vehicles/{id}/history`, `POST /vehicles/{id}/history-file`, `GET /vehicles/{id}/alerts`, вебхуки `/telegram/webhook` и `/max/webhook`.
  Гараж за Bearer. JSON snake_case. `identification_source`: `manual` | `drom` | `nhtsa`.
- **Go-порты**: `recommender.Advisor` (+ `Recommender`), `vin.VINProvider`;
  типы `domain.{Account,Vehicle,Rule,Alert}`. Владелец машины — `Vehicle.AccountID`.

---

## Доски

### Dev 1 — Go-сервер/платформа · klnkklnk
- [x] Скелет: chi API (`vehicles`/`alerts`), SQLite+миграции, users/vehicles CRUD, wiring, тесты
- [x] Шов: `api` зовёт `recommender.Advisor`. В `main` — `NewFromEnv`. `NewStubAdvisor` только в тестах `api`.
- [x] `POST /vehicles/{id}/service-events` + история ТО прокинута в `Advisor` (baseline для Dev 3)
- [x] CORS (allow-* для standalone-фронта) — в `api`
- [x] `GET /vehicles/{id}/service-events` (журнал ТО для UI)
- [x] **Auth**: Telegram и MAX, JWT access/refresh, сессия личного гаража. Email/пароль и `POST /auth/switch` сняты. См. `backend/auth/README.md`
- [x] **b2c привязан к аккаунту**: эндпоинты гаража за Bearer, авто скоупятся по `account_id` (X-Client-ID убран)
- [x] `DELETE /vehicles/{id}` (+ `store.DeleteVehicle`, каскад пробег/ТО) — под фичу удаления из гаража (запрос Dev 2). ⚠️ вписан в замороженный §4.A постфактум — на ревью/согласование Dev 1.
- **Статус:** личный гараж (account-based) и auth — в коде. Bitrix и кабинет СТО удалены. `main` подключает `Advisor` и `VINProvider` через `NewFromEnv`.

### Dev 2 — Фронтенд-сервер (Vue/TS) · Карина Демченко · `frontend/`
- [x] Переalign на контракт `vehicles`/`alerts` (§4.A)
- [x] Экраны b2c: гараж · добавление авто · рекомендации (карусель + 3D-сцена) · обновление пробега
- [x] Удаление машины из гаража (инлайн-подтверждение) — `client.deleteVehicle` → `DELETE /vehicles/{id}` + `useGarage.removeVehicle`
- [x] Сворачивание/разворачивание панели «Мой гараж» (плавно, состояние в localStorage)
- [x] Переименование «Регламент» → «Что пора обслужить»
- [x] Красная точка срочности (`useFleetAlerts`): в списке гаража, у активной машины возле «Что пора обслужить», на свёрнутом рельсе — при `OVERDUE`/`DUE` из `alerts`
- [x] Приложение за логином: после входа только гараж (Bearer). Вкладок «Кабинет СТО» нет.
- [x] «Что пора обслужить»: пока идёт разбор — «Агент ещё анализирует отзывы», без скелетов и без клиентского сдвига срока.
- **Координация:** **не трогать** `backend/`.

### Dev 3 — Рекомендательный слой · Alexandr Zorko · `backend/{vin,recommender,engine,data}`
> **Статус:** `main` поднимает `recommender.NewFromEnv`. При `LLM_MODE=live` интервалы только из найденных страниц и Ollama. Готового профиля в выдаче нет. VIN: Drom, затем NHTSA. `NewStubAdvisor` остаётся только в тестах `api`.
- [x] `vin.VINProvider`: Drom, затем NHTSA. `vin.Stub` только в тестах.
- [x] `recommender`: порт `Advisor`. При `LLM_MODE=live` поиск и Ollama, в карточке цитата страницы.
- [x] `engine`: `BuildAlerts` по пробегу и истории работ.
- [x] LLM: локальная Ollama, на VPS `qwen2.5:0.5b` (ADR-003). Промпт просит все замены со страницы, не одно масло.
- [x] Таблица «Под замену / тыс. км» на странице регламента читается целиком: масло, фильтры, свечи, жидкости. Числа из страницы.
- [x] Раздел таблицы выбирается по году, мощности и рынку VIN (LJD — Китай), а не по первой таблице страницы. Первый запрос читает этот раздел. Повторный показывает сохранённый разбор. Форумы дочитываются в фоне раз в `RECO_REFRESH` (по умолчанию 6 часов) и не затирают строки документации.
- [x] VIN и ручной ввод (марка, модель, год) идут в один поиск документации. Адрес каталога и текст поиска собираются из введённой марки и модели. Без мощности остаются только строки, одинаковые у модификаций этого года. Чужой год и запчасть с разным интервалом у модификаций не попадают в список.
- Заводская книга именно для VIN `LJD3AA293L0051345` отдельной страницей не найдена. Карточки идут из совпавшего раздела auto.ru, не из чужого абзаца «для Китая 5 000 км» в мануалах других моторов.
- **Не трогать** `api/`, `store/`, `main.go`, `frontend/`. Отдаёшь порты — Dev 1 подключает в `main.go`.

---

## Кабинет СТО и Bitrix — сняты

Личный гараж, рекомендации, VIN и auth (Telegram, MAX) остаются.
Пакеты `backend/bitrix`, `backend/b2b`, `backend/sink`, шифрование вебхуков и `/api/v1/b2b/*` удалены.
Таблицы `service_centers` и `pushed_actions` в sqlite не трогаем, чтобы старые файлы открывались; новых чтений и записей нет.

## Ветки

`main` — интеграционная. Рабочие: `dev1-backend` / `dev2-frontend` / `dev3-recommendations`.

## Открытые риски / действия

- ✅ **Dev 2** на контракте `vehicles`/`alerts`: Bearer, snake_case на проводе, camelCase в `client.ts`.
- ✅ **LLM** — локальная Ollama, на VPS `qwen2.5:0.5b` (ADR-003). Ключ облачной модели в код не кладётся.
- ⚠️ **ADR-001 vs спека:** ADR правит Zorko часто; контракты берём из **спеки §4**, ADR — за деталями данных.
