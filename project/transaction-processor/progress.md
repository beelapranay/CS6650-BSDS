# Milestone 1 — Progress Log

## Status: COMPLETE

---

## What Was Built

### 1. Infrastructure (`docker-compose.yml`, `Makefile`)
- `docker-compose.yml` — four services: `dynamodb-local` (Amazon DynamoDB Local), `elasticmq` (SQS-compatible), `api-svc`, `worker-svc`. App services restart on failure.
  - **Note:** LocalStack `latest` (2026.x) now requires a paid license. Replaced with `amazon/dynamodb-local` (port 8000) and `softwaremill/elasticmq-native` (port 9324) — both free and purpose-built.
- `Makefile` — targets:
  - `make init` → brings up compose, waits for both infra services, creates DynamoDB tables (`accounts`, `transactions`), creates SQS queue (`transfers`), runs seed script
  - `make smoke` → posts a single transfer via curl and polls for its status
  - `make test-load` → headless Locust run, both user classes (50 users, 5 min, exports CSV + HTML to `load-tests/results*`, `load-tests/report.html`)
  - `make test-load-hot` → headless Locust run, `HotAccountUser` only (Experiment 1 isolation)
  - `make verify` → runs balance verifier
  - `make up / down / deps` — utility targets

---

### 2. Go Module (`go.mod`, `go.sum`)
- Single module `transaction-processor` at the repo root (both services share it)
- Direct dependencies: `aws-sdk-go-v2` (config, credentials, dynamodb, sqs, attributevalue), `gin`
- `go mod tidy` run — `go.sum` generated, all transitive deps pinned
- `go build ./api-svc/... ./worker-svc/...` and `go vet` both pass clean

---

### 3. API Service (`api-svc/`)

| File | What it does |
|------|---|
| `models/transfer.go` | `TransferRequest`, `TransferResponse`, `Transaction` structs (shared with worker via import) |
| `db/dynamo.go` | `GetTransaction`, `PutTransactionIfNotExists` — conditional write with `attribute_not_exists(transaction_id)` for idempotency |
| `queue/sqs.go` | `SendMessage` — marshals `TransferRequest` to JSON and enqueues |
| `handlers/transfer.go` | `POST /transfer`: validates → idempotency check → write PENDING → enqueue → 202; `GET /transfer/:id`: lookup + return; `GET /health`: 200 |
| `main.go` | Initialises DynamoDB + SQS clients, wires Gin routes, starts on `$PORT` (default 8080) |
| `Dockerfile` | Multi-stage build from `golang:1.22-alpine`; final image is bare `alpine` |

---

### 4. Worker Service (`worker-svc/`)

| File | What it does |
|------|---|
| `db/dynamo.go` | `GetAccount` (consistent read), `UpdateBalanceOptimistic` (version-conditional UpdateItem with `#ver = :expected_version`), `UpdateTransactionStatus` |
| `queue/sqs.go` | Long-poll receive (WaitTimeSeconds=20, batch=10), `Delete` on success |
| `processor/transfer.go` | Full optimistic-locking transfer logic: idempotency guard → check funds → debit sender (conditional) → credit receiver (conditional) → mark COMPLETED. Retries up to 3× with exponential backoff on `ConditionalCheckFailedException`; marks FAILED after retries exhausted |
| `main.go` | Infinite poll loop — malformed messages deleted immediately; processing failures left in queue for SQS redelivery |
| `Dockerfile` | Same multi-stage pattern; no EXPOSE |

---

### 5. Seed Script (`scripts/seed_accounts.go`)
- Standalone `go run ./scripts/seed_accounts.go`
- Creates 100 accounts (`account-0` … `account-99`) + `hot-account-001`, each with `balance=10000.00`, `version=0`
- Writes all account IDs to `load-tests/accounts.json` for Locust to consume
- Prints expected total balance: `1,010,000.00`

---

### 6. Load Tests (`load-tests/`)

**`locustfile.py`**
- `TransferUser` — picks two random accounts from `accounts.json`, generates a UUID `transaction_id`, POSTs to `/transfer`. Simulates normal mixed load.
- `HotAccountUser` — always uses `hot-account-001` as `from_account`. Simulates the hot-account contention scenario (Experiment 1), driving the optimistic-locking retry path.

**`verify_balances.py`**
- Full DynamoDB scan of the `accounts` table
- Sums all balances, compares to expected total (101 accounts × 10,000 = 1,010,000) with 1-cent tolerance for float64 accumulation
- Prints accounts with negative balance
- Exits non-zero if difference > $0.01 or any negative balance found

---

## Load Test Results

### Run 1 — Mixed Load (TransferUser + HotAccountUser, 50 users, 5 min)

| Metric | `/transfer` (random) | `/transfer [hot]` | Aggregated |
|--------|---------------------|-------------------|------------|
| Requests | 24,247 | 56,949 | 81,196 |
| Failures | 0 (0.00%) | 0 (0.00%) | 0 (0.00%) |
| Avg latency | 4 ms | 4 ms | 4 ms |
| p50 | 3 ms | 3 ms | 3 ms |
| p95 | 11 ms | 10 ms | 10 ms |
| p99 | 35 ms | 31 ms | 32 ms |
| req/s | 80.88 | 189.96 | 270.84 |

**Balance verify after Run 1:** PASSED — difference `1.097e-11` (float64 noise, well within $0.01 tolerance)

### Run 2 — Hot Account Only (HotAccountUser, 50 users, 5 min)

| Metric | `/transfer [hot]` |
|--------|-------------------|
| Requests | 114,021 |
| Failures | 0 (0.00%) |
| Avg latency | 3 ms |
| p50 | 2 ms |
| p95 | 7 ms |
| p99 | 28 ms |
| req/s | 380.37 |

**Key observation:** Even with 50 concurrent users hammering a single sender account, p95 stays at 7 ms and failure rate is 0%. The optimistic-locking retry loop absorbs all version conflicts without surfacing errors to the client.

---

## Completed Checklist

- [x] `make init` — LocalStack replaced, tables + queue created, 101 accounts seeded
- [x] Smoke test — single transfer confirmed PENDING → COMPLETED end-to-end
- [x] `make test-load` — 5 min mixed run, 81,196 requests, 0 failures, p95 10 ms, ~271 req/s
- [x] `make test-load-hot` — 5 min hot-account run, 114,021 requests, 0 failures, p95 7 ms, ~380 req/s
- [x] `make verify` — balance correctness confirmed post-load (diff < 1e-10, within tolerance)
- [x] HTML reports exported: `load-tests/report.html` (mixed), `load-tests/report-hot.html` (hot)
- [x] CSV stats exported: `load-tests/results_*.csv`, `load-tests/results-hot_*.csv`
