package messenger

import "testing"

const (
	wantStartText  = "Это servys. Здесь ваша машина, история обслуживания и что пора сделать."
	wantButtonText = "Открыть гараж"
	wantMiniAppURL = "https://garage.example/app"
)

func telegramCfg() Config {
	return Config{
		TelegramBotToken:      "tg-token",
		TelegramWebhookSecret: "tg-secret",
		MiniAppURL:            wantMiniAppURL,
	}
}

func maxCfg() Config {
	return Config{
		MaxBotToken:      "max-token",
		MaxWebhookSecret: "max-secret",
		MiniAppURL:       wantMiniAppURL,
	}
}

func TestHandleTelegram(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		secret string
		body   string
		status int
		chatID string
		button bool
	}{
		{name: "token off", cfg: Config{TelegramWebhookSecret: "tg-secret"}, secret: "tg-secret", body: `{"message":{"text":"/start","chat":{"id":1}}}`, status: 404},
		{name: "empty secret", cfg: Config{TelegramBotToken: "tg-token"}, secret: "", body: `{"message":{"text":"/start","chat":{"id":1}}}`, status: 403},
		{name: "empty header", secret: "", body: `{"message":{"text":"/start","chat":{"id":1}}}`, status: 403},
		{name: "wrong secret", secret: "other", body: `{"message":{"text":"/start","chat":{"id":1}}}`, status: 403},
		{name: "not json", body: `{`, status: 400},
		{name: "array", body: `[1]`, status: 400},
		{name: "null", body: `null`, status: 400},
		{name: "two objects", body: `{}{}`, status: 400},
		{name: "empty object", body: `{}`, status: 200},
		{name: "edited message", body: `{"edited_message":{"text":"/start","chat":{"id":1}}}`, status: 200},
		{name: "callback", body: `{"callback_query":{"data":"/start","message":{"chat":{"id":1}}}}`, status: 200},
		{name: "other text", body: `{"message":{"text":"привет","chat":{"id":1}}}`, status: 200},
		{name: "start prefix glued", body: `{"message":{"text":"/startfoo","chat":{"id":1}}}`, status: 200},
		{name: "start", body: `{"message":{"text":"/start","chat":{"id":42}}}`, status: 200, chatID: "42", button: true},
		{name: "start trimmed", body: `{"message":{"text":"  /start  ","chat":{"id":7}}}`, status: 200, chatID: "7", button: true},
		{name: "start with payload", body: `{"message":{"text":"/start garage","chat":{"id":7}}}`, status: 200, chatID: "7", button: true},
		{name: "start mention", body: `{"message":{"text":"/start@servys_bot hi","chat":{"id":-100}}}`, status: 200, chatID: "-100", button: true},
		{name: "wide chat id", body: `{"message":{"text":"/start","chat":{"id":9007199254740993}}}`, status: 200, chatID: "9007199254740993", button: true},
		{name: "no chat id", body: `{"message":{"text":"/start"}}`, status: 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			if cfg.TelegramBotToken == "" && cfg.TelegramWebhookSecret == "" && tt.name != "token off" && tt.name != "empty secret" {
				cfg = telegramCfg()
			}
			if tt.name == "empty header" {
				cfg = telegramCfg()
			}
			secret := tt.secret
			if secret == "" && tt.name != "empty secret" && tt.name != "empty header" && tt.name != "token off" {
				secret = cfg.TelegramWebhookSecret
			}
			assertOutcome(t, HandleTelegram(cfg, secret, []byte(tt.body)), tt.status, "telegram", tt.chatID, tt.button, cfg.MiniAppURL)
		})
	}
}

func TestHandleTelegramEmptyMiniAppOmitsButton(t *testing.T) {
	cfg := telegramCfg()
	cfg.MiniAppURL = ""
	got := HandleTelegram(cfg, cfg.TelegramWebhookSecret, []byte(`{"message":{"text":"/start","chat":{"id":3}}}`))
	assertOutcome(t, got, 200, "telegram", "3", false, "")
	if got.Send == nil || got.Send.Text != wantStartText {
		t.Fatalf("text: %#v", got.Send)
	}
}

func TestHandleMax(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		secret string
		body   string
		status int
		chatID string
		button bool
	}{
		{name: "token off", cfg: Config{MaxWebhookSecret: "max-secret"}, secret: "max-secret", body: `{"update_type":"bot_started","user":{"user_id":1}}`, status: 404},
		{name: "empty secret", cfg: Config{MaxBotToken: "max-token"}, secret: "", body: `{"update_type":"bot_started","user":{"user_id":1}}`, status: 403},
		{name: "empty header", secret: "", body: `{"update_type":"bot_started","user":{"user_id":1}}`, status: 403},
		{name: "wrong secret", secret: "nope", body: `{"update_type":"bot_started","user":{"user_id":1}}`, status: 403},
		{name: "not json", body: `nope`, status: 400},
		{name: "null", body: `null`, status: 400},
		{name: "other update", body: `{"update_type":"message_callback","callback":{"payload":"/start"}}`, status: 200},
		{name: "bot stopped", body: `{"update_type":"bot_stopped","user":{"user_id":1}}`, status: 200},
		{name: "bot started", body: `{"update_type":"bot_started","payload":"hello","user":{"user_id":55}}`, status: 200, chatID: "55", button: true},
		{name: "bot started wide id", body: `{"update_type":"bot_started","user":{"user_id":9007199254740993}}`, status: 200, chatID: "9007199254740993", button: true},
		{name: "bot started without user", body: `{"update_type":"bot_started"}`, status: 200},
		{name: "message start", body: `{"update_type":"message_created","message":{"sender":{"user_id":77},"body":{"text":" /start extra "}}}`, status: 200, chatID: "77", button: true},
		{name: "message not start", body: `{"update_type":"message_created","message":{"sender":{"user_id":77},"body":{"text":"привет"}}}`, status: 200},
		{name: "mention is not max start", body: `{"update_type":"message_created","message":{"sender":{"user_id":77},"body":{"text":"/start@bot"}}}`, status: 200},
		{name: "body text wins", body: `{"update_type":"message_created","message":{"sender":{"user_id":8},"text":"/start","body":{"text":"no"}}}`, status: 200},
		{name: "flat text", body: `{"update_type":"message_created","message":{"sender":{"user_id":9},"text":"/start"}}`, status: 200, chatID: "9", button: true},
		{name: "start without sender", body: `{"update_type":"message_created","message":{"body":{"text":"/start"}}}`, status: 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			if cfg.MaxBotToken == "" && cfg.MaxWebhookSecret == "" && tt.name != "token off" && tt.name != "empty secret" {
				cfg = maxCfg()
			}
			if tt.name == "empty header" {
				cfg = maxCfg()
			}
			secret := tt.secret
			if secret == "" && tt.name != "empty secret" && tt.name != "empty header" && tt.name != "token off" {
				secret = cfg.MaxWebhookSecret
			}
			assertOutcome(t, HandleMax(cfg, secret, []byte(tt.body)), tt.status, "max", tt.chatID, tt.button, cfg.MiniAppURL)
		})
	}
}

func TestHandleMaxEmptyMiniAppOmitsButton(t *testing.T) {
	cfg := maxCfg()
	cfg.MiniAppURL = ""
	got := HandleMax(cfg, cfg.MaxWebhookSecret, []byte(`{"update_type":"bot_started","user":{"user_id":4}}`))
	assertOutcome(t, got, 200, "max", "4", false, "")
}

func TestHandlersDoNotEchoSecrets(t *testing.T) {
	body := []byte(`{"message":{"text":"/start","chat":{"id":1}},"secret":"tg-secret","token":"tg-token"}`)
	got := HandleTelegram(telegramCfg(), "nope", body)
	if got.Status != 403 || got.Send != nil {
		t.Fatalf("mismatch must not send: %#v", got)
	}
	ok := HandleTelegram(telegramCfg(), "tg-secret", body)
	if ok.Send == nil || ok.Send.Text != wantStartText || ok.Send.Text == string(body) {
		t.Fatalf("reply text leaked or missing: %#v", ok.Send)
	}
}

func assertOutcome(t *testing.T, got Outcome, status int, provider, chatID string, button bool, mini string) {
	t.Helper()
	if got.Status != status {
		t.Fatalf("status %d, want %d", got.Status, status)
	}
	if chatID == "" {
		if got.Send != nil {
			t.Fatalf("unexpected send: %#v", got.Send)
		}
		return
	}
	if got.Send == nil {
		t.Fatal("missing send")
	}
	if got.Send.Provider != provider || got.Send.ChatID != chatID || got.Send.Text != wantStartText {
		t.Fatalf("outbound %#v", got.Send)
	}
	if button {
		if got.Send.ButtonText != wantButtonText || got.Send.ButtonURL != mini {
			t.Fatalf("button %#v", got.Send)
		}
		return
	}
	if got.Send.ButtonText != "" || got.Send.ButtonURL != "" {
		t.Fatalf("button must be empty: %#v", got.Send)
	}
}
