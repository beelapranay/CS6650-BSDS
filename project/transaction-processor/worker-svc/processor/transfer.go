package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"transaction-processor/api-svc/models"
	"transaction-processor/worker-svc/db"
)

const maxRetries = 3

const (
	lockingModeOptimistic  = "optimistic"
	lockingModePessimistic = "pessimistic"
)

type Processor struct {
	db             *db.DynamoClient
	preCommitDelay time.Duration
	lockingMode    string
	lockTTL        time.Duration
}

func NewProcessor(dynamo *db.DynamoClient) *Processor {
	return &Processor{
		db:             dynamo,
		preCommitDelay: loadPreCommitDelay(),
		lockingMode:    loadLockingMode(),
		lockTTL:        loadLockTTL(),
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

		if isRetryableConcurrencyError(err) {
			backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
			log.Printf("[processor] retryable concurrency conflict on attempt %d for tx %s in %s mode, retrying in %v",
				attempt+1, req.TransactionID, p.lockingMode, backoff)
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
	switch p.lockingMode {
	case lockingModePessimistic:
		return p.attemptTransferPessimistic(ctx, req)
	default:
		return p.attemptTransferOptimistic(ctx, req)
	}
}

func (p *Processor) attemptTransferOptimistic(ctx context.Context, req models.TransferRequest) error {
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

func (p *Processor) attemptTransferPessimistic(ctx context.Context, req models.TransferRequest) error {
	lockIDs := orderedAccountIDs(req.FromAccount, req.ToAccount)
	acquired := make([]string, 0, len(lockIDs))
	committed := false

	defer func() {
		if committed {
			return
		}
		for i := len(acquired) - 1; i >= 0; i-- {
			if err := p.db.ReleaseAccountLock(ctx, acquired[i], req.TransactionID); err != nil {
				log.Printf("[processor] failed to release lock on %s for tx %s: %v", acquired[i], req.TransactionID, err)
			}
		}
	}()

	for _, accountID := range lockIDs {
		if err := p.db.AcquireAccountLock(ctx, accountID, req.TransactionID, time.Now().UTC(), p.lockTTL); err != nil {
			return fmt.Errorf("acquire lock for %s: %w", accountID, err)
		}
		acquired = append(acquired, accountID)
	}

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

	if err := p.db.CompleteTransferPessimistic(ctx, req.TransactionID, sender, receiver, req.Amount); err != nil {
		if isRetryableConcurrencyError(err) {
			tx, txErr := p.db.GetTransaction(ctx, req.TransactionID)
			if txErr == nil && tx != nil {
				switch tx.Status {
				case "COMPLETED", "FAILED":
					log.Printf("[processor] tx %s observed in terminal state %s after redelivery", req.TransactionID, tx.Status)
					return nil
				}
			}
		}
		return fmt.Errorf("complete pessimistic transfer: %w", err)
	}

	committed = true
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
	if err != nil || ms < 0 {
		log.Printf("[processor] ignoring invalid PRE_COMMIT_DELAY_MS=%q", raw)
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func loadLockingMode() string {
	mode := os.Getenv("LOCKING_MODE")
	switch mode {
	case "", lockingModeOptimistic:
		return lockingModeOptimistic
	case lockingModePessimistic:
		return lockingModePessimistic
	default:
		log.Printf("[processor] ignoring invalid LOCKING_MODE=%q; defaulting to %s", mode, lockingModeOptimistic)
		return lockingModeOptimistic
	}
}

func loadLockTTL() time.Duration {
	raw := os.Getenv("LOCK_TTL_SECONDS")
	if raw == "" {
		return 90 * time.Second
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		log.Printf("[processor] ignoring invalid LOCK_TTL_SECONDS=%q; defaulting to 90", raw)
		return 90 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func orderedAccountIDs(first, second string) []string {
	ids := []string{first, second}
	slices.Sort(ids)
	return ids
}

func isRetryableConcurrencyError(err error) bool {
	var condErr *types.ConditionalCheckFailedException
	if errors.As(err, &condErr) {
		return true
	}

	var txErr *types.TransactionCanceledException
	if errors.As(err, &txErr) {
		for _, reason := range txErr.CancellationReasons {
			if reason.Code != nil && *reason.Code == "ConditionalCheckFailed" {
				return true
			}
		}
	}
	return false
}
