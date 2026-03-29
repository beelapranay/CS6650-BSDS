package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"transaction-processor/api-svc/db"
	"transaction-processor/api-svc/handlers"
	"transaction-processor/api-svc/queue"
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

	h := handlers.NewHandler(dynamo, sqsClient)

	r := gin.Default()
	r.GET("/health", handlers.HealthCheck)
	r.POST("/transfer", h.PostTransfer)
	r.GET("/transfer/:id", h.GetTransfer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
