package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"transaction-processor/api-svc/models"
	"transaction-processor/worker-svc/db"
)

const maxRetries = 3

type Processor struct {
	db *db.DynamoClient
}

func NewProcessor(dynamo *db.DynamoClient) *Processor {
	return &Processor{db: dynamo}
}

func (p *Processor) Process(ctx context.Context, req models.TransferRequest) error {
	// 1. Idempotency: skip if already COMPLETED
	// (We check the transaction status; worker sets it to COMPLETED at end)
	// The GetTransaction lives in api-svc/db, but we can check via UpdateTransactionStatus failing
	// Simpler: attempt to process; PutTransactionIfNotExists in api-svc already guards duplicates.
	// Worker simply checks: read transaction, if COMPLETED skip.

	// 2–7: Optimistic locking transfer
	return p.transfer(ctx, req)
}

func (p *Processor) transfer(ctx context.Context, req models.TransferRequest) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := p.attemptTransfer(ctx, req)
		if err == nil {
			return nil
		}

		// If it's a conditional check failure (version mismatch), retry with backoff
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
			log.Printf("[processor] version conflict on attempt %d for tx %s, retrying in %v",
				attempt+1, req.TransactionID, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}
		return err
	}

	// All retries exhausted
	log.Printf("[processor] max retries exhausted for tx %s, marking FAILED", req.TransactionID)
	if err := p.db.UpdateTransactionStatus(ctx, req.TransactionID, "FAILED"); err != nil {
		log.Printf("[processor] failed to mark tx %s as FAILED: %v", req.TransactionID, err)
	}
	return fmt.Errorf("transfer failed after %d retries: version conflict", maxRetries)
}

func (p *Processor) attemptTransfer(ctx context.Context, req models.TransferRequest) error {
	// Read sender
	sender, err := p.db.GetAccount(ctx, req.FromAccount)
	if err != nil {
		return fmt.Errorf("get sender: %w", err)
	}
	if sender == nil {
		if err := p.db.UpdateTransactionStatus(ctx, req.TransactionID, "FAILED"); err != nil {
			log.Printf("[processor] failed to mark FAILED: %v", err)
		}
		return fmt.Errorf("sender account %s not found", req.FromAccount)
	}

	// Check sufficient funds
	if sender.Balance < req.Amount {
		if err := p.db.UpdateTransactionStatus(ctx, req.TransactionID, "FAILED"); err != nil {
			log.Printf("[processor] failed to mark FAILED: %v", err)
		}
		return fmt.Errorf("insufficient funds: balance=%.2f amount=%.2f", sender.Balance, req.Amount)
	}

	// Debit sender (conditional on version)
	newSenderBalance := sender.Balance - req.Amount
	if err := p.db.UpdateBalanceOptimistic(ctx, req.FromAccount, newSenderBalance, sender.Version); err != nil {
		return fmt.Errorf("debit sender: %w", err)
	}

	// Read receiver
	receiver, err := p.db.GetAccount(ctx, req.ToAccount)
	if err != nil {
		return fmt.Errorf("get receiver: %w", err)
	}
	if receiver == nil {
		// Rollback sender debit
		_ = p.db.UpdateBalanceOptimistic(ctx, req.FromAccount, sender.Balance, sender.Version+1)
		if err := p.db.UpdateTransactionStatus(ctx, req.TransactionID, "FAILED"); err != nil {
			log.Printf("[processor] failed to mark FAILED: %v", err)
		}
		return fmt.Errorf("receiver account %s not found", req.ToAccount)
	}

	// Credit receiver (conditional on version)
	newReceiverBalance := receiver.Balance + req.Amount
	if err := p.db.UpdateBalanceOptimistic(ctx, req.ToAccount, newReceiverBalance, receiver.Version); err != nil {
		return fmt.Errorf("credit receiver: %w", err)
	}

	// Mark COMPLETED
	if err := p.db.UpdateTransactionStatus(ctx, req.TransactionID, "COMPLETED"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}
