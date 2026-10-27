# servys — фронт (Dev 2)

Standalone веб-приложение на **Vue 3 + TypeScript + Vite**. Личный гараж: машина, пробег, регламент.
Ходит в Go-API по Bearer. Без сети работает mock (`VITE_USE_MOCK=1`).

Источник правды по контракту — спека `docs/superpowers/specs/2026-07-11-servys-mvp-design.md` §4.

## Запуск

```bash
npm install
npm run dev        # http://localhost:5173, режим мока (VITE_USE_MOCK=1)
npm run typecheck  # vue-tsc --noEmit
npm run build      # typecheck + production-сборка в dist/
```

Переключение mock ↔ live — через env, а не код:

```
VITE_USE_MOCK=1        # мок без сети (dev)
VITE_USE_MOCK=0        # живой API
VITE_API_BASE_URL=     # база API (пусто → тот же origin, dev-proxy /api)
VITE_API_TARGET=       # куда dev-proxy шлёт /api (дефолт http://localhost:8080)
```

## Структура

```
src/
├── types/api.ts        контракт §4.A как TS-типы (Vehicle, Alert, VinResolveResult)
├── api/client.ts       эндпоинты §4.A, Bearer, mock-стор; snake_case провода → camelCase типов
├── composables/        useRecommendations (алерты, AbortController), useGarage, useAuth
├── car3d/engine.ts     WebGL: 5 типов кузова, вращение и drag
├── data/               presets (цвет и кузов), vin (офлайн-декодер, источник manual)
├── ui/                 tokens.css (бумажная колонка), status.ts (OK/SOON/DUE/OVERDUE, иначе «—»)
├── components/         AuthView, GarageView, ServiceBookView, ProfileView, AddCarModal, RecommendationsView
└── App.vue             вход, затем вкладки Гараж, Книжка, Профиль
```

## Дев-инструменты

- Демо-VIN для «Добавить машину»: `JTDBE32K700261000` (Toyota Camry), `WVWZZZ1KZAW000001` (VW Golf),
  `5UXWX7C5XBA000001` (BMW X5). В офлайне любой ввод от 11 символов декодируется локально, источник — `manual`.

## Статус контракта

Слой данных на контракте **§4.A**: Bearer, `vehicles`/`alerts`, `vin/resolve`, `odometer`,
`service-events`, `history`. В JSON API — snake_case. В TypeScript — camelCase, перевод в `api/client.ts`.
`identification_source`: `manual` | `drom` | `nhtsa`. Карта «модуль ↔ эндпоинт» — в спеке §5.

## Границы (см. CLAUDE.md)

Dev 2 владеет только `frontend/`. Контракты (§4) меняем по общему согласию.
