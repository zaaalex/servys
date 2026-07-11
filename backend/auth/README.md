# Auth — единый вход и авторизация (Dev 1)

**Один аккаунт — много точек входа — JWT access/refresh — переключение b2c|b2b.**
Точка входа ≠ аккаунт: веб/Telegram/мобилка/… это разные `Identity`, но все резолвятся в один `Account`.

## Модель

```
Account            один человек
  Identity[]       точки входа: password(email) | telegram | bitrix(слот)
  Membership[]     контексты: {b2c} (есть всегда) + {b2b, tenant=СТО, role}
  RefreshToken[]   серверные refresh (хэш, ротация, отзыв)
```

Активный контекст (b2c/b2b + тенант + роль) зашит в **access-токен** (JWT HS256, ~15 мин).
**Refresh** (~30 дней) — непрозрачный, хранится хэшом, ротируется при обновлении.

## Эндпоинты

```
POST /api/v1/auth/register  {email, password}      -> {access_token, refresh_token, expires_in}
POST /api/v1/auth/login     {email, password}
POST /api/v1/auth/telegram  {init_data}            # Telegram Mini App (при TELEGRAM_BOT_TOKEN)
POST /api/v1/auth/refresh   {refresh_token}         # ротация
POST /api/v1/auth/logout    {refresh_token}
GET  /api/v1/auth/me        (Bearer)                -> account_id, active_context, contexts[]
POST /api/v1/auth/switch    (Bearer) {ctx_type, tenant_id} -> {access_token}  # сменить контекст
```

Клиент шлёт `Authorization: Bearer <access>`. Переключение b2c↔b2b — `/switch` выдаёт новый access
с другим контекстом (валидируется по memberships), как переключатель воркспейсов.

## Гейт b2b (per-СТО, вариант 2)

- `/api/v1/b2b/service-centers*` — **Bearer обязателен**. Подключивший СТО становится владельцем;
  скан/список — только для своих СТО (проверка membership). Чужой СТО → `403`.
- `POST /api/v1/b2b/scan-all` — **операторское**: заголовок `X-Admin-Token` (env `ADMIN_TOKEN`).

## Включение (env)

- `JWT_SECRET` — включает auth (HS256).
- `TELEGRAM_BOT_TOKEN` — включает вход через Telegram (иначе `/auth/telegram` → 503).
- `ADMIN_TOKEN` — включает `scan-all`.
- `APP_SECRET_KEY` — шифрование вебхуков СТО (b2b).

## Точки входа

- **Веб/мобилка** — email+пароль (bcrypt). ✅
- **Telegram** — initData Mini App (HMAC-валидация). ✅ (при токене бота)
- **Bitrix** — **интеграция, не вход** в MVP; провайдер `bitrix` зарезервирован на будущее.

## Осознанно отложено

OAuth/Marketplace и вход через Bitrix, привязка нескольких точек входа к аккаунту (linking UI),
RS256/JWKS (для отдельного auth-сервиса), перевод b2c-эндпоинтов с гостевого `X-Client-ID` на аккаунт.
