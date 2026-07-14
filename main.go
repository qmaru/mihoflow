package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"mihoflow/services"
)

const (
	defaultListenAddr   = "127.0.0.1:8080"
	defaultClashURL     = "http://127.0.0.1:9090"
	defaultCollectEvery = time.Second
	defaultFlushEvery   = 5 * time.Minute
	defaultHistoryDays  = 90
	defaultCleanupDays  = 97
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := services.Config{
		ListenAddr:   env("LISTEN_ADDR", defaultListenAddr),
		ClashURL:     env("CLASH_URL", defaultClashURL),
		ClashAPIKey:  env("CLASH_API_KEY", ""),
		DBPath:       env("DB_PATH", "mihoflow.db"),
		Debug:        envBool("DEBUG", false),
		CollectEvery: envDuration("COLLECT_INTERVAL", defaultCollectEvery),
		FlushEvery:   envDuration("FLUSH_INTERVAL", defaultFlushEvery),
		HistoryDays:  envInt("HISTORY_DAYS", defaultHistoryDays),
		CleanupDays:  envInt("CLEANUP_DAYS", defaultCleanupDays),
	}

	service, err := services.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()
	if err := service.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(env(name, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		log.Printf("invalid %s=%q, using %s", name, value, fallback)
		return fallback
	}
	return duration
}
