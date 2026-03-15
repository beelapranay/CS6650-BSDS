package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"hw7/internal/processor"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	queueURL := os.Getenv("ORDER_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("ORDER_QUEUE_URL must be set")
	}

	workerCount := intEnv("PROCESSOR_WORKERS", 1)
	delay := durationEnv("PAYMENT_DELAY", 3*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := processor.New(ctx, logger, queueURL, workerCount, delay)
	if err != nil {
		log.Fatal(err)
	}

	logger.Info("starting order processor", "worker_count", workerCount, "queue_url", queueURL, "payment_delay", delay.String())

	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}
