# servys — дизайн MVP и командный процесс

- **Дата:** 2026-07-11 (ред. после ревью ADR-001)
- **Команда:** 3 разработчика
- **Срок:** ~полдня (~5 рабочих часов)
- **Статус:** утверждён, источник правды для реализации
- **Связанный ADR:** `docs/adr/ADR-001-car-maintenance-mvp.md`

> **Ревизия (сведено с ADR-001):** изначально делаем **фронт + бэк + рекомендационный слой (b2c)**.
> Bitrix-интеграция — **только на уровне b2b и отложена** (b2b запланирован, но не в этой сборке). Данные — **гибрид YAML+LLM**.
> Мульти-авто / юзеры / история пробега / шедулер — **оставлены**. Порты `Recommender`/`Sink`/`Tenant`
> сохранены; функционал из ADR встроен за ними.

---

## Статус реализации (обновлено 2026-07-11)

| Слой | Владелец | Статус |
|------|----------|--------|
| Go-бэкенд: b2c-платформа, движок ТО, Bitrix-коннектор, b2b-слой + шедулер, **auth + гейт** | Dev 1 | ✅ в `main`, тесты зелёные |
| Фронт b2c (гараж / авто / пробег / рекомендации, 3D-сцена) | Dev 2 (Карина) | ✅ работает, переalign на `vehicles`/`alerts` |
| Рекомендательный слой (боевые правила/LLM + VIN/Drom) | Dev 3 (Zorko) | ⚠️ ADR готов, код — **заглушки** (`recommender`/`vin`) |
| Фронт **b2b-панель + auth-флоу** (login/JWT/switch, экраны СТО) | агент → ветка `feat/b2b-frontend` | 🔄 **in progress** (в `main` не смёржено) |

- **b2c-вертикаль работает end-to-end** (фронт ↔ API ↔ движок), но рекомендации пока на демо-правилах-заглушках.
- **b2b-бэкенд и auth** проверены через API/тесты; b2b-UI и auth-флоу на фронте — **в работе у агента**.
- Контракты: auth — `backend/auth/README.md`, b2b — `backend/b2b/README.md`.

---

## 1. Продукт

**servys** — сервис превентивного обслуживания авто. По автомобилю (VIN опционально / ручной ввод)
и пробегу подсказывает регламент и типовые поломки, привязанные к пробегу. Пользователь ведёт свой
гараж, обновляет пробег и видит статусы/напоминания в приложении.

**Изначальная сборка (b2c):** фронт + бэк + **рекомендационный слой**. Без Bitrix.

### Два режима (tenant type)

- **b2c** *(делаем сейчас)* — частный автовладелец. Всё в веб-приложении, Bitrix не участвует.
- **b2b** *(запланирован, отложен)* — автосервис/дилер: та же основа **+** интеграция с их Bitrix24
  (задачи `tasks.task.add`, далее CRM). Bitrix осмыслен **только тут**: задача ставится на сотрудника
  сервиса; частнику (b2c) ставить некому.

## 2. Архитектура

Модульный монолит на Go + отдельный веб-фронт (Vue/Vite). Один Go-процесс: REST API + SQLite +
шедулер (+ outbox-воркер на этапе b2b). Два процесса на деве: фронт `:5173`, бэк `:8080`.

**Стек бэка:** `net/http` + `go-chi/chi`, `database/sql` + `modernc.org/sqlite`, SQL-миграции при
старте, явный `App`/constructor-wiring без DI-фреймворка.

### Порты (швы)

- **`Recommender`** — рекомендационный слой: правила из `maintenance_rules.yaml` (верифиц., демо)
  + LLM (провайдер — выбор Dev 3) для моделей вне YAML. **Не выдумывает интервалы**: нет правила → `REGULATION_NOT_FOUND`.
- **`VINProvider`** — VIN → характеристики (адаптер Drom, best-effort). При любой ошибке — ручная форма.
- **`Sink`** — исходящая доставка напоминаний. Реализация Bitrix — **b2b, отложена**; в b2c напоминания
  живут только в приложении.
- **`Tenant`/`User`** — пользователь по browser-token (`X-Client-ID`). `Tenant.Type: b2c|b2b`.

**Движок напоминаний + шедулер** считают статусы (`MAINTENANCE_SOON/DUE/OVERDUE`,
`ODOMETER_UPDATE_REQUIRED` и т.д.) по правилам + истории пробега/обслуживания. **Пробег — временной
ряд** (`odometer_readings`), а не одно число.

**Данные — гибрид:** `backend/data/maintenance_rules.yaml` (верифиц. демо, `verified:false` = не
официальный регламент) + LLM-догенерация для неизвестных моделей.

## 3. Раскладка репозитория

```
servys/
├── CLAUDE.md
├── backend/                  # Go-модуль
│   ├── main.go               # Dev 1 — wiring
│   ├── api/                  # Dev 1 — chi-хендлеры: me/vehicles/vin/odometer/service-events/alerts
│   ├── domain/               # Dev 1 — Tenant/User, Vehicle, Rule, Alert (заморожено на фазе 0)
│   ├── store/                # Dev 1 — SQLite, миграции, репозитории
│   ├── engine/               # Dev 3 — движок «что и когда обслужить» (determination по пробегу)
│   ├── sink/                 # Dev 1 — порт Sink (+ outbox); Bitrix-реализация отложена (b2b)
│   ├── recommender/          # Dev 3 — рекомендательный слой: YAML-правила + LLM (провайдер — выбор Dev 3), impl Advisor/Recommender
│   ├── vin/                  # Dev 3 — VINProvider (адаптер Drom)
│   ├── bitrix/               # Dev 1 — Sink-коннектор (вебхук, tasks.task.add); активен на b2b
│   └── data/maintenance_rules.yaml   # Dev 3 — база правил (демо)
├── frontend/                 # Vue SPA (TypeScript), standalone веб-приложение (Dev 2)
│   └── mock/                 # mock-ответы API для параллельной работы
└── docs/
    ├── adr/                  # ADR (единый дом) + template.md + README.md
    └── superpowers/specs/    # спеки (этот документ)
```

## 4. Замороженные контракты

Швы между разработчиками. Согласуем и фиксируем на фазе 0.

### A. HTTP API-контракт (Go-бэк ↔ Vue-фронт) — модель `vehicles`/`alerts`

> ⚠️ **Смена контракта:** заменяем `POST /api/v1/recommendations` на модель из ADR-001.
> Фронт Dev 2 строился на `/recommendations` + `mock/recommendations.json` — **нужен переalign**
> под эндпоинты ниже (переименовать мок, обновить `types/api.ts`).

```
GET   /api/v1/health                              -> {"status":"ok"}
GET   /api/v1/me                                  # создаёт/находит юзера по X-Client-ID
POST  /api/v1/vin/resolve                         # VIN -> характеристики (best-effort Drom)
POST  /api/v1/vehicles                            # добавить авто (VIN или вручную) + первый пробег
GET   /api/v1/vehicles                            # гараж пользователя
GET   /api/v1/vehicles/{id}
PATCH /api/v1/vehicles/{id}/odometer              # обновить пробег (нельзя уменьшать)
POST  /api/v1/vehicles/{id}/service-events        # отметить ТО выполненным (baseline)
GET   /api/v1/vehicles/{id}/alerts                # рекомендации/статусы по авто
# dev-only: POST /api/v1/dev/run-jobs (при APP_ENV=dev)
```

Идентификация — заголовок `X-Client-ID: <uuid>` (browser-token, MVP-ограничение, не production-auth).

### B. Доменные типы + порты `Recommender` / `VINProvider` (Dev 1 ↔ Dev 3)

```go
package domain

type TenantType string
const ( TenantB2C TenantType = "b2c"; TenantB2B TenantType = "b2b" )

type User struct {
    ID         string
    ClientKey  string
    TenantType TenantType // в b2c-MVP всегда b2c
}

type Vehicle struct {
    ID, UserID           string
    VIN                  string // может быть пустым (ручной ввод)
    Make, Model          string
    Year, EngineCC, PowerHP int
    Color, BodyType      string
    IdentificationSource string // "drom" | "manual"
    CurrentOdometer      int
    OdometerUpdatedAt    time.Time
}

type Rule struct {
    Code, Title, Operation string
    IntervalKm, IntervalMonths, LeadKm int
    Verified bool
    Source   string
}

type Alert struct {
    ID, VehicleID    string
    Type             string // ODOMETER_UPDATE_REQUIRED | MAINTENANCE_SOON | MAINTENANCE_DUE | ...
    RuleCode         string
    Severity, Status string
    Title, Description string
    DueAtKm          int
}
```

```go
package recommender

// Источник знаний: правила для авто (YAML + LLM). Пусто => REGULATION_NOT_FOUND.
// Движок напоминаний (пакет engine, Dev 1) превращает правила + пробег/историю в Alert.
type Recommender interface {
    Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}

// package vin
type VINProvider interface {
    Resolve(ctx context.Context, vin string) (domain.Vehicle, error)
}
```

- **Dev 1** пишет API/движок против **заглушек** `Recommender`/`VINProvider`.
- **Dev 3** реализует боевые: YAML+LLM (провайдер — выбор Dev 3) и адаптер Drom. Подменяет заглушки.
- Провайдер/модель LLM — **зона ответственности Dev 3**; ключ через env, только на бэке.

### C. Исходящий порт `Sink` — **b2b, отложено**

```go
package sink

type Reminder struct {
    Tenant  domain.Tenant
    Vehicle domain.Vehicle
    Alert   domain.Alert
}

type Sink interface {
    Deliver(ctx context.Context, r Reminder) error
}
```

В b2c не задействован (напоминания в приложении). Реализация в `bitrix/` (`tasks.task.add`) —
на этапе b2b. Детали механизма/идемпотентности/outbox — в ADR-001, §5.8–5.9.

## 5. Архитектура фронта (Dev 2)

> **Статус:** фронт **реализован** (`frontend/`) как standalone Vue 3 + TS + Vite, работает по моку
> (`npm run dev`), `typecheck` и `build` зелёные. Слой данных **переведён на контракт §4.A**
> (`vehicles`/`alerts`, camelCase JSON, идентификация `X-Client-ID`). Точные формы бэка ещё не
> заморожены (`backend/` не поднят) — при появлении API сверить казинг/поля с Dev 1; правка
> локализована в `types/api.ts` + `api/client.ts` + моках.

Фронт держит **гараж пользователя** с 3D-аватаром машины и показывает **регламент обслуживания**.
Вся сложность — в чётком разделении слоёв: работать против мока и переключаться на живой API сменой
env, а не правкой кода.

**Стек:** Vue 3 (`<script setup>` + Composition API) + **TypeScript (strict)** + Vite.
Состояние — локальные composables (Pinia избыточна). Сеть — нативный `fetch` в тонкой обёртке.
3D-машина — самописный WebGL без библиотек. Пакетный менеджер — npm.

### Раскладка `frontend/` (как реализовано)

```
frontend/
├── index.html
├── vite.config.ts            # dev-proxy /api → Go-бэк; alias @ (src) и @mock
├── .env / .env.production    # VITE_USE_MOCK, VITE_API_BASE_URL, VITE_API_TARGET
├── mock/
│   ├── vehicles.json         # сид гаража (GET /vehicles) — якорь контракта
│   └── alerts.json           # алерты (GET /vehicles/{id}/alerts)
└── src/
    ├── main.ts
    ├── App.vue               # дек-слайдер: слайд «гараж» + слайд «регламент»
    ├── types/api.ts          # TS-типы контракта §4.A: Vehicle, Alert, VinResolveResult, …
    ├── api/client.ts         # единственная точка сети: эндпоинты §4.A, X-Client-ID, mock-стор
    ├── composables/
    │   ├── useRecommendations.ts   # загрузка алертов: статус + защита от гонки (AbortController)
    │   └── useGarage.ts            # гараж поверх /vehicles: список, активная, add, updateOdometer
    ├── car3d/
    │   └── engine.ts               # WebGL-фабрика: 5 типов кузова, металлик-шейдер, вращение+drag
    ├── data/
    │   ├── presets.ts              # цвета-аватары, типы кузова, apiBodyToScene, hexToRgb
    │   └── vin.ts                  # мок-декодер VIN (за client.resolveVin)
    ├── ui/
    │   ├── tokens.css              # глобальные токены + стили (тёмная кинематографичная тема)
    │   └── status.ts               # AlertStatus → лейбл/тон/порядок (+ fallback)
    └── components/
        ├── CarScene.vue            # обёртка над WebGL-движком (props type/color, destroy на unmount)
        ├── GaragePanel.vue         # сайдбар: профиль + список машин + «добавить»
        ├── AddCarModal.vue         # добавление по VIN (resolve → createVehicle) + мини-3D-превью
        └── RecommendationsView.vue # регламент: алерты, состояния, анимации, обновление пробега
```

### Слои и ответственность

1. **`types/api.ts` — контракт §4.A как типы.** `Vehicle`, `Alert` (статусы `OK|SOON|DUE|OVERDUE|…`),
   `VinResolveResult`, `CreateVehicleRequest`, `Me`. Единственный источник формы данных.
2. **`api/client.ts` — единственная точка сети.** Все эндпоинты §4.A (`me`, `vin/resolve`, `vehicles`,
   `odometer`, `service-events`, `alerts`), заголовок **`X-Client-ID`** (browser-token в localStorage).
   `VITE_USE_MOCK=1` → in-memory стор (add/patch живут в сессии), моки `@mock/*.json`.
   - **Dev-сценарии:** в mock-режиме `success|empty|error|slow` (переключатель на слайде регламента).
   - **Пересчёт статусов** в моке: `recomputeStatus()` по `dueAtKm` относительно `currentOdometer`
     (заглушка вместо reminder-движка Dev 1).
3. **`composables/useRecommendations.ts` — состояние экрана.** Загружает алерты авто; статус + **защита
   от гонки** через `AbortController` (устаревший ответ отбрасывается).
4. **`composables/useGarage.ts` — гараж.** Синглтон поверх `/vehicles`: асинхронная загрузка, активная
   машина, `addVehicle` (→ `POST /vehicles`), `setOdometer` (→ `PATCH /odometer`, нельзя уменьшать).
5. **`car3d/engine.ts` — 3D-движок.** Фабрика `createCarScene(canvas, opts)` → независимые сцены
   (главная + мини-превью в модалке). API: `setType/setColorRGB/flourish/resize/destroy`.
6. **`ui/status.ts` — семантика статусов.** Единый маппинг severity/status → лейбл/тон/порядок с
   **обязательным fallback** для неизвестных значений.
7. **Компоненты — презентационные.** Бизнес-логики нет: рисуют по props, эмитят события; данные и
   состояние — в composables и `client.ts`.

### UI/UX

- **Одна страница — вертикальный слайдер (дек):** слайд 1 — гараж с 3D-машиной, слайд 2 — регламент.
  Центральная стрелка вниз плавно листает вниз; «↑ Гараж» возвращает наверх (`scroll-snap` + smooth).
- **Гараж:** профиль + список машин (цвет-аватар, марка, пробег). Клик по машине делает её активной —
  3D-модель перекрашивается и перестраивается под её тип кузова, регламент внизу пересобирается.
- **Добавление по VIN:** модалка с живым **мини-3D-превью**; VIN → `POST /vin/resolve` (в моке —
  локальный декодер), затем текущий пробег + выбор типа кузова и цвета-аватара → `POST /vehicles`.
- **3D-машина:** WebGL low-poly, 5 типов кузова, цвет кузова из `vehicle.color`, металлик-блик,
  авто-вращение + drag.
- **Регламент:** сводка (одометр-счётчик + бар срочности), карточки алертов со статусами;
  **обновление пробега** (`PATCH /odometer`) пересобирает алерты. Состояния loading/empty/error.
- **Семантика статусов (`status.ts`):** `OVERDUE/DUE` → красный, `SOON/INSPECTION_REQUIRED` → янтарный,
  `OK/NO_INTERVAL/RESEARCHING` → нейтральный; сортировка по срочности; **fallback** для неизвестных.
- **Тема:** тёмная кинематографичная (осознанно единый мир, без переключателя); адаптив от мобильного;
  анимации отключаются под `prefers-reduced-motion`.

### Конфиг и локальная разработка

- Команды: `npm run dev` (Vite по моку), `npm run build` (`vue-tsc` + сборка), `npm run typecheck`.
- **Dev-proxy** `/api` → Go-бэк (env `VITE_API_TARGET`, дефолт `http://localhost:8080`) — против CORS;
  в проде фронт и API за одним доменом/реверс-прокси.
- **CORS на бэке:** для standalone-фронта Go-API отдаёт `Access-Control-Allow-Origin`.
- Секретов на фронте нет — ключ LLM только на бэке. Идентификация — заголовок `X-Client-ID`
  (browser-token в localStorage, ставится в `client.ts` на каждый запрос).

### Соответствие контракту §4.A (реализовано)

Слой данных переведён на `vehicles`/`alerts`. Что к какому эндпоинту привязано:

| Фронт-модуль                                 | Эндпоинт §4.A                                       | Статус |
|----------------------------------------------|-----------------------------------------------------|--------|
| `useGarage` / `client.listVehicles`          | `GET`/`POST /api/v1/vehicles`, `GET /vehicles/{id}` | ✅ |
| `AddCarModal` / `client.resolveVin`          | `POST /api/v1/vin/resolve`                          | ✅ |
| `RecommendationsView` / `client.getAlerts`   | `GET /api/v1/vehicles/{id}/alerts`                  | ✅ |
| обновление пробега / `client.updateOdometer` | `PATCH /api/v1/vehicles/{id}/odometer` (не уменьшать) | ✅ |
| `client.addServiceEvent`                     | `POST /api/v1/vehicles/{id}/service-events`         | ⏳ метод есть, UI-кнопки «отметить ТО» пока нет |
| `client.getMe` + `X-Client-ID`               | `GET /api/v1/me` + заголовок на каждом запросе      | ✅ |

**Осталось при появлении бэка (`backend/`):** сверить с Dev 1 точный казинг/поля JSON (сейчас
camelCase по ADR §5.2), выключить `VITE_USE_MOCK`, добавить UI «отметить ТО выполненным».

## 6. Ответственность (кто чем владеет)

| Dev | Слой | Стек | Владеет | НЕ трогает |
|-----|------|------|---------|-----------|
| 1 | Go-сервер / платформа + интеграции | Go | `api/`, `store/`, `domain/` (стюард), `sink/` (порт), `bitrix/`, `main.go` | `recommender/`, `vin/`, `engine/`, `data/`, `frontend/` |
| 2 | Фронтенд-сервер | Vue/TS | `frontend/` | весь `backend/` |
| 3 | **Рекомендательный слой** (VIN + пробег + «что и когда обслужить») | Go | `recommender/`, `vin/`, `engine/`, `data/` | `api/`, `store/`, `sink/`, `main.go`, `frontend/` |

**Bitrix-коннектор (`bitrix/`)** — Dev 1, за портом `Sink` (вебхук, без OAuth); включается на b2b.

**Правило против merge-конфликтов:** `domain/`, `sink/` (порт) и контракты заморожены на фазе 0.
`main.go` правит **только Dev 1** (Dev 3 отдаёт конструкторы `Recommender`/`VINProvider`).

## 7. Git-стратегия

- Моно-репо `servys`, одна ветка на человека: `dev1-backend`, `dev2-frontend`, `dev3-recommendations`.
- Мержим в `main` **рано и часто** — пакетное разделение почти исключает конфликты.
- Первый общий коммит = скелет репо + контракты + моки. От него все ответвляются.

## 8. Тайминг (~5 часов)

| Время | Что | Кто |
|-------|-----|-----|
| 0:00–0:40 | Заморозить контракты, скелет (go.mod, dirs, `npm create vue@latest`, миграции SQLite, YAML-заглушка, моки) | Все вместе |
| 0:40–3:30 | Параллельная работа по таблице ответственности | Каждый в своём потоке |
| 3:30–4:30 | Интеграция: заглушки → живые YAML+LLM/Drom; фронт → живой API; e2e по одной демо-машине | Все |
| 4:30–5:00 | Полировка демо + буфер | Все |

## 9. Скоуп MVP и роадмап

**MVP (сегодня, b2c) — фронт + бэк + рекомендационный слой:**
- гараж пользователя (browser-token), добавление авто (VIN опц. / ручной ввод — основной путь);
- обновление пробега + история; движок напоминаний + шедулер; alerts/статусы в приложении;
- рекомендации: гибрид YAML (демо) + LLM (провайдер — выбор Dev 3) для неизвестных моделей;
- **одна демо-машина отрабатывает железобетонно**;
- **Bitrix НЕ входит**.

**Роадмап (аддитивно, за теми же портами):**
- **b2b** *(минимальный слой реализован, Dev 1)*: движок удержания для СТО на вебхуке — подключение
  портала, чтение автопарка из CRM (`crm.contact.list`), ретеншн-дела (`crm.activity.todo.add`,
  идемпотентно). Эндпоинты `/api/v1/b2b/*`. Детали — `backend/b2b/README.md`. Дальше — CRM-движок/сделки
  удержания, multi-tenant, OAuth-приложение. Механика/идемпотентность — ADR-001 §5.8–5.9.
- **auth** *(реализовано, Dev 1)*: единый аккаунт + точки входа (email/Telegram) + JWT access/refresh +
  переключение b2c/b2b; b2b-эндпоинты закрыты (per-СТО по аккаунту, `scan-all` по admin-токену).
  Контракт — `backend/auth/README.md`.
- **фронт b2b-панель + auth-флоу** *(🔄 in progress, агент, ветка `feat/b2b-frontend`)*: экраны СТО,
  вход/регистрация, хранение токенов + Bearer, авто-refresh, переключатель контекста. В `main` не смёржено.

**Отложено:** OAuth/Marketplace и вход через Bitrix, привязка нескольких точек входа (linking),
RS256/JWKS, перевод b2c с гостевого `X-Client-ID` на аккаунт, календарь, CRM-сущности/сделки,
внешний готовый датасет, PostgreSQL/очереди.

## 10. Как команда работает с этим документом и с Claude Code

- Документ и `CLAUDE.md` **закоммичены в репо** → у каждого появляются после `git pull`.
- Каждый участник запускает Claude Code **внутри своего клона `servys/`**; `CLAUDE.md` в корне
  подхватывается автоматически — «натравливать» вручную не нужно. Достаточно сказать, кто он
  (Dev 1/2/3), и работать от своего чек-листа.
- Архитектурные решения фиксируем в `docs/adr/` (см. `docs/adr/README.md`).
