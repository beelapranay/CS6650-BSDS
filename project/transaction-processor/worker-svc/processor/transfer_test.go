package processor

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestOrderedAccountIDs(t *testing.T) {
	got := orderedAccountIDs("account-9", "account-1")
	if got[0] != "account-1" || got[1] != "account-9" {
		t.Fatalf("orderedAccountIDs returned %v", got)
	}
}

func TestLoadLockingModeDefaultsToOptimistic(t *testing.T) {
	t.Setenv("LOCKING_MODE", "")
	if got := loadLockingMode(); got != lockingModeOptimistic {
		t.Fatalf("expected %q, got %q", lockingModeOptimistic, got)
	}
}

func TestLoadLockingModeAcceptsPessimistic(t *testing.T) {
	t.Setenv("LOCKING_MODE", lockingModePessimistic)
	if got := loadLockingMode(); got != lockingModePessimistic {
		t.Fatalf("expected %q, got %q", lockingModePessimistic, got)
	}
}

func TestLoadPreCommitDelayAllowsZero(t *testing.T) {
	t.Setenv("PRE_COMMIT_DELAY_MS", "0")
	if got := loadPreCommitDelay(); got != 0 {
		t.Fatalf("expected zero delay, got %v", got)
	}
}

func TestLoadLockTTLDefaultsOnInvalidInput(t *testing.T) {
	t.Setenv("LOCK_TTL_SECONDS", "-1")
	if got := loadLockTTL(); got != 90*time.Second {
		t.Fatalf("expected default lock ttl, got %v", got)
	}
}

func TestIsRetryableConcurrencyError(t *testing.T) {
	condErr := &types.ConditionalCheckFailedException{}
	if !isRetryableConcurrencyError(condErr) {
		t.Fatal("expected conditional check error to be retryable")
	}

	txErr := &types.TransactionCanceledException{
		CancellationReasons: []types.CancellationReason{
			{Code: strPtr("ConditionalCheckFailed")},
		},
	}
	if !isRetryableConcurrencyError(txErr) {
		t.Fatal("expected transaction canceled conditional check to be retryable")
	}

	if isRetryableConcurrencyError(errors.New("boom")) {
		t.Fatal("expected generic error not to be retryable")
	}
}

func strPtr(v string) *string {
	return &v
}
