// Command servys-backend — точка входа (Dev 1): wiring и HTTP-сервер.
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
	"github.com/zaaalex/servys/backend/b2b"
	"github.com/zaaalex/servys/backend/bitrix"
	"github.com/zaaalex/servys/backend/crypto"
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

	// Wiring: боевые Advisor/VINProvider подменит Dev 3 (сейчас — стабы).
	advisor := recommender.NewStubAdvisor()
	srv := &api.Server{
		Store: st,
		Adv:   advisor,
		VIN:   vin.NewStub(),
	}

	// b2b включается только при заданном APP_SECRET_KEY (шифрование вебхуков СТО).
	if secret := os.Getenv("APP_SECRET_KEY"); secret != "" {
		c, err := crypto.New(secret)
		if err != nil {
			log.Fatalf("crypto: %v", err)
		}
		st.SetCipher(c)
		srv.B2B = &b2b.Service{
			Fleet:     bitrix.CRMFleet{},
			Advisor:   advisor, // тот же движок рекомендаций, что и в b2c
			Retention: bitrix.CRMRetention{},
			Dedupe:    st,
		}
		log.Println("b2b включён (APP_SECRET_KEY задан)")
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
