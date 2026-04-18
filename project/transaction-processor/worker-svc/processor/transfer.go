package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"transaction-processor/api-svc/models"
	"transaction-processor/worker-svc/db"
)

const maxRetries = 3

type Processor struct {
	db             *db.DynamoClient
	preCommitDelay time.Duration
}

func NewProcessor(dynamo *db.DynamoClient) *Processor {
	return &Processor{
		db:             dynamo,
		preCommitDelay: loadPreCommitDelay(),
	}
}

func (p *Processor) Process(ctx context.Context, req models.TransferRequest) error {
	tx, err := p.db.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}
	if tx == nil {
		return fmt.Errorf("transaction %s not found", req.TransactionID)
	}
	switch tx.Status {
	case "COMPLETED", "FAILED":
		log.Printf("[processor] tx %s already in terminal state %s", req.TransactionID, tx.Status)
		return nil
	}

	return p.transfer(ctx, req)
}

func (p *Processor) transfer(ctx context.Context, req models.TransferRequest) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := p.attemptTransfer(ctx, req)
		if err == nil {
			return nil
		}

		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
			log.Printf("[processor] version conflict on attempt %d for tx %s, retrying in %v",
				attempt+1, req.TransactionID, backoff)
			if err := sleepWithContext(ctx, backoff); err != nil {
				return err
			}
			continue
		}
		return err
	}

	log.Printf("[processor] max retries exhausted for tx %s, marking FAILED", req.TransactionID)
	return p.failTransfer(ctx, req.TransactionID, "retry budget exhausted")
}

func (p *Processor) attemptTransfer(ctx context.Context, req models.TransferRequest) error {
	sender, err := p.db.GetAccount(ctx, req.FromAccount)
	if err != nil {
		return fmt.Errorf("get sender: %w", err)
	}
	if sender == nil {
		return p.failTransfer(ctx, req.TransactionID, fmt.Sprintf("sender account %s not found", req.FromAccount))
	}

	receiver, err := p.db.GetAccount(ctx, req.ToAccount)
	if err != nil {
		return fmt.Errorf("get receiver: %w", err)
	}
	if receiver == nil {
		return p.failTransfer(ctx, req.TransactionID, fmt.Sprintf("receiver account %s not found", req.ToAccount))
	}

	if sender.Balance < req.Amount {
		return p.failTransfer(ctx, req.TransactionID, fmt.Sprintf(
			"insufficient funds for tx %s: balance=%.2f amount=%.2f",
			req.TransactionID,
			sender.Balance,
			req.Amount,
		))
	}

	if p.preCommitDelay > 0 {
		log.Printf("[processor] delaying tx %s for %v before commit", req.TransactionID, p.preCommitDelay)
		if err := sleepWithContext(ctx, p.preCommitDelay); err != nil {
			return err
		}
	}

	if err := p.db.CompleteTransfer(ctx, req.TransactionID, sender, receiver, req.Amount); err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			tx, txErr := p.db.GetTransaction(ctx, req.TransactionID)
			if txErr == nil && tx != nil {
				switch tx.Status {
				case "COMPLETED", "FAILED":
					log.Printf("[processor] tx %s observed in terminal state %s after redelivery", req.TransactionID, tx.Status)
					return nil
				}
			}
		}
		return fmt.Errorf("complete transfer: %w", err)
	}

	return nil
}

func (p *Processor) failTransfer(ctx context.Context, txID string, reason string) error {
	updated, err := p.db.MarkTransactionFailedIfPending(ctx, txID)
	if err != nil {
		return fmt.Errorf("mark failed for tx %s: %w", txID, err)
	}
	if updated {
		log.Printf("[processor] tx %s marked FAILED: %s", txID, reason)
	} else {
		log.Printf("[processor] tx %s already left PENDING while handling failure: %s", txID, reason)
	}
	return nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func loadPreCommitDelay() time.Duration {
	raw := os.Getenv("PRE_COMMIT_DELAY_MS")
	if raw == "" {
		return 0
	}

	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		log.Printf("[processor] ignoring invalid PRE_COMMIT_DELAY_MS=%q", raw)
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}
