DYNAMODB_ENDPOINT=http://localhost:8000
SQS_ENDPOINT=http://localhost:9324
SQS_QUEUE_URL=http://localhost:9324/000000000000/transfers

AWS_ENV=AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_DEFAULT_REGION=us-east-1

DYNAMO_FLAGS=--endpoint-url=$(DYNAMODB_ENDPOINT) --region us-east-1
SQS_FLAGS=--endpoint-url=$(SQS_ENDPOINT) --region us-east-1

.PHONY: init up down deps seed test-load verify smoke

## Bring up all services
up:
	docker compose up -d --build

## Tear down all services
down:
	docker compose down -v

## Full init: start compose, create infra, seed data
init: up wait-infra create-tables create-queue seed

wait-infra:
	@echo "Waiting for DynamoDB Local..."
	@until curl -s $(DYNAMODB_ENDPOINT)/ > /dev/null 2>&1; do sleep 2; done
	@echo "DynamoDB Local ready."
	@echo "Waiting for ElasticMQ..."
	@until curl -s $(SQS_ENDPOINT)/ > /dev/null 2>&1; do sleep 2; done
	@echo "ElasticMQ ready."

create-tables:
	@echo "Creating DynamoDB tables..."
	$(AWS_ENV) aws $(DYNAMO_FLAGS) dynamodb create-table \
		--table-name accounts \
		--attribute-definitions AttributeName=account_id,AttributeType=S \
		--key-schema AttributeName=account_id,KeyType=HASH \
		--billing-mode PAY_PER_REQUEST || true
	$(AWS_ENV) aws $(DYNAMO_FLAGS) dynamodb create-table \
		--table-name transactions \
		--attribute-definitions AttributeName=transaction_id,AttributeType=S \
		--key-schema AttributeName=transaction_id,KeyType=HASH \
		--billing-mode PAY_PER_REQUEST || true
	@echo "Tables created."

create-queue:
	@echo "Creating SQS queue..."
	$(AWS_ENV) aws $(SQS_FLAGS) sqs create-queue --queue-name transfers || true
	@echo "Queue created."

seed:
	@echo "Seeding accounts..."
	AWS_REGION=us-east-1 \
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	DYNAMODB_ENDPOINT_URL=$(DYNAMODB_ENDPOINT) \
	DYNAMODB_ACCOUNTS_TABLE=accounts \
	go run ./scripts/seed_accounts.go

## Download Go dependencies
deps:
	go mod tidy
	go mod download

## Smoke test: POST a transfer and check status
smoke:
	@echo "Posting transfer..."
	@TXN_ID=$$(python3 -c "import uuid; print(str(uuid.uuid4()))"); \
	curl -s -X POST http://localhost:8080/transfer \
		-H "Content-Type: application/json" \
		-d "{\"transaction_id\":\"$$TXN_ID\",\"from_account\":\"account-0\",\"to_account\":\"account-1\",\"amount\":10.0}" | jq .; \
	sleep 2; \
	curl -s http://localhost:8080/transfer/$$TXN_ID | jq .

LOCUST=/tmp/locust-venv/bin/locust

## Run Locust load test — TransferUser (normal mixed load)
test-load:
	cd load-tests && $(LOCUST) -f locustfile.py --host=http://localhost:8080 \
		--users 50 --spawn-rate 5 --run-time 5m --headless \
		--csv=results --html=report.html

## Run Locust load test — HotAccountUser only (Experiment 1: hot-account contention)
test-load-hot:
	cd load-tests && $(LOCUST) -f locustfile.py HotAccountUser --host=http://localhost:8080 \
		--users 50 --spawn-rate 5 --run-time 5m --headless \
		--csv=results-hot --html=report-hot.html

## Verify balance correctness after load test
verify:
	DYNAMODB_ENDPOINT_URL=$(DYNAMODB_ENDPOINT) \
	AWS_REGION=us-east-1 \
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	DYNAMODB_ACCOUNTS_TABLE=accounts \
	/tmp/locust-venv/bin/python3 load-tests/verify_balances.py
