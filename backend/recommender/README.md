# Рекомендательный слой — kickoff для Dev 3

Ты (Dev 3) владеешь **рекомендательным слоем**: `backend/{vin, recommender, engine, data}`.
Это всё, что превращает авто в план ТО: **парсинг VIN → знание правил → выявление «что и когда обслужить»**.

> Старт для агента: _«Я Dev 3 (рекомендательный слой). Реализуй по `backend/recommender/README.md`»_.
> Контекст: `PROGRESS.md`, спека §4 (контракты), ADR-001 (детали данных — «как»).

## Что реализовать (заменить стабы)

Платформа (Dev 1) зовёт тебя через **один порт** — реализуй его:

```go
// backend/recommender/recommender.go
type Advisor interface {
    Alerts(ctx context.Context, v domain.Vehicle) ([]domain.Alert, error)
}
```

Три файла со стабами → твоя боевая реализация:

| Стаб сейчас | Что сделать |
|-------------|-------------|
| `recommender.StubAdvisor` (`NewStubAdvisor`) | боевой `Advisor`: правила (YAML/LLM) → `engine` → `[]Alert` |
| `recommender.Stub` (`Recommender.Rules`) | правила регламента из `data/maintenance_rules.yaml` + LLM-догенерация для моделей вне YAML |
| `vin.Stub` (`VINProvider.Resolve`) | адаптер Drom (best-effort, ADR §5); при любой ошибке — типизированная ошибка, платформа отдаст ручную форму |
| `engine.BuildAlerts` | развить логику: baseline из истории ТО, `MAINTENANCE_HISTORY_REQUIRED`, статусы (ADR §8). Сейчас упрощение baseline=0 |

**Отдаёшь конструктор** `recommender.NewAdvisor(...)` — Dev 1 подключит его в `main.go` вместо `NewStubAdvisor`.

## Правила

- **НЕ трогай** `api/`, `store/`, `main.go`, `sink/`, `frontend/`.
- **`domain/` заморожен** (типы `Vehicle`, `Rule`, `Alert`, ...). Нужно новое поле — согласуй с Dev 1.
- Нет правила для модели → **не выдумывай интервал**: отдай alert типа `REGULATION_NOT_FOUND` (ADR §5.5).
- Готово, когда: `go test ./...` зелёный, `Advisor.Alerts` возвращает реальные alerts, `vin.Resolve` парсит Drom.

## «Как» — из ADR-001

- **§5** — VIN/Drom: `variant_key`, уровни совпадения, типизированные ошибки.
- **§6** — LLM-конвейер базы знаний (источники, схема, валидация, кэш).
- **§8** — расчёт сроков, история обслуживания, статусы.

## ⚠️ Открытый вопрос — LLM

Спека/решение команды = **Claude API**; ADR-001 (rev2) = **Gemini 3.1 Flash-Lite + SearXNG**.
**Согласуй одно значение** до реализации LLM-части и зафиксируй в спеке. Ключ — через env
(`ANTHROPIC_API_KEY` / соответствующий), никогда в коде. **Начать можно без LLM** — на YAML-правилах
и `engine`, LLM подключить вторым шагом.
