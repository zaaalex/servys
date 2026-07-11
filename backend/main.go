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

	// Wiring: боевые Recommender/VINProvider подменит Dev 3 (сейчас — стабы).
	srv := &api.Server{
		Store: st,
		Rec:   recommender.NewStub(),
		VIN:   vin.NewStub(),
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
