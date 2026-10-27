# messenger

Тонкий слой бота servys для Telegram и MAX. Пакет проверяет секрет вебхука,
отличает `/start` от остальных апдейтов и проверяет initData мини-приложения MAX.
Очередей, sweeper, рейт-лимита и журнала аудита нет. Токен, секрет и сырое тело
в логи не пишутся.

`HandleTelegram` и `HandleMax` ничего не отправляют: они возвращают статус и,
если нужно ответить, `Outcome.Send`. HTTP-вызов делает `Sender` из `NewHTTPSender`.

## Маршруты

Пакет их не регистрирует. Интегратор монтирует:

- `POST /api/v1/telegram/webhook` — заголовок `X-Telegram-Bot-Api-Secret-Token`, вызов `HandleTelegram`.
- `POST /api/v1/max/webhook` — заголовок `X-Max-Bot-Api-Secret`, вызов `HandleMax`.

Ответ маршрута — `Outcome.Status`. Если `Send` не nil, его забирает `Sender.Send`.

Пустой токен провайдера — 404. Пустой или чужой секрет — 403. Тело не JSON — 400.
Узнанный, но не стартовый апдейт — 200 без отправки. Старт — 200, текст и кнопка
«Открыть гараж» на `MINI_APP_URL` (без URL кнопка не ставится).

## Окружение

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_WEBHOOK_SECRET`
- `MAX_BOT_TOKEN`
- `MAX_WEBHOOK_SECRET`
- `MINI_APP_URL`
- `MAX_API_BASE` — необязательно; пустое значение значит `https://platform-api2.max.ru`

## Вход MAX

`ValidateMaxInitData` проверяет подпись initData (HMAC-SHA256, ключ `WebAppData`),
срок `auth_date` (сутки назад и не больше пяти минут вперёд) и числовой `user.id`.
Дальше — существующая identity аккаунта: провайдер `"max"`, внешний id — строка,
которую вернула функция. Так же устроен вход Telegram, только провайдер `"telegram"`.
