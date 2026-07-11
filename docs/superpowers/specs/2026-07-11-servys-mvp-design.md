# servys — дизайн MVP и командный процесс

- **Дата:** 2026-07-11 (ред. после ревью ADR-001)
- **Команда:** 3 разработчика
- **Срок:** ~полдня (~5 рабочих часов)
- **Статус:** утверждён, источник правды для реализации
- **Связанный ADR:** `docs/adr/ADR-001-car-maintenance-mvp.md`

> **Ревизия (сведено с ADR-001):** изначально делаем **фронт + бэк + рекомендационный слой (b2c)**.
> Bitrix-интеграция — **только на уровне b2b и отложена** (под вопросом). Данные — **гибрид YAML+LLM**.
> Мульти-авто / юзеры / история пробега / шедулер — **оставлены**. Порты `Recommender`/`Sink`/`Tenant`
> сохранены; функционал из ADR встроен за ними.

---

## 1. Продукт

**servys** — сервис превентивного обслуживания авто. По автомобилю (VIN опционально / ручной ввод)
и пробегу подсказывает регламент и типовые поломки, привязанные к пробегу. Пользователь ведёт свой
гараж, обновляет пробег и видит статусы/напоминания в приложении.

**Изначальная сборка (b2c):** фронт + бэк + **рекомендационный слой**. Без Bitrix.

### Два режима (tenant type)

- **b2c** *(делаем сейчас)* — частный автовладелец. Всё в веб-приложении, Bitrix не участвует.
- **b2b** *(под вопросом, позже)* — автосервис/дилер: та же основа **+** интеграция с их Bitrix24
  (задачи `tasks.task.add`, далее CRM). Bitrix осмыслен **только тут**: задача ставится на сотрудника
  сервиса; частнику (b2c) ставить некому.

## 2. Архитектура

Модульный монолит на Go + отдельный веб-фронт (Vue/Vite). Один Go-процесс: REST API + SQLite +
шедулер (+ outbox-воркер на этапе b2b). Два процесса на деве: фронт `:5173`, бэк `:8080`.

**Стек бэка:** `net/http` + `go-chi/chi`, `database/sql` + `modernc.org/sqlite`, SQL-миграции при
старте, явный `App`/constructor-wiring без DI-фреймворка.

### Порты (швы)

- **`Recommender`** — рекомендационный слой: правила из `maintenance_rules.yaml` (верифиц., демо)
  + LLM (Claude) для моделей вне YAML. **Не выдумывает интервалы**: нет правила → `REGULATION_NOT_FOUND`.
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
│   ├── engine/               # Dev 1 — движок напоминаний + шедулер
│   ├── sink/                 # Dev 1 — порт Sink (+ outbox); Bitrix-реализация отложена (b2b)
│   ├── recommender/          # Dev 3 — рекомендационный слой: YAML-правила + LLM (Claude), impl Recommender
│   ├── vin/                  # Dev 3 — VINProvider (адаптер Drom)
│   ├── bitrix/               # (b2b, отложено) — Sink через tasks.task.add
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
- **Dev 3** реализует боевые: YAML+LLM (Claude) и адаптер Drom. Подменяет заглушки.
- Модель Claude — на этапе реализации (скилл `claude-api`), ключ через env `ANTHROPIC_API_KEY`.

### C. Исходящий порт `Sink` — **b2b, отложено**

```go
package sink

type Reminder struct {
    Tenant domain.Tenant
    Alert  domain.Alert
}

type Sink interface {
    Deliver(ctx context.Context, r Reminder) error
}
```

В b2c не задействован (напоминания в приложении). Реализация в `bitrix/` (`tasks.task.add`) —
на этапе b2b. Детали механизма/идемпотентности/outbox — в ADR-001, §5.8–5.9.

## 5. Архитектура фронта (Dev 2)

> ⚠️ **Требует переalign под §4.A:** секция ниже описана вокруг `POST /api/v1/recommendations`.
> Контракт сменился на модель `vehicles`/`alerts` — Dev 2 обновляет `types/api.ts`, `api/client.ts`
> и мок. Слоистая структура ниже остаётся валидной, меняются только формы данных и вызовы.

Фронт — тонкий слой над контрактом A: получить `car` из формы, дёрнуть API, отрисовать `items`.
Вся сложность — в чётком разделении слоёв, чтобы работать против мока и переключиться на живой
API сменой env, а не правкой кода.

**Стек:** Vue 3 (`<script setup>` + Composition API) + **TypeScript (strict)** + Vite.
Состояние — локальные composables (Pinia/Vuex для одного экрана избыточны). Сеть — нативный
`fetch` в тонкой обёртке. Пакетный менеджер — npm.

### Раскладка `frontend/`

```
frontend/
├── index.html
├── vite.config.ts          # dev-proxy /api → Go-бэк (локально обходим CORS)
├── .env / .env.production  # VITE_API_BASE_URL, VITE_USE_MOCK
├── mock/recommendations.json
└── src/
    ├── main.ts
    ├── App.vue             # layout + оркестрация экрана
    ├── types/api.ts        # TS-типы контракта A (1:1 с mock.json) — единственный источник форм данных
    ├── api/client.ts       # fetch-обёртка: postRecommendations(car), health(); mock/live-переключатель
    ├── composables/
    │   └── useRecommendations.ts   # статус запроса idle|loading|success|error + data
    ├── components/
    │   ├── CarForm.vue             # ввод make/model/year/mileage + валидация
    │   ├── RecommendationList.vue  # список + состояния loading/empty/error
    │   └── RecommendationCard.vue  # карточка item: severity/status → бейдж/цвет
    └── ui/
        ├── tokens.css              # CSS-переменные: палитра, отступы, радиусы, типографика
        └── status.ts               # маппинг severity/status → лейбл + цвет (единое место)
```

### Слои и ответственность

1. **`types/api.ts` — контракт как типы.** Ровно повторяет ответ `POST /api/v1/recommendations`
   (union-литералы для `category`/`severity`/`status`). Всё типизируется от него; расхождение
   с `mock.json` правим здесь — это единственный источник правды по форме данных на фронте.
2. **`api/client.ts` — единственная точка сети.** Компоненты не знают про `fetch`/URL. Базовый
   URL из `VITE_API_BASE_URL`; флаг `VITE_USE_MOCK=1` отдаёт мок без сети (Dev 2 работает, пока
   API не поднят). Переключение на живой API = смена env, а не кода.
   - **Загрузка мока:** `mock/recommendations.json` лежит вне `src/`/`public/`, поэтому Vite его
     сам не отдаёт — импортируем через alias (`@mock/recommendations.json`) в `vite.config.ts`.
     Один и тот же файл остаётся общим якорем контракта A с бэком.
   - **Варианты мока для состояний:** статический success-мок не позволяет проверить ветки
     `empty`/`error`. Клиент в mock-режиме умеет по флагу вернуть пустой `items`, ошибку или
     задержку (`VITE_MOCK_SCENARIO=success|empty|error|slow`) — иначе эти состояния не оттестировать.
   - **Runtime-guard:** TS-типы только compile-time; на живом ответе делаем лёгкую проверку формы
     (и defensive-рендер через optional chaining), чтобы дрейф контракта не давал тихий `undefined`.
3. **`useRecommendations.ts` — состояние экрана.** Держит статус (`idle/loading/success/error`)
   и результат; вся асинхронщина и обработка ошибок инкапсулированы здесь.
   - **Защита от гонки:** API синхронно ходит в LLM (секунды), двойной сабмит роняет порядок
     ответов — устаревший ответ отбрасываем через `AbortController`/request-token, в UI попадает
     только последний запрос.
   - **Ретрай:** храним параметры последней машины, чтобы кнопка «Повторить» пересылала тот же
     запрос, а не пустоту.
4. **Компоненты — презентационные.** `CarForm` эмитит `submit(car)`, `App.vue` зовёт composable,
   `RecommendationList`/`Card` рисуют. Бизнес-логики в компонентах нет.

### UI/UX

- **Один экран:** форма ввода → под ней список рекомендаций (или пустое/лоадинг/ошибка).
- **Дизайн-токены** в `tokens.css` (без тяжёлого UI-фреймворка): единая палитра, типографика,
  отступы. Светлая тема, адаптив от мобильного.
- **Семантика статусов — единый маппинг в `status.ts`:**
  - `severity`: `low` → нейтральный, `medium` → янтарный, `high` → красный;
  - `status`: `overdue` → красный «Просрочено», `due_soon` → янтарный «Скоро», `upcoming` → серый «Впереди».
  - Список сортируем: сперва `overdue`, затем `due_soon`, затем `upcoming`.
  - **Fallback обязателен:** неизвестный `severity`/`status` (недетерминизм LLM/дрейф бэка) →
    нейтральный бейдж, а не `undefined`/краш карточки; неизвестный `status` уходит в конец сортировки.
- **Состояния списка:** скелетон/спиннер при загрузке, дружелюбное пустое состояние, явная
  ошибка с кнопкой «Повторить».
- **Форма:** `year`/`mileage_km` — числа; `<input type="number">` отдаёт строку, а пустое поле →
  `NaN`, поэтому коэрсим к числу и отбиваем `NaN`/отрицательные до отправки. Кнопка `disabled`,
  пока форма невалидна **и** пока идёт запрос (иначе двойной сабмит — см. защиту от гонки выше).

### Конфиг и локальная разработка

- **Dev-proxy в `vite.config.ts`:** `/api` → `http://localhost:<go-port>`, чтобы в деве не ловить
  CORS; в проде фронт и API за одним доменом/реверс-прокси.
- **CORS на бэке:** для standalone-фронта Go-API отдаёт `Access-Control-Allow-Origin`
  (согласовать с Dev 1 на фазе 0). Секретов на фронте нет — ключ Claude только на бэке.

### Швы под рост (в MVP не реализуем, но не мешаем)

- API-слой изолирован → мобилку/бота позже прикрутят к тем же типам из `types/api.ts`.
- `tenant` фронту в b2c-MVP не нужен; при появлении b2b добавится в запрос/контекст, не ломая экран.
- Пробег сейчас одно число в форме; когда бэк перейдёт на временной ряд — добавится отдельный
  виджет истории, форма ввода не меняется.

## 6. Ответственность (кто чем владеет)

| Dev | Слой | Стек | Владеет | НЕ трогает |
|-----|------|------|---------|-----------|
| 1 | Go API + БД + движок + порты + wiring | Go | `api/`, `store/`, `domain/`, `engine/`, `sink/` (порт), `main.go` | `recommender/`, `vin/`, `bitrix/`, `frontend/` |
| 2 | Vue-фронт (standalone) | Vue/TS | `frontend/` | `backend/` |
| 3 | Рекомендационный слой + VIN | Go | `recommender/`, `vin/`, `data/*.yaml` | `api/`, `store/`, `main.go`, `frontend/` |

**Bitrix-синк (`bitrix/`)** — этап **b2b, отложено** (позже Dev 3).

**Правило против merge-конфликтов:** `domain/`, `sink/` (порт) и контракты заморожены на фазе 0.
`main.go` правит **только Dev 1** (Dev 3 отдаёт конструкторы `Recommender`/`VINProvider`).

## 7. Git-стратегия

- Моно-репо `servys`, одна ветка на человека: `dev1-backend`, `dev2-frontend`, `dev3-recommender`.
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
- рекомендации: гибрид YAML (демо) + LLM (Claude) для неизвестных моделей;
- **одна демо-машина отрабатывает железобетонно**;
- **Bitrix НЕ входит**.

**Роадмап (аддитивно, за теми же портами):**
- **b2b** *(под вопросом)*: включаем `Sink`/`bitrix/` — задачи `tasks.task.add`, далее CRM-движок
  удержания, multi-tenant, OAuth-приложение. Механика/идемпотентность — ADR-001 §5.8–5.9.

**Отложено:** Bitrix целиком (до b2b), OAuth/Marketplace, календарь, CRM-сущности, внешний
готовый датасет (только исследование), production-auth, PostgreSQL/очереди.

## 10. Как команда работает с этим документом и с Claude Code

- Документ и `CLAUDE.md` **закоммичены в репо** → у каждого появляются после `git pull`.
- Каждый участник запускает Claude Code **внутри своего клона `servys/`**; `CLAUDE.md` в корне
  подхватывается автоматически — «натравливать» вручную не нужно. Достаточно сказать, кто он
  (Dev 1/2/3), и работать от своего чек-листа.
- Архитектурные решения фиксируем в `docs/adr/` (см. `docs/adr/README.md`).
