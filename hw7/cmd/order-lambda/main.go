package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"hw7/internal/orders"
)

var (
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	delay  = durationEnv("PAYMENT_DELAY", 3*time.Second)
)

func main() {
	lambda.Start(handle)
}

func handle(ctx context.Context, event events.SNSEvent) error {
	simulator := orders.NewPaymentSimulator("lambda", 1, delay, logger)

	for _, record := range event.Records {
		var order orders.Order
		if err := json.Unmarshal([]byte(record.SNS.Message), &order); err != nil {
			logger.ErrorContext(ctx, "failed to decode SNS order message", "error", err)
			return err
		}

		order.Status = orders.StatusProcessing
		if err := simulator.Process(ctx, order); err != nil {
			return err
		}
		order.Status = orders.StatusCompleted
		logger.InfoContext(ctx, "lambda completed order", "order_id", order.OrderID)
	}

	return nil
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
