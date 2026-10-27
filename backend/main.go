// Command servys-backend запускает HTTP API личного гаража.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zaaalex/servys/backend/api"
	"github.com/zaaalex/servys/backend/auth"
	"github.com/zaaalex/servys/backend/lane"
	"github.com/zaaalex/servys/backend/messenger"
	"github.com/zaaalex/servys/backend/ocr"
	"github.com/zaaalex/servys/backend/recommender"
	"github.com/zaaalex/servys/backend/store"
	"github.com/zaaalex/servys/backend/vin"
)

func main() {
	dbPath := envOr("DB_PATH", "./data/app.db")
	port := envOr("PORT", "8080")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("mkdir data: %v", err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	runtime, err := recommender.NewFromEnv()
	if err != nil {
		log.Fatalf("recommendation runtime: %v", err)
	}
	vinProvider, err := vin.NewFromEnv(nil)
	if err != nil {
		log.Fatalf("VIN provider: %v", err)
	}
	advisor := runtime.Advisor
	msgCfg := messenger.Config{
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		MaxBotToken:           os.Getenv("MAX_BOT_TOKEN"),
		MaxWebhookSecret:      os.Getenv("MAX_WEBHOOK_SECRET"),
		MaxAPIBase:            os.Getenv("MAX_API_BASE"),
		MiniAppURL:            os.Getenv("MINI_APP_URL"),
	}
	ocrGap, searchGap, err := lane.GapsFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("heavy lane: %v", err)
	}
	heavy := lane.New(ocrGap, searchGap)
	laneCtx, laneCancel := context.WithCancel(context.Background())
	defer laneCancel()
	go heavy.Start(laneCtx)
	runtime.SetHeavyLane(heavy)
	log.Printf("heavy lane: пауза после снимка %s, после поиска %s", ocrGap, searchGap)

	srv := &api.Server{
		Store:     st,
		Adv:       advisor,
		VIN:       vinProvider,
		Messenger: msgCfg,
		Send:      messenger.NewHTTPSender(msgCfg, &http.Client{Timeout: 10 * time.Second}),
		UploadDir: envOr("UPLOAD_DIR", "./data/uploads"),
		Lane:      heavy,
	}
	if os.Getenv("OCR_DISABLE") != "1" {
		script := ocr.FindScript()
		command := ocr.Command{Python: ocr.Python(script), Script: script}
		srv.Scan = command
		log.Printf("ocr: %s %s", command.Python, script)
	}

	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		authSvc := auth.New(st, []byte(secret))
		authSvc.BotToken = msgCfg.TelegramBotToken
		authSvc.MaxBotToken = msgCfg.MaxBotToken
		authSvc.DevLogin = os.Getenv("SERVYS_DEV_LOGIN") == "1"
		srv.Auth = authSvc
		log.Println("auth включён (JWT_SECRET задан)")
	}

	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("servys backend слушает :%s (db=%s)", port, dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	laneCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Println("servys backend остановлен")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
