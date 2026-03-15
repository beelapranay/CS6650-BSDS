package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"hw7/internal/messaging"
	"hw7/internal/orders"
	"hw7/internal/receiver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := envOrDefault("LISTEN_ADDR", ":8080")
	delay := durationEnv("PAYMENT_DELAY", 3*time.Second)
	syncWorkers := intEnv("SYNC_PAYMENT_WORKERS", 1)
	topicARN := os.Getenv("ORDER_EVENTS_TOPIC_ARN")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	publisher, err := messaging.NewPublisher(ctx, topicARN, logger)
	if err != nil {
		log.Fatal(err)
	}

	server := receiver.NewServer(
		logger,
		publisher,
		orders.NewPaymentSimulator("sync-api", syncWorkers, delay, logger),
	)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting order receiver", "addr", addr, "sync_payment_workers", syncWorkers, "payment_delay", delay.String())

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
