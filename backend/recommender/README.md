# Рекомендательный слой

Dev 3 владеет `backend/{vin, recommender, engine, data}`.

`main` собирает рантайм через `recommender.NewFromEnv` и `vin.NewFromEnv`.

## Порты

```go
type Recommender interface {
    Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}

type Advisor interface {
    Alerts(ctx context.Context, v domain.Vehicle, history []domain.ServiceEvent) ([]domain.Alert, error)
}
```

`NewAdvisor` берёт правила и считает сроки в `engine.BuildAlerts`.

При `LLM_MODE=live` интервалы берёт поиск страниц и Ollama: в карточку попадает факт с цитатой из текста страницы. Готового профиля и типового списка в выдаче нет.

VIN: Drom, затем NHTSA. `vin.Stub` и `recommender.NewStubAdvisor` остаются для тестов.

Файлы: `advisor.go` (порты), `chain.go`, `pipeline.go`, `search.go`, `stub.go`.

## Правила

- Не трогать `api/`, `store/`, `main.go`, `frontend/`.
- `domain` менять только вместе с Dev 1.
- Нет правила и нет интервала — alert `REGULATION_NOT_FOUND`.
- LLM — локальная Ollama. На немецком VPS модель `qwen2.5:0.5b` (ADR-003). Ключ облачной модели в код не класть.

Детали данных — ADR-001 (§5 VIN, §6 конвейер знаний, §8 сроки). При расхождении по HTTP и портам права спека §4.
