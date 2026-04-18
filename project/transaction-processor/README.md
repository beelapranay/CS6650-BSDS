# Transaction Processor

Distributed financial transaction processor for CS6650. The system exposes an HTTP API that records transfers, queues them asynchronously, and applies them with optimistic concurrency control in DynamoDB.

## Current milestone status

Weeks 1-2 completed:
- API and worker services are scaffolded in Go.
- Transfers are recorded as `PENDING`, enqueued to SQS, and processed asynchronously.
- Idempotent request submission exists at the API layer.
- Local correctness verification exists through `load-tests/verify_balances.py`.

Weeks 3-4 completed in this repo:
- Docker Compose runs the full system locally.
- Locust load tests cover both mixed traffic and hot-account contention.
- Crash-recovery safety now uses a single DynamoDB transaction for debit, credit, and transaction completion, so a worker crash cannot leave a half-applied transfer behind.
- Terraform under [`infra/`](./infra) provisions ECR, ECS/Fargate, ALB, SQS, and DynamoDB for AWS deployment.
- Cloud bootstrap scripts handle image push, account seeding, and smoke testing.

Not implemented yet:
- Week 5-6 pessimistic-locking path and comparative experiment.
- Week 5-6 horizontal scaling experiment results.
- Week 7-8 analysis artifacts beyond the draft reports already in the repository.

## Local workflow

Prerequisites:
- Docker
- Go 1.22+
- AWS CLI
- Terraform
- Locust installed at `/tmp/locust-venv/bin/locust`

Bring everything up locally:

```bash
make init
make smoke
make test-load
make verify
```

Run the week 3-4 crash-recovery experiment locally:

```bash
PRE_COMMIT_DELAY_MS=8000 docker compose up -d --build
make create-tables create-queue seed
```

Then:
1. Submit a transfer.
2. Kill `worker-svc` during the artificial pre-commit delay window.
3. Let Docker restart the worker and allow the SQS message to redeliver.
4. Confirm the transfer reaches a terminal state and run `make verify`.

Because the worker now commits debit, credit, and transaction status in one DynamoDB transaction, redelivery after a crash is safe.

## Cloud deployment

1. Copy [`infra/terraform.tfvars.example`](./infra/terraform.tfvars.example) to `infra/terraform.tfvars` and set your AWS account details.
2. Run `make tf-init` and `make tf-apply`.
3. Run `make push-images` to build and push both images to ECR.
4. Re-run `make tf-apply` so ECS picks up the pushed image tags if needed.
5. Run `make cloud-seed`.
6. Run `make cloud-smoke`.

Useful Terraform outputs:
- `base_url`
- `api_ecr_repository_url`
- `worker_ecr_repository_url`
- `accounts_table_name`
- `transactions_table_name`
- `queue_url`

## Files

- [`api-svc/`](./api-svc): HTTP API for transfer submission and status lookup
- [`worker-svc/`](./worker-svc): Asynchronous transfer processor
- [`load-tests/`](./load-tests): Locust workloads and balance verification
- [`scripts/`](./scripts): seed, push-image, and smoke-test helpers
- [`infra/`](./infra): AWS Terraform for week 3-4 deployment
