# Расширение каталога обслуживания + «регламент/отзывы» + полный чек-лист

**Дата:** 2026-07-11
**Статус:** согласован (продукт), в работе
**Затрагивает слои:** Dev 3 (`recommender/`, `engine/`, `data/`), Dev 1 (`domain/`, `api/`), Dev 2 (`frontend/`)

## Проблема

Приложение показывает 2–3 позиции обслуживания (в HEAD `maintenance_rules.yaml` всего 2 правила
для KIA K3; словарь LLM в `knowledge.go` — 10 компонентов). Пользователь хочет **полный вывод
всех основных запчастей (масло, фильтры, тормоза и т.д.) плюс дополнительные**, а также отдельно
показывать **народные данные из отзывов** («ГРМ по отзывам ходит ~50к, меняют на 45к»), рядом с
официальным регламентом.

## Решение — три оси

1. **Каталог компонентов (≥40)** — единый источник правды `code → {название RU, категория, диапазон интервала}`.
2. **Две категории важности:** `primary` (основные) / `secondary` (дополнительные) — из каталога.
3. **Два основания знания на одной карточке:** регламент (`interval_km`) + опциональный блок **отзывов**
   (`community`: реальный интервал + текст + источник + число упоминаний).
4. **Полный чек-лист:** движок эмитит и статус «в норме» (OK), а не только Soon/Due/Overdue.

---

## Контракт данных (единый для всех слоёв)

### Каталог компонентов — `backend/recommender/catalog.go` (Dev 3, НОВЫЙ)

Единый источник правды. Заменяет мапу `components` в `knowledge.go`.

```go
type ComponentCategory = string // "primary" | "secondary"

type ComponentSpec struct {
    Code       string
    TitleRU    string
    Category   ComponentCategory
    MinKm, MaxKm int // разумный диапазон интервала для валидации LLM
}

// Catalog — map[code]ComponentSpec, ≥40 позиций (см. таблицу ниже).
func Lookup(code string) (ComponentSpec, bool)
func TitleFor(code string) string          // «» если нет
func CategoryFor(code string) string       // "" если нет
func ValidationRanges() map[string][2]int  // для knowledge.go
```

`knowledge.go`: `var components` заменяется на `catalog.ValidationRanges()`; валидация интервала
и «unknown component» продолжает работать, но список — из каталога.

### Доменные типы — `backend/domain/domain.go` (Dev 1, замороженный контракт — правка согласована)

```go
// CommunityNote — данные из отзывов/форумов владельцев (НЕ официальный регламент).
type CommunityNote struct {
    RealIntervalKm int    // что советуют владельцы (напр. 45000); 0 — не задан
    Note           string // человекочитаемый вывод сообщества
    Source         string // URL источника или "demo"
    Reports        int    // сколько отзывов/источников (сила консенсуса); 0 — не задан
}

// Rule += :
    Category  string          // "primary" | "secondary"
    Community *CommunityNote   // nil, если данных из отзывов нет

// Alert += :
    Category  string
    Community *CommunityNote

// НОВАЯ константа типа:
    AlertMaintenanceOK = "MAINTENANCE_OK"

// НОВЫЕ константы категории:
    CategoryPrimary   = "primary"
    CategorySecondary = "secondary"
```

### Движок — `backend/engine/engine.go` (Dev 3)

`BuildAlerts`:
- **Эффективный интервал** = `IntervalKm`, если > 0; иначе `Community.RealIntervalKm`, если > 0; иначе
  правило по-км не считаем (пропуск, как сейчас для интервала по времени).
- **Не пропускать OK:** статус `"OK"` теперь эмитится как `Type = MAINTENANCE_OK`, `Severity = low`.
- Прокидывать `Category` и `Community` из `Rule` в `Alert`.
- `severityFor`: OK → low.

Существующие пороги `EvaluateByOdometer` не трогаем.

### API — `backend/api/api.go` (Dev 1), `alertJSON`

Добавить поля (snake_case):
```go
"category": a.Category,
"community": communityJSON(a.Community), // nil → поле опускаем или null
```
`communityJSON`:
```json
"community": { "real_interval_km": 45000, "note": "...", "source": "demo", "reports": 12 }
```

### Фронт-контракт — `frontend/src/types/api.ts` (Dev 2)

```ts
export type MaintenanceCategory = 'primary' | 'secondary'

export interface CommunityNote {
  realIntervalKm: number
  note: string
  source: string
  reports: number
}

// Alert += :
  category: MaintenanceCategory
  community?: CommunityNote | null

// AlertType += 'MAINTENANCE_OK'
```
`client.ts` `getAlerts` (живой ветка): маппит `category` (fallback `'primary'`) и `community`
(snake_case → camelCase, `null` → `undefined`). Существующий `status = type.replace('MAINTENANCE_','')`
уже даст `OK` для `MAINTENANCE_OK` — статус «В норме» появится автоматически.

---

## Каталог компонентов (seed ≥40)

Категория: **P** = primary (основные), **S** = secondary (дополнительные). Интервал — ориентир для demo.

**Основные (primary):** engine_oil (Моторное масло, 10000), engine_oil_filter (Масляный фильтр, 10000),
engine_air_filter (Воздушный фильтр двигателя, 30000), cabin_filter (Салонный фильтр, 15000),
fuel_filter (Топливный фильтр, 40000), spark_plugs (Свечи зажигания, 40000),
brake_fluid (Тормозная жидкость, 40000), brake_pads_front (Передние тормозные колодки, 40000),
brake_pads_rear (Задние тормозные колодки, 60000), brake_discs_front (Передние тормозные диски, 80000),
brake_discs_rear (Задние тормозные диски, 100000), engine_coolant (Охлаждающая жидкость, 60000),
transmission_fluid (Масло коробки передач, 60000), timing_belt (Ремень ГРМ, 90000),
accessory_belt (Ремень навесного оборудования, 60000), battery (Аккумулятор, 60000),
tires (Шины, 60000), wiper_blades (Щётки стеклоочистителя, 20000).

**Дополнительные (secondary):** ignition_coils (Катушки зажигания, 100000),
power_steering_fluid (Жидкость ГУР, 80000), differential_fluid (Масло редуктора, 60000),
transfer_case_fluid (Масло раздатки, 60000), timing_chain (Цепь ГРМ, 150000),
water_pump (Помпа охлаждения, 90000), thermostat (Термостат, 100000),
serpentine_tensioner (Натяжитель ремня, 90000), glow_plugs (Свечи накаливания, 100000),
dpf (Сажевый фильтр, 120000), pcv_valve (Клапан PCV, 80000), oxygen_sensor (Лямбда-зонд, 100000),
shock_absorbers (Амортизаторы, 80000), suspension_bushings (Сайлентблоки, 90000),
cv_joints (ШРУСы и пыльники, 90000), wheel_bearings (Ступичные подшипники, 100000),
wheel_alignment (Развал-схождение, 30000), ac_refrigerant (Хладагент кондиционера, 60000),
ac_system_check (Кондиционер: проверка, 60000), washer_fluid (Жидкость стеклоомывателя, 10000),
headlight_bulbs (Лампы фар, 60000), exhaust_system (Выхлопная система, 100000),
clutch (Сцепление, 120000), engine_mounts (Подушки двигателя, 0 — только отзывы).

Ranges для валидации: `[max(1000, интервал/4), min(300000, интервал*3)]`; для `engine_mounts` — `[10000, 200000]`.

## Demo-данные — `backend/data/maintenance_rules.yaml` (Dev 3)

Профиль KIA K3 2020 (1353cc/130hp) — все ≥40 компонентов каталога. Парсер `NewYAML` в
`recommender.go` расширить **плоскими** ключами (без вложенности):

```yaml
- code: timing_belt
  category: primary            # опц.; если пусто — берётся из каталога
  operation: replace
  interval_km: 90000
  lead_km: 2000
  verified: false
  source: demo
  community_km: 60000          # → CommunityNote.RealIntervalKm
  community_note: "Владельцы отмечают растяжение к 60 000 км — меняют раньше регламента."
  community_source: demo
  community_reports: 18
```
`title` в YAML можно опускать — берётся из каталога (`TitleFor`), YAML переопределяет при наличии.

Demo-отзывы (≥5): engine_oil (real 7500, «сокращают до 7 500 км в городе», 34),
transmission_fluid (real 60000, «меняют ATF к 60к вопреки “на весь срок”», 21),
ignition_coils (real 70000, «пропуски зажигания к 70к», 15),
timing_belt (real 60000, 18), brake_discs_front (real 50000, «быстрый износ передних дисков», 12),
engine_mounts (interval_km 0, community_km 90000, «стук подушек к 90к», 9 — пример «только отзывы»).

## Раскладка фронта — `RecommendationsView.vue` (Dev 2)

- Две секции: **«Основные»** (`category==='primary'`) и **«Дополнительные»** (`category==='secondary'`),
  каждая — своя карусель/группа, сортировка внутри по срочности (`byUrgency`).
- Легенда «Срочно / Скоро / В норме» — по всем алертам (уже есть; заработает счётчик «В норме»).
- В карточке под строкой регламента — блок **«По отзывам»** при `community`:
  иконка/бейдж, `Меняют на {realIntervalKm} км`, текст `note`, пометка «не регламент» и `{reports} отзывов`.
- `mock/alerts.json` обновить под новый контракт: ≥40 позиций с `category`, часть с `community`,
  коды/названия — из каталога выше; демонстрирует обе секции и «в норме» в mock-режиме.

## Порядок и верификация

- **Backend-агент** (Dev 3 + Dev 1): catalog.go, domain, engine, recommender.go парсер, knowledge.go,
  api.go, yaml, тесты. Верификация: `cd backend && go build ./... && go test ./...`.
- **Frontend-агент** (Dev 2): types, client, RecommendationsView, mock/alerts.json, тесты.
  Верификация: сборка/типы (`npm run build` или `tsc --noEmit`) + `npm run test` если есть.
- Агенты независимы: реализуют против этого контракта; общих файлов нет.

## Вне скоупа (YAGNI)

- Живой LLM-поиск отзывов (Ollama+SearXNG) — модель данных готовим, боевой парсинг «отзывов» из
  пайплайна LLM оставляем на следующий шаг; сейчас отзывы — demo-данные.
- Реальные регламенты по маркам, кроме демо-профиля KIA K3.
- baseline из service_events (по-прежнему от нового, MVP).
