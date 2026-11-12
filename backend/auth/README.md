# Auth — единый вход (личный гараж)

**Один аккаунт — Telegram или MAX — JWT access/refresh. Сессия всегда b2c. Email и пароль сняты.**

## Модель

```
Account            один человек
  Identity[]       точки входа: telegram | max
  Membership[]     личный контекст b2c (есть всегда)
  RefreshToken[]   серверные refresh (хэш, ротация, отзыв)
```

Access-токен (JWT HS256, ~15 мин) несёт account id и контекст b2c.
**Refresh** (~30 дней) — непрозрачный, хранится хэшом, ротируется при обновлении.

## Эндпоинты

```
POST /api/v1/auth/telegram  {init_data}            # при TELEGRAM_BOT_TOKEN
POST /api/v1/auth/max       {init_data}            # при MAX_BOT_TOKEN
POST /api/v1/auth/refresh   {refresh_token}
POST /api/v1/auth/logout    {refresh_token}
GET  /api/v1/auth/me        (Bearer)                -> account_id, active_context, contexts[], is_admin
GET  /api/v1/admin/overview  (Bearer, role=admin)     # люди, машины, семь дней
GET  /api/v1/admin/users    (Bearer, role=admin)
POST /api/v1/admin/users    {telegram_id, name?}    # заранее заводит свой аккаунт
```

Клиент шлёт `Authorization: Bearer <access>`. Кабинет СТО и `POST /auth/switch` сняты.

## Включение (env)

- `JWT_SECRET` — включает auth (HS256).
- `TELEGRAM_BOT_TOKEN` — включает вход через Telegram (иначе `/auth/telegram` → 503).
- `TELEGRAM_ADMIN_ID` — числовой id. Этот человек при входе из Telegram получает роль `admin`. Вход из MAX роль не даёт.
- `MAX_BOT_TOKEN` — включает вход через MAX (иначе `/auth/max` → 503).

## Точки входа

- **Telegram и MAX** — initData мини-приложения (HMAC). Без токена своего бота вход выключен.
- Обычный браузер аккаунт не создаёт.
