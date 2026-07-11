# servys — фронт (Dev 2)

Standalone веб-приложение на **Vue 3 + TypeScript + Vite**. Гараж пользователя с 3D-аватаром
машины и регламент обслуживания. Ходит в Go-API; пока API не поднят — работает по
`mock/recommendations.json`.

Источник правды по архитектуре — спека `docs/superpowers/specs/2026-07-11-servys-mvp-design.md` §5.

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
├── types/api.ts        контракт §4.A как TS-типы (Vehicle, Alert, VinResolveResult, …)
├── api/client.ts       единственная точка сети: эндпоинты §4.A, X-Client-ID, mock-стор
├── composables/        useRecommendations (алерты + защита от гонки), useGarage (/vehicles)
├── car3d/engine.ts     самописный WebGL: 5 типов кузова, металлик-шейдер, вращение
├── data/               presets (цвета/типы кузова, apiBodyToScene, hexToRgb), vin (мок-декодер)
├── ui/                 tokens.css (глобальные стили), status.ts (AlertStatus → тон + fallback)
├── components/         CarScene, GaragePanel, AddCarModal, RecommendationsView
└── App.vue             дек-слайдер: слайд «гараж» + слайд «регламент»
```

## Дев-инструменты

- Переключатель `mock_scenario` на слайде регламента (только в mock-режиме): `success | empty | error | slow`
  — прогоняет все состояния экрана.
- Демо-VIN для «Добавить машину»: `JTDBE32K700261000` (Toyota Camry), `WVWZZZ1KZAW000001` (VW Golf),
  `5UXWX7C5XBA000001` (BMW X5). Любой ввод ≥11 символов декодируется generic-декодером.

## Статус контракта

Слой данных переведён на контракт **§4.A** (`vehicles`/`alerts`, `vin/resolve`, `odometer`,
`service-events`, `X-Client-ID`), JSON — camelCase (ADR §5.2). Работает по моку (`mock/*.json`).
Точные формы бэка ещё не заморожены — при появлении API сверить казинг/поля с Dev 1 и выключить
`VITE_USE_MOCK`. Карта «модуль ↔ эндпоинт» — в спеке §5.

## Границы (см. CLAUDE.md)

Dev 2 владеет только `frontend/`. Контракты (§4) меняем по общему согласию.
