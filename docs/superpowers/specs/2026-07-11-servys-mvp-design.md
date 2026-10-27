# servys — дизайн MVP и командный процесс

- **Дата:** 2026-07-11 (ред. после ревью ADR-001)
- **Команда:** 3 разработчика
- **Срок:** ~полдня (~5 рабочих часов)
- **Статус:** утверждён, источник правды для реализации
- **Связанный ADR:** `docs/adr/ADR-001-car-maintenance-mvp.md`

> **Ревизия 2026-10-26:** продукт — личный гараж в Telegram и MAX. Bitrix, кабинет СТО, sink и гостевой заголовок сняты. §4 описывает живой код. ADR-001 — история решений; при расхождении по HTTP и портам права эта спека.

---

## Статус реализации (обновлено 2026-10-26)

| Слой | Владелец | Статус |
|------|----------|--------|
| Go-платформа: HTTP, SQLite, auth Telegram/MAX, боты | Dev 1 | в коде |
| Фронт мини-приложения | Dev 2 | в коде, живой API через Bearer |
| Рекомендации: YAML, типовой регламент, VIN Drom/NHTSA, Ollama по `LLM_MODE` | Dev 3 | в коде, шов `recommender.Advisor` |

---

## 1. Продукт

**servys** — сервис превентивного обслуживания авто. По автомобилю (VIN опционально / ручной ввод)
и пробегу подсказывает регламент и типовые поломки, привязанные к пробегу. Пользователь ведёт свой
гараж, обновляет пробег и видит статусы/напоминания в приложении.

**Изначальная сборка (b2c):** фронт + бэк + **рекомендационный слой**. Без Bitrix.

Один режим: личный гараж. Вход из Telegram или MAX. Bitrix и кабинет СТО в продукт не входят.

## 2. Архитектура

Модульный монолит на Go и отдельный Vue-фронт. Один Go-процесс: REST, SQLite, вебхуки ботов. Без DI-фреймворка: `main` собирает `store`, `recommender.Advisor`, `vin.VINProvider`, `auth` и `messenger`.

**Стек бэка:** `net/http` + `go-chi/chi`, `database/sql` + `modernc.org/sqlite`. Миграции при старте.

### Порты

- **`recommender.Advisor`** — единственный шов платформы и рекомендаций: авто и история работ → `[]domain.Alert`.
- **`recommender.Recommender`** — правила для авто. Цепочка: точный профиль из YAML, при `LLM_MODE=live` поиск источников с бюджетом времени, затем типовой регламент легкового авто.
- **`vin.VINProvider`** — VIN → марка, модель, год, объём, мощность. Сначала Drom, при пустом ответе NHTSA. Ошибка → ручная форма.
- **`messenger.Sender`** — исходящее сообщение в Telegram или MAX.
- **`auth.Store`** — аккаунт, identity, refresh. Реализация в `store`.

Движок `engine.BuildAlerts` считает статус по каждому правилу с интервалом в км. Пробег хранится и текущим значением, и рядом `odometer_readings`.

Источник идентификации авто: `manual`, `drom` или `nhtsa` (`domain.Source*`). Владелец машины — `Vehicle.AccountID`.

JSON HTTP — snake_case (`engine_cc`, `mileage_km`, `rule_code`, `due_at_km`, `body_type`, `identification_source`). В TypeScript клиент переводит это в camelCase в `frontend/src/api/client.ts`.

## 3. Раскладка репозитория

```
servys/
├── backend/
│   ├── main.go               # wiring
│   ├── api/                  # HTTP: auth, vehicles, vin, alerts, history, webhooks
│   ├── auth/                 # Telegram, MAX, JWT
│   ├── domain/               # Account, Vehicle, Rule, Alert
│   ├── store/                # SQLite
│   ├── messenger/            # вебхуки и исходящие сообщения ботов
│   ├── maintenance/          # разбор текста сервисной книжки в работы
│   ├── engine/               # срок по пробегу и правилам
│   ├── recommender/          # Advisor, YAML, каталог, поиск, Ollama
│   ├── vin/                  # Drom и NHTSA
│   └── data/maintenance_rules.yaml
├── frontend/                 # Vue SPA
└── docs/
```

## 4. Замороженные контракты

Швы между разработчиками. Согласуем и фиксируем на фазе 0.

### A. HTTP API-контракт (Go-бэк ↔ Vue-фронт) — модель `vehicles`/`alerts`

> ⚠️ **Смена контракта:** заменяем `POST /api/v1/recommendations` на модель из ADR-001.
> Фронт Dev 2 строился на `/recommendations` + `mock/recommendations.json` — **нужен переalign**
> под эндпоинты ниже (переименовать мок, обновить `types/api.ts`).

```
GET   /api/v1/health
POST  /api/v1/auth/telegram
POST  /api/v1/auth/max
POST  /api/v1/auth/refresh
POST  /api/v1/auth/logout
GET   /api/v1/auth/me                              # Bearer
POST  /api/v1/auth/dev                             # только SERVYS_DEV_LOGIN=1
POST  /api/v1/telegram/webhook
POST  /api/v1/max/webhook
POST  /api/v1/vin/resolve                          # Bearer
POST  /api/v1/vehicles
GET   /api/v1/vehicles
GET   /api/v1/vehicles/{id}
DELETE /api/v1/vehicles/{id}                       # 204, чужое -> 404
PATCH /api/v1/vehicles/{id}/odometer               # тело {"mileage_km"}
POST  /api/v1/vehicles/{id}/service-events         # тело {"rule_code","odometer","performed_at"}
GET   /api/v1/vehicles/{id}/service-events
POST  /api/v1/vehicles/{id}/history
POST  /api/v1/vehicles/{id}/history-file
GET   /api/v1/vehicles/{id}/history
GET   /api/v1/vehicles/{id}/alerts
```

Гараж за `Authorization: Bearer`. Машина в JSON: `vin`, `make`, `model`, `year`, `engine_cc`, `power_hp`, `color`, `body_type`, `identification_source` (`manual` | `drom` | `nhtsa`), `mileage_km`, `odometer_updated_at`. Создание принимает те же поля и `mileage_km`. Неизвестный `identification_source` сохраняется как `manual`.

Алерт: `rule_code`, `type`, `severity`, `title`, `description`, `due_at_km`, `category`, `basis`, опционально `community`. `type` — одна из констант `domain.Alert*`: `MAINTENANCE_SOON`, `MAINTENANCE_DUE`, `MAINTENANCE_OVERDUE`, `MAINTENANCE_OK`, `ODOMETER_UPDATE_REQUIRED`, `MAINTENANCE_HISTORY_REQUIRED`, `REGULATION_NOT_FOUND`. Клиент показывает статус `SOON` / `DUE` / `OVERDUE` / `OK`, снимая префикс `MAINTENANCE_`.

### B. Доменные типы + порты `Recommender` / `VINProvider` (Dev 1 ↔ Dev 3)

```go
package domain

type TenantType string
const ( TenantB2C TenantType = "b2c" )

const (
    SourceManual = "manual"
    SourceDrom   = "drom"
    SourceNHTSA  = "nhtsa"
)

type Vehicle struct {
    ID, AccountID        string
    VIN                  string
    Make, Model          string
    Year, EngineCC, PowerHP int
    Color, BodyType      string
    IdentificationSource string
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

type Recommender interface {
    Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}

type Advisor interface {
    Alerts(ctx context.Context, v domain.Vehicle, history []domain.ServiceEvent) ([]domain.Alert, error)
}

// package vin
type VINProvider interface {
    Resolve(ctx context.Context, vin string) (domain.Vehicle, error)
}
```

`api` вызывает `Advisor` и `VINProvider`, не `engine` и не YAML напрямую. LLM — локальная Ollama, модель на измеренном VPS `qwen2.5:0.5b` (ADR-003). Ключ облачной модели в код не кладётся.

Порт `sink.Sink` и пакет `bitrix` сняты. Напоминание живёт в приложении.

## 5. Архитектура фронта (Dev 2)

> Фронт — Vue 3 + TypeScript. Сеть в `src/api`, состояние в `composables`, экраны в `components`. Живой API включается `VITE_USE_MOCK=0`. Запросы гаража идут с Bearer. Snake_case ответа переводится в `types/api.ts` только в `client.ts`.

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
│   ├── vehicles.json         # пример ответа GET /vehicles (форма контракта); гараж стартует пустым
│   └── alerts.json           # алерты (GET /vehicles/{id}/alerts)
└── src/
    ├── main.ts
    ├── App.vue               # вход, затем вкладки Гараж, Книжка, Профиль
    ├── types/api.ts          # TS-типы контракта §4.A: Vehicle, Alert, VinResolveResult, …
    ├── api/client.ts         # сеть гаража: Bearer, snake_case → camelCase, mock-стор
    ├── composables/
    │   ├── useRecommendations.ts   # загрузка алертов: статус + защита от гонки (AbortController)
    │   └── useGarage.ts            # гараж поверх /vehicles: список, активная, add, updateOdometer
    ├── car3d/
    │   └── engine.ts               # WebGL-фабрика: 5 типов кузова, металлик-шейдер, вращение+drag
    ├── data/
    │   ├── presets.ts              # цвета-аватары, типы кузова, apiBodyToScene, hexToRgb
    │   └── vin.ts                  # офлайн-декодер VIN; живой путь — POST /vin/resolve
    ├── ui/
    │   ├── tokens.css              # токены бумажной колонки (фон #eeefe7, ширина до 430px)
    │   └── status.ts               # AlertStatus → лейбл/тон/порядок (+ fallback)
    └── components/
        ├── AuthView.vue            # экран «откройте из Telegram или MAX»
        ├── CarScene.vue            # обёртка над WebGL-движком (props type/color, destroy на unmount)
        ├── GaragePanel.vue         # список машин и добавление
        ├── GarageView.vue          # гараж: 3D, пробег, регламент
        ├── AddCarModal.vue         # VIN или ручной ввод, цвет, кузов, пробег
        ├── RecommendationsView.vue # сроки обслуживания
        ├── ServiceBookView.vue     # сервисная книжка
        └── ProfileView.vue         # профиль
```

### Слои и ответственность

1. **`types/api.ts` — контракт §4.A как типы.** `Vehicle`, `Alert` (статусы `OK|SOON|DUE|OVERDUE`),
   `VinResolveResult`, `CreateVehicleRequest`, `Me`. Единственный источник формы данных.
2. **`api/client.ts` — единственная точка сети гаража.** Эндпоинты §4.A. Заголовок `Authorization: Bearer`.
   `VITE_USE_MOCK=1` → in-memory стор. Моки `@mock/*.json`.
   Пересчёт статусов в моке: `recomputeStatus()` по `dueAtKm` относительно пробега. Нет срока — `OK`.
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

- **Три вкладки:** Гараж, Книжка, Профиль. До входа — экран «откройте из Telegram или MAX».
- **Гараж:** список машин и 3D-аватар активной. Клик делает машину активной: цвет и кузов меняют сцену, сроки пересобираются.
- **Добавление:** модалка с мини-3D. VIN → `POST /vin/resolve` (офлайн-декодер помечает источник `manual`), затем пробег, кузов и цвет → `POST /vehicles`.
- **3D-машина:** WebGL low-poly, 5 типов кузова, цвет из `vehicle.color`, вращение и drag.
- **Сроки:** карточки алертов. `PATCH /odometer` пересобирает их.
- **Книжка:** текст или файл истории и подтверждённые работы.
- **Статусы (`status.ts`):** `OVERDUE` и `DUE` — crit, `SOON` — warn, `OK` — calm. Неизвестный тип — «—». Сортировка по срочности.
- **Тема:** бумажный фон `#eeefe7`, колонка до 430px. Анимации отключаются под `prefers-reduced-motion`.

### Конфиг и локальная разработка

- Команды: `npm run dev` (Vite по моку), `npm run build` (`vue-tsc` + сборка), `npm run typecheck`.
- **Dev-proxy** `/api` → Go-бэк (env `VITE_API_TARGET`, дефолт `http://localhost:8080`) — против CORS;
  в проде фронт и API за одним доменом/реверс-прокси.
- **CORS на бэке:** для standalone-фронта Go-API отдаёт `Access-Control-Allow-Origin`.
- Секретов на фронте нет. Вход — initData Telegram или MAX, сессия — access и refresh в `api/session.ts`.

### Соответствие контракту §4.A (реализовано)

Слой данных переведён на `vehicles`/`alerts`. Что к какому эндпоинту привязано:

| Фронт-модуль                                 | Эндпоинт §4.A                                       | Статус |
|----------------------------------------------|-----------------------------------------------------|--------|
| `useGarage` / `client.listVehicles`          | `GET`/`POST /api/v1/vehicles`, `GET /vehicles/{id}` | ✅ |
| удаление машины / `client.deleteVehicle`     | `DELETE /api/v1/vehicles/{id}` (инлайн-подтверждение) | ✅ |
| срочность в гараже / `useFleetAlerts`        | `GET /api/v1/vehicles/{id}/alerts` (точка при OVERDUE/DUE) | ✅ |
| `AddCarModal` / `client.resolveVin`          | `POST /api/v1/vin/resolve`                          | ✅ |
| `RecommendationsView` / `client.getAlerts`   | `GET /api/v1/vehicles/{id}/alerts`                  | ✅ |
| обновление пробега / `client.updateOdometer` | `PATCH /api/v1/vehicles/{id}/odometer` (не уменьшать) | ✅ |
| «Отметить выполненным» / `client.addServiceEvent` | `POST /api/v1/vehicles/{id}/service-events`    | ✅ (кнопка на карточке + попап с датой/заметкой) |
| `useAuth` / `api/auth.ts`                    | `POST /api/v1/auth/telegram`, `/max`, `/refresh`, `GET /auth/me` | ✅ |

Провод — snake_case из §4.A. `types/api.ts` держит camelCase после `client.ts`.

## 6. Ответственность (кто чем владеет)

| Dev | Слой | Стек | Владеет | НЕ трогает |
|-----|------|------|---------|-----------|
| 1 | Платформа | Go | `api/`, `store/`, `domain/`, `auth/`, `messenger/`, `maintenance/`, `main.go` | `recommender/`, `vin/`, `engine/`, `data/`, `frontend/` |
| 2 | Фронтенд | Vue/TS | `frontend/` | весь `backend/` |
| 3 | Рекомендации | Go | `recommender/`, `vin/`, `engine/`, `data/` | `api/`, `store/`, `main.go`, `frontend/` |

`domain` и HTTP §4.A меняются только общим согласием. `main` подключает конструкторы Dev 3 и не содержит правил обслуживания.

## 7. Git

- Ветки: `dev1-backend`, `dev2-frontend`, `dev3-recommendations`. Интеграция — `main`.

## 8. Скоуп

Личный гараж в Telegram и MAX:

- машина по VIN или вручную, пробег, история работ текстом или файлом;
- сроки в приложении только из найденных отзывов и страниц: при `LLM_MODE=live` поиск и Ollama извлекают цитату с интервалом; готового профиля в данных нет;
- модель на VPS — `qwen2.5:0.5b` (ADR-003);
- вход: initData Telegram или MAX, JWT access/refresh. Локальный `POST /auth/dev` только при `SERVYS_DEV_LOGIN=1`.

Кабинет СТО, Bitrix, sink, гостевой заголовок и вход по паролю в продукт не входят. Июльский план на полдня записан в ADR-001 как история.

## 10. Как команда работает с этим документом и с Claude Code

- Документ и `CLAUDE.md` **закоммичены в репо** → у каждого появляются после `git pull`.
- Каждый участник запускает Claude Code **внутри своего клона `servys/`**; `CLAUDE.md` в корне
  подхватывается автоматически — «натравливать» вручную не нужно. Достаточно сказать, кто он
  (Dev 1/2/3), и работать от своего чек-листа.
- Архитектурные решения фиксируем в `docs/adr/` (см. `docs/adr/README.md`).
