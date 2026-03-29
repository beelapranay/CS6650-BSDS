# Distributed Financial Transaction Processor — Milestone 1 Spec

## Goal
Get a working end-to-end pipeline running locally with enough Locust data to produce initial charts for the milestone report.

---

## Stack
- Language: Go
- HTTP Framework: Gin
- Queue: AWS SQS (via LocalStack locally)
- Database: AWS DynamoDB (via LocalStack locally)
- Local Infra: Docker + docker-compose + LocalStack
- Load Testing: Locust (Python)

---

## Repository Structure

```
transaction-processor/
├── api-svc/
│   ├── main.go
│   ├── handlers/
│   │   └── transfer.go
│   ├── models/
│   │   └── transfer.go
│   ├── queue/
│   │   └── sqs.go
│   ├── db/
│   │   └── dynamo.go
│   └── Dockerfile
├── worker-svc/
│   ├── main.go
│   ├── processor/
│   │   └── transfer.go
│   ├── db/
│   │   └── dynamo.go
│   ├── queue/
│   │   └── sqs.go
│   └── Dockerfile
├── scripts/
│   └── seed_accounts.go
├── load-tests/
│   ├── locustfile.py
│   └── verify_balances.py
├── docker-compose.yml
└── README.md
```

---

## Part 1: DynamoDB Schema

### Table: `accounts`
- Partition key: `account_id` (String)
- Attributes: `account_id`, `balance` (Number), `version` (Number)

### Table: `transactions`
- Partition key: `transaction_id` (String)
- Attributes: `transaction_id`, `from_account`, `to_account`, `amount`, `status` (PENDING | COMPLETED | FAILED), `created_at`

---

## Part 2: API Service (`api-svc`)

### `models/transfer.go`
```go
type TransferRequest struct {
    TransactionID string  `json:"transaction_id"` // client-generated UUID
    FromAccount   string  `json:"from_account"`
    ToAccount     string  `json:"to_account"`
    Amount        float64 `json:"amount"`
}

type TransferResponse struct {
    TransactionID string `json:"transaction_id"`
    Status        string `json:"status"`
    Message       string `json:"message"`
}
```

### `handlers/transfer.go`

**POST /transfer**
- Validate request (non-empty fields, amount > 0)
- Idempotency check: if transaction_id already exists in DynamoDB, return existing status
- Write transaction record with status PENDING using `attribute_not_exists(transaction_id)` conditional write
- Enqueue message to SQS as JSON
- Return 202 Accepted with transaction_id

**GET /transfer/:id**
- Look up transaction_id in DynamoDB, return status and details

**GET /health**
- Return 200 OK

### `queue/sqs.go`
- Initialize SQS client via AWS SDK v2, endpoint overrideable via env var (for LocalStack)
- `SendMessage(payload TransferRequest) error`

### `db/dynamo.go`
- Initialize DynamoDB client, endpoint overrideable via env var
- `GetTransaction(id string)`
- `PutTransactionIfNotExists(tx Transaction) error`

---

## Part 3: Worker Service (`worker-svc`)

### `queue/sqs.go`
- Poll SQS in a loop, long polling with WaitTimeSeconds=20
- Process up to 10 messages per batch
- Delete message only after successful processing
- On error, do NOT delete — let visibility timeout expire for SQS redeliver

### `processor/transfer.go`
Optimistic locking transfer logic:
1. Check idempotency — if transaction_id already COMPLETED in DynamoDB, skip
2. Read sender account (balance + version)
3. Check sufficient funds, return FAILED if not
4. Conditional write to debit sender: `SET balance = balance - amount, version = version + 1 WHERE version = :current_version`
5. If conditional write fails (version mismatch), retry up to 3 times with exponential backoff, mark FAILED after retries exhausted
6. Conditional write to credit receiver (same pattern)
7. Update transaction status to COMPLETED

### `db/dynamo.go`
- `GetAccount(id string) (Account, error)`
- `UpdateBalanceOptimistic(id string, newBalance float64, expectedVersion int) error`
- `UpdateTransactionStatus(id string, status string) error`

---

## Part 4: Seed Script (`scripts/seed_accounts.go`)

Standalone Go script:
- Creates 100 accounts in DynamoDB, each with balance=10000.00 and version=0
- Writes account IDs to `accounts.json` for Locust to consume
- Prints total expected balance (100 * 10000 = 1,000,000) for post-test verification

---

## Part 5: Load Tests (`load-tests/`)

### `locustfile.py`

**TransferUser** (normal load):
- Load account IDs from accounts.json
- Pick two random accounts, generate UUID transaction_id, POST /transfer

**HotAccountUser** (Experiment 1 preview):
- Always use account_id = "hot-account-001" as from_account
- Simulates concurrent transfer storm against a single hot account

Run both and collect: requests/sec, p95 latency, failure rate. These are your initial charts.

### `verify_balances.py`
- Scan all accounts in DynamoDB, sum all balances
- Compare to expected total from seed script
- Print accounts with negative balance
- Exit non-zero if total doesn't match

---

## Part 6: docker-compose.yml

Services:
- `localstack`: emulates SQS + DynamoDB locally (image: localstack/localstack)
- `api-svc`: depends on localstack, env vars pointing to localstack endpoint
- `worker-svc`: depends on localstack, same env vars

Add an `init` script or Makefile target that:
1. Starts docker-compose
2. Creates DynamoDB tables via AWS CLI against localstack
3. Creates SQS queue via AWS CLI against localstack
4. Runs seed script

---

## Environment Variables

| Variable | Description |
|---|---|
| `AWS_REGION` | us-east-1 |
| `AWS_ENDPOINT_URL` | http://localstack:4566 (local) / blank on AWS |
| `SQS_QUEUE_URL` | Full SQS queue URL |
| `DYNAMODB_ACCOUNTS_TABLE` | accounts |
| `DYNAMODB_TRANSACTIONS_TABLE` | transactions |
| `PORT` | HTTP port for api-svc (default 8080) |

---

## Dockerfiles

### `api-svc/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o api-svc .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api-svc .
EXPOSE 8080
CMD ["./api-svc"]
```

### `worker-svc/Dockerfile`
Same pattern, binary name = worker-svc, no EXPOSE needed.

---

## Build Order for Claude Code

1. docker-compose.yml + LocalStack init script
2. DynamoDB schema (tables + seed script) — verify with `awslocal dynamodb scan`
3. api-svc: models → db → queue → handlers → main.go
4. worker-svc: db → queue → processor → main.go
5. Smoke test: POST a transfer via curl, verify worker processes it, check balance updated
6. Locust scripts — run TransferUser for 5 min, export charts
7. verify_balances.py — confirm correctness after load test
