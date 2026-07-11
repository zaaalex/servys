# ADR-002: Локальная LLM через Ollama с лимитом 2 ГБ RAM

- **Статус:** принято
- **Дата:** 2026-07-11
- **Area:** integration
- **Авторы:** команда Servys
- **Связь со спекой:** §2 и §4.B в `docs/superpowers/specs/2026-07-11-servys-mvp-design.md`
- **Заменяет:** выбор облачной LLM в ADR-001 в части LLM-провайдера и способа извлечения знаний; остальные решения ADR-001 сохраняются

## Контекст

Servys должен бесплатно собирать из открытых автомобильных источников структурированные сведения о регламентном обслуживании: компонент, операция, интервал по пробегу и времени, режим эксплуатации и подтверждающий фрагмент источника.

Ограничения решения:

- LLM должна запускаться локально, без тарификации за запросы;
- регистрация, API-ключ и облачный billing для рабочего пути не допускаются;
- весь процесс Ollama, включая дочерний model runner, должен потреблять не более **2 ГБ оперативной памяти**;
- размер модели на диске не является ограничением;
- GPU не обязателен, основной целевой режим — CPU inference;
- Go-backend, SQLite, SearXNG и другие процессы не входят в 2-гигабайтный лимит Ollama и ограничиваются отдельно;
- приложение не должно получать от модели свободный текст и использовать его как готовую рекомендацию;
- неточное или неподтвержденное значение безопаснее пропустить, чем угадать;
- модель не должна получать VIN, пользовательские идентификаторы, историю владельца, Bitrix24 webhook и другие секреты;
- напоминания `SOON`, `DUE` и `OVERDUE` должны по-прежнему рассчитываться детерминированным Go-кодом.

Маленькая локальная модель не обладает достаточной надежностью, чтобы одновременно выполнять веб-поиск, читать десятки больших документов, определять точную модификацию автомобиля, разрешать конфликты рынков и формировать окончательный регламент. Поэтому область ее ответственности должна быть узкой и проверяемой.

## Решение

### 1. Runtime и модель

Используем локальный **Ollama** и модель:

```text
qwen2.5:1.5b
```

Характеристики выбранной сборки Ollama на момент принятия ADR:

```text
архитектура:    Qwen2
параметры:      1.54B
квантизация:    Q4_K_M
размер файла:   около 986 МБ
лицензия:       Apache 2.0
языки:          русский, английский, китайский и другие
GPU:            не требуется
```

Причины выбора:

- модель заметно сильнее варианта `qwen2.5:0.5b`, но ее квантизированные веса оставляют запас внутри лимита 2 ГБ;
- Qwen2.5 ориентирована на instruction following, структурированные данные и JSON;
- поддерживаются языки, типичные для источников Servys: русский, английский и китайский;
- Apache 2.0 допускает использование модели в проекте;
- Ollama предоставляет локальный HTTP API и structured outputs по JSON Schema;
- локальный запуск не требует аккаунта и API-ключа.

Выбор модели не означает, что Servys доверяет ее встроенным знаниям об автомобилях. Факты принимаются только из текста источника, переданного в конкретном запросе.

### 2. Роль LLM

Ollama/Qwen используется только как **узкий extractor**:

```text
короткий фрагмент документа
        ↓
явно указанные в нем факты
        ↓
JSON по фиксированной схеме
```

Модель не выполняет:

- веб-поиск;
- загрузку URL;
- декодирование VIN;
- выбор доверенных доменов;
- определение юридической пригодности источника;
- окончательное разрешение конфликтов;
- расчет дат и статусов обслуживания;
- автоматическое назначение ремонта;
- генерацию обязательной замены на основании форумного сообщения.

Если в переданном фрагменте нет явно указанного факта, модель обязана вернуть пустой массив `facts`.

### 3. Конвейер данных

```mermaid
flowchart LR
    SIG[VehicleSignature без VIN]
    SEARCH[SearchProvider / SearXNG]
    FETCH[DocumentFetcher]
    NORMALIZE[HTML/PDF text normalization]
    SELECT[Candidate fragment selector]
    OLLAMA[Ollama qwen2.5:1.5b]
    VALIDATE[Go validator]
    PROFILE[(Knowledge profile / SQLite)]
    ENGINE[Deterministic maintenance engine]

    SIG --> SEARCH
    SEARCH --> FETCH
    FETCH --> NORMALIZE
    NORMALIZE --> SELECT
    SELECT -->|1 короткий chunk| OLLAMA
    OLLAMA -->|JSON Schema| VALIDATE
    VALIDATE -->|только принятые факты| PROFILE
    PROFILE --> ENGINE
```

Ответственность компонентов:

1. `SearchProvider` выполняет фиксированные поисковые запросы и возвращает URL.
2. `DocumentFetcher` безопасно загружает HTML или текстовый PDF.
3. Go-код очищает документ, классифицирует источник и находит кандидатные фрагменты по словарям компонентов, операциям, числам и единицам.
4. Один короткий фрагмент передается в Qwen.
5. Qwen возвращает только JSON по схеме.
6. Go-валидатор проверяет JSON, доказательство и допустимость значения.
7. Принятые факты сохраняются в общий knowledge profile по `variant_key`.
8. Движок обслуживания рассчитывает пользовательские сроки из утвержденных правил и истории автомобиля.

Поисковые запросы в P0 задаются шаблонами. LLM-generated search queries не используются, чтобы не расходовать контекст и не поручать маленькой модели планирование исследования.

### 4. Контракты

Публичный порт `Recommender` сохраняется:

```go
type Recommender interface {
    Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}
```

Внутри knowledge pipeline добавляется более узкий порт:

```go
type DocumentChunk struct {
    SourceID string
    URL      string
    Title    string
    Text     string
}

type ExtractedFact struct {
    ComponentCode  string  `json:"componentCode"`
    Operation      string  `json:"operation"`
    IntervalKM     *int    `json:"intervalKm"`
    IntervalMonths *int    `json:"intervalMonths"`
    ScheduleMode   string  `json:"scheduleMode"`
    UsageMode      string  `json:"usageMode"`
    Evidence       string  `json:"evidence"`
}

type Extraction struct {
    Facts []ExtractedFact `json:"facts"`
}

type KnowledgeExtractor interface {
    Extract(
        ctx context.Context,
        signature VehicleSignature,
        chunk DocumentChunk,
    ) (Extraction, error)
}
```

`SourceID`, URL, класс источника и применимость автомобиля задаются Go-кодом. Модель не генерирует URL и не может добавить источник, которого не было во входе.

### 5. Вход модели

В модель передаются только:

- обезличенная `VehicleSignature` без VIN;
- один нормализованный фрагмент источника;
- закрытый список разрешенных `componentCode`;
- закрытый список операций;
- JSON Schema;
- инструкция извлекать только явно написанные факты.

Пример:

```text
VEHICLE:
KIA K3, 2020, gasoline, market hint CN

SOURCE:
Replace the engine oil and oil filter every 10,000 km or 12 months,
whichever comes first.
```

Запрещено передавать:

- VIN;
- `X-Client-ID`, `user_id`, email, телефон и имя владельца;
- Bitrix24 portal URL и webhook;
- пользовательскую историю обслуживания;
- ключи и токены;
- полный дамп базы данных;
- непроверенные инструкции, найденные внутри HTML.

Текст документа трактуется как недоверенные данные, а не как инструкции модели.

### 6. Structured output

Запрос выполняется через локальный endpoint:

```text
POST http://127.0.0.1:11434/api/chat
```

Обязательные параметры:

```json
{
  "model": "qwen2.5:1.5b",
  "stream": false,
  "keep_alive": "30s",
  "options": {
    "num_ctx": 2048,
    "num_predict": 320,
    "temperature": 0
  },
  "format": {
    "type": "object",
    "properties": {
      "facts": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "componentCode": {
              "type": "string",
              "enum": [
                "engine_oil",
                "engine_oil_filter",
                "engine_air_filter",
                "cabin_filter",
                "spark_plugs",
                "brake_fluid",
                "engine_coolant",
                "transmission_fluid",
                "timing_drive",
                "fuel_filter"
              ]
            },
            "operation": {
              "type": "string",
              "enum": [
                "replace",
                "inspect",
                "measure",
                "adjust",
                "diagnose"
              ]
            },
            "intervalKm": {
              "type": ["integer", "null"]
            },
            "intervalMonths": {
              "type": ["integer", "null"]
            },
            "scheduleMode": {
              "type": "string",
              "enum": [
                "mileage",
                "time",
                "whichever_first",
                "unspecified"
              ]
            },
            "usageMode": {
              "type": "string",
              "enum": ["normal", "severe", "unknown"]
            },
            "evidence": {
              "type": "string"
            }
          },
          "required": [
            "componentCode",
            "operation",
            "intervalKm",
            "intervalMonths",
            "scheduleMode",
            "usageMode",
            "evidence"
          ]
        }
      }
    },
    "required": ["facts"]
  }
}
```

JSON Schema также включается текстом в prompt, поскольку это повышает устойчивость structured output на маленькой модели.

Пример допустимого ответа:

```json
{
  "facts": [
    {
      "componentCode": "engine_oil",
      "operation": "replace",
      "intervalKm": 10000,
      "intervalMonths": 12,
      "scheduleMode": "whichever_first",
      "usageMode": "normal",
      "evidence": "Replace the engine oil and oil filter every 10,000 km or 12 months, whichever comes first."
    },
    {
      "componentCode": "engine_oil_filter",
      "operation": "replace",
      "intervalKm": 10000,
      "intervalMonths": 12,
      "scheduleMode": "whichever_first",
      "usageMode": "normal",
      "evidence": "Replace the engine oil and oil filter every 10,000 km or 12 months, whichever comes first."
    }
  ]
}
```

Приложение не использует текст вне `message.content` и не сохраняет reasoning или произвольные пояснения модели.

### 7. Ограничение контекста и chunking

Конфигурация по умолчанию:

```text
контекст модели:             2048 токенов
максимальный output:         320 токенов
полезный фрагмент документа: 800–1000 токенов
перекрытие chunk-ов:         80–100 токенов
источников за проход:        до 5
одновременных LLM-запросов:  1
```

В лимит 2048 входят system prompt, схема, подпись автомобиля, текст источника и ответ. Полные руководства и длинные PDF в модель не передаются.

Go-код предварительно выбирает фрагменты, содержащие сочетание:

- синонима известного компонента;
- операции или формулировки периодичности;
- числа;
- единицы `km`, `mi`, `months`, `years` либо их локализованный эквивалент.

Для PDF без текстового слоя OCR не выполняется в P0. Такой источник сохраняется как непрочитанный либо обрабатывается отдельным будущим адаптером.

### 8. Программная валидация

Structured output гарантирует форму ответа, но не истинность значений. Ни один факт не публикуется напрямую.

Go-валидатор обязан проверить:

1. ответ является валидным JSON и соответствует схеме;
2. `componentCode` входит в закрытый каталог Servys;
3. `operation` входит в разрешенный enum;
4. интервалы положительны и входят в допустимый диапазон конкретного компонента;
5. `scheduleMode` согласован с заполненными полями;
6. `evidence` после детерминированной нормализации пробелов является точной подстрокой исходного chunk-а;
7. операция подтверждается evidence: `inspect` не может быть преобразован в `replace`;
8. источник и его URL существуют в текущем knowledge job;
9. источник применим к нужной марке, модели, году, рынку и силовой установке с известным уровнем точности;
10. forum/owner-report не создает обязательное replacement rule;
11. конфликтующие значения сохраняются раздельно и не усредняются;
12. модель не создает новый URL, новый компонент или новый тип операции.

Поле `confidence`, сгенерированное моделью, не используется как основание публикации и поэтому отсутствует в контракте extractor-а. Уровень доказательности рассчитывается Go-кодом из класса источника, применимости и количества независимых подтверждений.

При ошибке отдельный факт отклоняется. Ошибка одного факта не делает валидными остальные и не должна приводить к частично неконсистентной записи: профиль сохраняется транзакционно после завершения валидации прохода.

### 9. Ограничение памяти Ollama

Лимит 2 ГБ задается не оценкой, а cgroup systemd для всего сервиса Ollama, включая дочерние runner-процессы.

Установка модели:

```bash
curl -fsSL https://ollama.com/install.sh | sh
ollama pull qwen2.5:1.5b
```

Override сервиса:

```bash
sudo systemctl edit ollama.service
```

```ini
[Service]
Environment="OLLAMA_HOST=127.0.0.1:11434"
Environment="OLLAMA_CONTEXT_LENGTH=2048"
Environment="OLLAMA_NUM_PARALLEL=1"
Environment="OLLAMA_MAX_LOADED_MODELS=1"
Environment="OLLAMA_MAX_QUEUE=8"
Environment="OLLAMA_FLASH_ATTENTION=1"
Environment="OLLAMA_KV_CACHE_TYPE=q8_0"
Environment="OLLAMA_KEEP_ALIVE=30s"
Environment="OLLAMA_NO_CLOUD=1"

MemoryAccounting=yes
MemoryHigh=1800M
MemoryMax=2G
```

Применение:

```bash
sudo systemctl daemon-reload
sudo systemctl restart ollama
```

Смысл параметров:

- `OLLAMA_NUM_PARALLEL=1` не умножает память контекста параллельными запросами;
- `OLLAMA_MAX_LOADED_MODELS=1` запрещает одновременно держать несколько моделей;
- `OLLAMA_CONTEXT_LENGTH=2048` ограничивает KV cache;
- `OLLAMA_KV_CACHE_TYPE=q8_0` примерно вдвое уменьшает память KV cache относительно `f16` с небольшой потерей точности;
- `OLLAMA_FLASH_ATTENTION=1` используется, когда выбранный backend его поддерживает;
- `OLLAMA_KEEP_ALIVE=30s` освобождает память вскоре после серии запросов;
- `OLLAMA_NO_CLOUD=1` отключает облачные модели и web search Ollama;
- `MemoryHigh=1800M` создает раннее давление на память;
- `MemoryMax=2G` является жестким пределом RAM cgroup.

Swap отдельно не запрещается. Он может использоваться ОС как страховка, но не отменяет требование `MemoryMax=2G` для резидентной памяти cgroup.

Точное потребление зависит от версии Ollama, CPU backend и входного prompt. Поэтому заявляется не фиксированное значение RSS, а проверяемое ограничение: запрос либо завершается внутри 2 ГБ, либо контролируемо падает и переводит job в retry/fallback.

### 10. Конфигурация Servys

Рекомендуемые переменные backend:

```env
LLM_MODE=live
LLM_PROVIDER=ollama
LLM_FALLBACK_PROVIDER=none

OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_MODEL=qwen2.5:1.5b
OLLAMA_TIMEOUT=120s
OLLAMA_CONTEXT_LENGTH=2048
OLLAMA_MAX_OUTPUT_TOKENS=320
OLLAMA_KEEP_ALIVE=30s

KNOWLEDGE_WORKER_CONCURRENCY=1
KNOWLEDGE_CHUNK_TOKENS=900
KNOWLEDGE_CHUNK_OVERLAP=90
KNOWLEDGE_MAX_SOURCES=5
KNOWLEDGE_REQUIRE_EXACT_EVIDENCE=true
KNOWLEDGE_PROMPT_VERSION=ollama-qwen25-v1
KNOWLEDGE_SCHEMA_VERSION=v1
```

Если backend работает в Docker, а Ollama установлен на хосте:

```env
OLLAMA_BASE_URL=http://host.docker.internal:11434
```

Для Linux-контейнера:

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
```

Ollama не публикуется в интернет и не вызывается непосредственно frontend-ом. Доступ к ней имеет только backend.

### 11. Очередь, cache и отказоустойчивость

Knowledge build выполняется фоновым worker-ом и не блокирует добавление автомобиля.

Правила:

- concurrency worker-а — `1`;
- один chunk — один LLM-запрос;
- при cache hit по `variant_key + prompt_version + schema_version` повторный сбор не запускается;
- модель не вызывается при каждом открытии карточки или обновлении пробега;
- timeout одного запроса — 120 секунд;
- при невалидном JSON допускается один повтор с укороченным chunk-ом;
- при timeout, HTTP 503, недоступности Ollama или cgroup OOM job переводится в retry с backoff;
- после исчерпания попыток профиль получает состояние `failed`, но автомобиль и базовые карточки остаются доступны;
- до появления подтвержденного профиля применяются только существующие core/YAML rules и явно обозначенные fallback-правила;
- автоматический переход на облачную модель запрещен, чтобы рабочий путь оставался бесплатным и локальным.

Рекомендуемый backoff knowledge job:

```text
1 минута → 5 минут → 30 минут
```

### 12. Безопасность

- Ollama слушает только `127.0.0.1`.
- Cloud-функции отключены через `OLLAMA_NO_CLOUD=1`.
- В prompt не передаются VIN, пользовательские данные и секреты.
- HTML/PDF считается недоверенным вводом и отделяется от system instructions.
- `DocumentFetcher` применяет HTTPS/SSRF-проверки, лимит body, MIME allowlist и безопасную политику redirect.
- URL и `source_id` присваиваются backend-ом, а не моделью.
- В логах не сохраняется полный документ или полный prompt; допустимы hash источника, model, latency, prompt/schema version и код ошибки.
- Frontend не имеет доступа к Ollama endpoint.

### 13. Наблюдаемость

Проверка загруженной модели:

```bash
ollama ps
```

Проверка cgroup:

```bash
systemctl show ollama.service \
  -p MemoryCurrent \
  -p MemoryPeak \
  -p MemoryHigh \
  -p MemoryMax
```

Backend публикует технические метрики без содержимого prompt-а:

```text
knowledge_jobs_total{status}
ollama_requests_total{result}
ollama_request_duration_seconds
ollama_invalid_json_total
ollama_rejected_facts_total{reason}
knowledge_cache_hits_total
```

`MemoryPeak` проверяется на worst-case fixture после каждого обновления Ollama или модели.

### 14. Scope P0

В первом релизе локальная модель извлекает только регламентные операции из официальных и технических источников:

- моторное масло и фильтр;
- воздушный и салонный фильтры;
- свечи;
- тормозную жидкость;
- охлаждающую жидкость;
- трансмиссионную жидкость;
- ремень/цепь ГРМ;
- топливный фильтр;
- явно сформулированные inspections.

`Risk intelligence`, форумная аналитика, типовые поломки и выводы из пользовательских отзывов не входят в P0 локальной модели. Их добавление требует отдельного benchmark и отдельного решения, поскольку Qwen2.5 1.5B недостаточно надежна для сложного синтеза неоднородных свидетельств.

## Рассмотренные альтернативы

### Gemini Flash-Lite

Плюсы:

- высокое качество извлечения;
- URL Context и чтение PDF/HTML на стороне провайдера;
- большой контекст;
- меньше собственного кода для document ingestion.

Почему не выбрано:

- бесплатный tier не является гарантированным SLA;
- для публичного использования могут потребоваться billing и соблюдение облачных условий;
- данные передаются внешнему провайдеру;
- решение не удовлетворяет требованию полностью локального бесплатного рабочего пути.

### `qwen2.5:0.5b`

Плюсы:

- существенно меньший расход RAM;
- более быстрый CPU inference.

Почему не выбрано:

- заметно ниже качество сопоставления терминов и следования сложной JSON Schema;
- выше риск пропуска фактов и смешения `inspect`/`replace`;
- при лимите 2 ГБ есть возможность использовать более сильную 1.5B-модель.

`qwen2.5:0.5b` остается ручным rollback-вариантом, если конкретный target host не проходит memory acceptance test. Автоматически переключаться между моделями backend не должен.

### `qwen3:1.7b` и более крупные модели

Плюсы:

- потенциально лучшее рассуждение и instruction following.

Почему не выбрано:

- меньший запас внутри жесткого лимита 2 ГБ;
- выше вероятность cgroup OOM на максимальном настроенном контексте;
- преимуществ для узкого extraction-сценария недостаточно, чтобы принять операционный риск.

### `qwen3.5:0.8b`

Плюсы:

- компактная современная модель;
- помещается в лимит.

Почему не выбрано:

- меньший размер модели по сравнению с Qwen2.5 1.5B;
- перед заменой требуется отдельный benchmark на автомобильных fixtures;
- текущая задача преимущественно текстовая, а Qwen2.5 1.5B имеет подходящий профиль JSON extraction.

### Только regex/парсеры без LLM

Плюсы:

- минимальная память;
- полная детерминированность;
- высокая скорость.

Почему не выбрано как единственный путь:

- источники используют разные языки, таблицы, формулировки и единицы;
- количество ручных шаблонов быстро растет;
- regex остается обязательным prefilter/validator, но не единственным extractor-ом.

### LLM как автономный search/research agent

Плюсы:

- меньше явной оркестрации в Go.

Почему отклонено:

- модель 1.5B недостаточно надежна для самостоятельного исследования;
- невозможно гарантировать происхождение и применимость фактов;
- возрастает prompt injection surface;
- увеличиваются контекст и потребление RAM;
- результат сложнее проверить программно.

## Последствия

### Положительные

- ✅ Нет оплаты за токены и запросы.
- ✅ Не нужны аккаунт, API-ключ или billing LLM-провайдера.
- ✅ Технические документы и prompt-ы не покидают сервер.
- ✅ Потребление Ollama имеет жесткий проверяемый предел 2 ГБ RAM.
- ✅ Приложение получает типизированный JSON, а не свободный текст.
- ✅ Источник и evidence сохраняются для каждого принятого правила.
- ✅ Доменный `Recommender` и движок напоминаний не зависят от конкретной модели.
- ✅ Модель можно заменить без изменения frontend API и расчета сроков.
- ✅ Cache по модификации исключает повторные вычисления для одинаковых автомобилей.

### Отрицательные и техдолг

- ⚠️ CPU inference может занимать десятки секунд на chunk в зависимости от сервера.
- ⚠️ Маленькая модель будет пропускать часть сложных таблиц и неявных формулировок.
- ⚠️ Backend должен самостоятельно загружать, очищать и chunk-ировать HTML/PDF.
- ⚠️ Требуется словарь автомобильных терминов минимум для русского, английского и китайского.
- ⚠️ OCR сканированных документов остается нерешенным.
- ⚠️ Обновление Ollama или модели может изменить peak memory и качество, поэтому нужен regression benchmark.
- ⚠️ Жесткий `MemoryMax` может завершить runner; worker обязан корректно переживать такую ошибку.
- ⚠️ Structured JSON не устраняет семантические ошибки, поэтому validator остается обязательным.

### Влияние на контракты

- `Recommender` не меняется.
- Добавляется внутренний `KnowledgeExtractor`.
- Frontend API не меняется.
- Формат сохраненного knowledge profile остается provider-agnostic.
- Источник правила должен содержать `model`, `prompt_version`, `schema_version`, `source_id` и дату формирования.
- Облачный fallback отсутствует; его добавление потребует нового ADR.

## Критерии приемки

Решение считается реализованным, когда выполнены все условия:

1. `ollama pull qwen2.5:1.5b` и локальный запрос работают без регистрации и API-ключа.
2. Ollama слушает только `127.0.0.1:11434`.
3. `OLLAMA_NO_CLOUD=1` включен.
4. `MemoryMax=2G` виден через `systemctl show`.
5. Worst-case запрос с `num_ctx=2048`, `num_predict=320` и concurrency 1 не превышает 2 ГБ RAM; значение фиксируется через `MemoryPeak`.
6. При превышении лимита job завершается контролируемой ошибкой, а основной API Servys продолжает работать.
7. Ответ модели всегда проходит JSON parse и schema validation до использования.
8. Каждый сохраненный факт имеет evidence, найденный в исходном chunk-е после нормализации пробелов.
9. Модель не может сохранить URL, отсутствующий во входном knowledge job.
10. `inspect` не публикуется как `replace`.
11. Forum-only источник не создает replacement rule.
12. На fixture-корпусе из русских, английских и китайских фрагментов достигаются:
    - precision принятых фактов не ниже 95%;
    - recall явно сформулированных интервалов не ниже 80%;
    - 100% отклонение фактов с отсутствующим evidence.
13. Cache hit для существующего `variant_key` не вызывает Ollama.
14. При `LLM_MODE=fixture` и `LLM_MODE=disabled` приложение сохраняет работоспособность.

## План реализации

1. Добавить `KnowledgeExtractor` и DTO extraction-а.
2. Реализовать `integrations/ollama` через `net/http` и `/api/chat`.
3. Добавить JSON Schema и prompt version `ollama-qwen25-v1`.
4. Реализовать загрузку/нормализацию документов и deterministic chunk selector.
5. Реализовать validator evidence, enum, ranges, source applicability и conflict handling.
6. Подключить extractor к существующему `Recommender`/knowledge cache.
7. Добавить systemd override и пример `.env` в deployment documentation.
8. Собрать fixture corpus RU/EN/ZH и memory benchmark.
9. Проверить fallback `fixture/disabled` и ошибки timeout/503/OOM.
10. После прохождения критериев включить `LLM_PROVIDER=ollama` по умолчанию.

## Ссылки

- Ollama model card `qwen2.5:1.5b`: https://ollama.com/library/qwen2.5:1.5b
- Ollama Structured Outputs: https://docs.ollama.com/capabilities/structured-outputs
- Ollama FAQ: context, concurrency, KV cache, keep-alive и local-only mode: https://docs.ollama.com/faq
- Ollama Linux installation: https://docs.ollama.com/linux
- systemd resource control (`MemoryHigh`, `MemoryMax`): https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html
