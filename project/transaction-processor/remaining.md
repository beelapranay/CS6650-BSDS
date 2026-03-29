# Milestone 1 — Remaining Steps

## Step 1: Bring Up LocalStack and Init

```bash
make init
```

Verify:
- Both DynamoDB tables exist: `awslocal dynamodb list-tables`
- SQS queue exists: `awslocal sqs list-queues`
- Seed script ran: `awslocal dynamodb scan --table-name accounts | grep Count` should show 101 items
- `load-tests/accounts.json` exists and has 101 account IDs

---

## Step 2: Smoke Test

```bash
make smoke
```

Expected:
- POST /transfer returns 202 with a transaction_id
- GET /transfer/:id eventually returns status COMPLETED
- Worker logs show the transfer being processed

If this fails, the end-to-end pipeline is broken and load testing is pointless. Fix before moving on.

---

## Step 3: Load Test — Normal Mixed Load (TransferUser)

Run Locust with `TransferUser` only, headless, 50 users, 5 minutes:

```bash
make test-load
```

Collect and save:
- `reports/normal-load/locust_report.html`
- `reports/normal-load/locust_stats.csv`
- `reports/normal-load/locust_failures.csv`

Key numbers to note manually:
- Peak requests/sec
- p95 latency
- Failure rate %

---

## Step 4: Load Test — Hot Account Contention (HotAccountUser)

Edit `Makefile` or run Locust directly to use `HotAccountUser` class instead of `TransferUser`. Same params: 50 users, 5 minutes.

```bash
locust -f load-tests/locustfile.py --headless -u 50 -r 5 --run-time 5m \
  --host http://localhost:8080 \
  --user-classes HotAccountUser \
  --html reports/hot-account/locust_report.html \
  --csv reports/hot-account/locust
```

This drives the optimistic locking retry path hard. Collect same metrics as Step 3.

---

## Step 5: Verify Balance Correctness

Run after each load test, not just once:

```bash
make verify
```

Expected output:
- Total balance matches 1,010,000.00
- No accounts with negative balance
- Exit code 0

If totals don't match, there is a bug in the optimistic locking logic. Do not proceed to the milestone report until this passes cleanly after both load test runs.

---

## Step 6: Worker Logs — Check Retry Behavior

After the hot-account load test, grep worker logs for retry and failure signals:

```bash
docker logs transaction-processor-worker-svc-1 2>&1 | grep -E "retry|FAILED|ConditionalCheck"
```

Note how many retries were triggered — this is data for the milestone report and previews Experiment 3.

---

## Step 7: Export Charts for Milestone Report

From the two Locust HTML reports, screenshot or export:
1. Requests/sec over time (normal load vs hot account — shows contention impact)
2. Response time percentiles over time
3. Failure count over time

These are your "initial charts" for the milestone submission.

---

## Step 8: README

Write a short `README.md` covering:
- What the project is (2-3 sentences)
- How to run locally: `make init` → `make smoke` → `make test-load` → `make verify`
- Environment variables table (copy from build spec)
- Link to the Piazza proposal post

---

## Milestone 1 Submission Checklist

- [ ] GitHub repo link (make sure it's not private)
- [ ] `make init && make smoke` works end-to-end cleanly
- [ ] Normal load test report saved under `reports/normal-load/`
- [ ] Hot account load test report saved under `reports/hot-account/`
- [ ] `make verify` passes with correct totals after both runs
- [ ] Charts exported for the report
- [ ] README complete
- [ ] 5-page report written (separate from this codebase)
- [ ] 2-minute video recorded
