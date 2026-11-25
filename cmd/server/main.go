package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"prediksi/internal/notify"
	"prediksi/internal/store"
	"prediksi/internal/web"
	"prediksi/internal/worker"
)

func main() {
	st, err := store.Open(env("DB_USER", "root"), env("DB_PASS", ""), env("DB_ADDR", "127.0.0.1:3306"), env("DB_NAME", "prediksi"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := st.Migrate(); err != nil {
		log.Fatalf("migrasi: %v", err)
	}
	if err := st.Seed(); err != nil {
		log.Fatalf("seed: %v", err)
	}

	tg := notify.NewFromEnv()
	srv, err := web.New(st, tg)
	if err != nil {
		log.Fatalf("template: %v", err)
	}

	addr := env("HTTP_ADDR", "127.0.0.1:8080")
	summaryHour := 8
	if v := os.Getenv("SUMMARY_HOUR"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n < 24 {
			summaryHour = n
		}
	}
	go worker.Run(context.Background(), st, tg, summaryHour)

	log.Printf("Prediksi jalan di http://%s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
