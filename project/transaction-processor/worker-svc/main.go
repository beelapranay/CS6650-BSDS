package main

import (
	"context"
	"encoding/json"
	"log"

	"transaction-processor/api-svc/models"
	"transaction-processor/worker-svc/db"
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

	log.Println("worker started, polling SQS...")
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
				// Delete malformed messages so they don't block the queue
				if delErr := sqsClient.Delete(ctx, msg.ReceiptHandle); delErr != nil {
					log.Printf("failed to delete bad message: %v", delErr)
				}
				continue
			}

			log.Printf("processing tx %s: %s -> %s (%.2f)",
				req.TransactionID, req.FromAccount, req.ToAccount, req.Amount)

			if err := proc.Process(ctx, req); err != nil {
				log.Printf("processing failed for tx %s: %v — leaving in queue for redelivery", req.TransactionID, err)
				// Do NOT delete — let visibility timeout expire so SQS redelivers
				continue
			}

			// Success — delete from queue
			if err := sqsClient.Delete(ctx, msg.ReceiptHandle); err != nil {
				log.Printf("failed to delete processed message for tx %s: %v", req.TransactionID, err)
			}
		}
	}
}
