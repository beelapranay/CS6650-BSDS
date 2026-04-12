package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"final-mastery/internal/albumstore"
)

func main() {
	log.SetFlags(log.LstdFlags)

	host := envOrDefault("HOST", "0.0.0.0")
	port := envOrDefault("PORT", "8000")
	dataDir := os.Getenv("DATA_DIR")

	service, err := albumstore.BuildServiceFromEnv(dataDir, albumstore.MaxUploadBytesFromEnv())
	if err != nil {
		log.Fatalf("build service: %v", err)
	}
	defer service.Close()

	app := albumstore.NewApp(service, albumstore.PublicBaseURLFromEnv())
	server := &http.Server{
		Addr:              host + ":" + port,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       180 * time.Second,
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Printf("serving album store on http://%s:%s", host, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt64(name string, fallback int64) int64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
