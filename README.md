# CS6650 — Building Scalable Distributed Systems (BSDS)

Coursework repository for **CS6650: Building Scalable Distributed Systems**.

This repo contains homework deliverables, notes/slides, and a larger course project (a local, microservice-style transaction processor).

## Repository layout

- `1a/`, `1b/` — early course assignments
- `hw2/` … `hw10/` — homework submissions
- `crash-demo/` — crash / fault-tolerance demo material
- `humming-bird-midterm/` — midterm work
- `final-mastery/` — final mastery deliverables
- `ppts/` — lecture slides / decks
- `project/` — course project
  - `transaction-processor/` — main project implementation (Go + Docker)

> Note: Some directories also include reports, screenshots, PDFs, and zipped artifacts.

## Project: Transaction Processor

A simple transaction/transfer system built as two services backed by local AWS emulators:

- **api-svc** — HTTP API for creating transfers and checking transfer status
- **worker-svc** — background worker that processes queued transfers
- **DynamoDB Local** — stores `accounts` and `transactions`
- **ElasticMQ (SQS-compatible)** — queue used to decouple request ingestion from processing

### High-level flow

1. Client sends a transfer request to `api-svc`.
2. `api-svc` validates the request, checks idempotency, writes a **PENDING** transaction record to DynamoDB Local, and enqueues a message.
3. `worker-svc` polls the queue, performs balance checks, then applies debit/credit updates using optimistic locking.
4. The transaction is updated to **COMPLETED** (or **FAILED**) and can be queried via the API.

(See `project/transaction-processor/arch.md` for an architecture diagram.)

## Quickstart (transaction-processor)

### Prerequisites

- Docker + Docker Compose
- Go (for local development / scripts)
- AWS CLI (used by the `Makefile` to create tables/queue)
- Python 3 (for the load-test verification script)
- `jq` (used by the smoke test)

### Run locally

```bash
cd project/transaction-processor
make init
```

This will:

- `docker compose up` the services
- create DynamoDB tables: `accounts`, `transactions`
- create the SQS queue: `transfers`
- seed initial account balances

### Smoke test

```bash
cd project/transaction-processor
make smoke
```

### Load test

The Makefile expects a Locust virtualenv at `/tmp/locust-venv`.

```bash
cd project/transaction-processor
make test-load
# or (hot-account contention experiment)
make test-load-hot
```

### Verify balances after a test

```bash
cd project/transaction-processor
make verify
```

### Stop everything

```bash
cd project/transaction-processor
make down
```

## Notes / caveats

- The project uses local endpoints (DynamoDB Local on `localhost:8000`, ElasticMQ on `localhost:9324`).
- Default API port is `8080`.
- Many course artifacts (PDFs, PPTX, ZIPs) are committed for convenience.

## License

No license specified. If you intend this to be reused publicly, consider adding a license file.