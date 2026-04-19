package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"transaction-processor/api-svc/models"
	"transaction-processor/worker-svc/db"
	"transaction-processor/worker-svc/metrics"
	"transaction-processor/worker-svc/processor"
	"transaction-processor/worker-svc/queue"
)

func main() {
	ctx := context.Background()

	dynamo, err := db.NewDynamoClient(ctx)
	if err != nil {
		log.Fatalf("failed to init DynamoDB client: %v", err)
	}

	sqsClient, err := queue.NewSQSClient(ctx)
	if err != nil {
		log.Fatalf("failed to init SQS client: %v", err)
	}

	proc := processor.NewProcessor(dynamo)

	go serveMetrics()
	go logMetricsPeriodically(ctx)

	log.Printf("worker started, polling SQS with LOCKING_MODE=%s", getEnvOrDefault("LOCKING_MODE", "optimistic"))
	for {
		msgs, err := sqsClient.Receive(ctx)
		if err != nil {
			log.Printf("error receiving messages: %v", err)
			continue
		}

		for _, msg := range msgs {
			var req models.TransferRequest
			if err := json.Unmarshal([]byte(msg.Body), &req); err != nil {
				log.Printf("failed to parse message: %v — skipping", err)
				if delErr := sqsClient.Delete(ctx, msg.ReceiptHandle); delErr != nil {
					log.Printf("failed to delete bad message: %v", delErr)
				}
				continue
			}

			log.Printf("processing tx %s: %s -> %s (%.2f)",
				req.TransactionID, req.FromAccount, req.ToAccount, req.Amount)

			if err := proc.Process(ctx, req); err != nil {
				log.Printf("processing failed for tx %s: %v — leaving in queue for redelivery", req.TransactionID, err)
				continue
			}

			if err := sqsClient.Delete(ctx, msg.ReceiptHandle); err != nil {
				log.Printf("failed to delete processed message for tx %s: %v", req.TransactionID, err)
			}
		}
	}
}

func serveMetrics() {
	addr := ":" + getEnvOrDefault("METRICS_PORT", "9090")
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/metrics.json", metrics.JSONHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("[metrics] serving on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("[metrics] server exited: %v", err)
	}
}

func logMetricsPeriodically(ctx context.Context) {
	interval := parseDurationSeconds(os.Getenv("METRICS_LOG_INTERVAL_SECONDS"), 30*time.Second)
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := metrics.Default.Snapshot()
			payload, _ := json.Marshal(map[string]interface{}{
				"event":   "metrics_snapshot",
				"metrics": snap,
			})
			log.Printf("METRICS %s", payload)
		}
	}
}

func parseDurationSeconds(raw string, def time.Duration) time.Duration {
	if raw == "" {
		return def
	}
	secs, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
