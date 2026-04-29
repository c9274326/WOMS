package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/c9274326/woms/internal/api"
)

func main() {
	addr := env("HTTP_ADDR", ":8080")
	jwtSecret := env("JWT_SECRET", "change-me-in-production")
	store := api.NewMemoryStore()
	if env("DEMO_SEED_DATA", "true") != "false" {
		store = api.NewDemoMemoryStore()
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(jwtSecret, store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("woms api listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("api server failed: %v", err)
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
