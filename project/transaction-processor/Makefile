# Cloud-only workflow. All targets operate against AWS via LabRole.
# Nothing in this Makefile runs services locally.

TF_CHDIR ?= infra
LOCUST ?= ./scripts/run_locust.sh
VERIFY_PYTHON ?= $(shell if [ -x /tmp/locust-venv/bin/python3 ]; then printf /tmp/locust-venv/bin/python3; else command -v python3; fi)

# Resolve AWS coordinates from terraform outputs at make-time only when a
# target needs them. Keeps targets usable even before tf-apply.
AWS_REGION_CMD      = terraform -chdir=$(TF_CHDIR) output -raw aws_region
ALB_URL_CMD         = terraform -chdir=$(TF_CHDIR) output -raw base_url
ACCOUNTS_TABLE_CMD  = terraform -chdir=$(TF_CHDIR) output -raw accounts_table_name
QUEUE_URL_CMD       = terraform -chdir=$(TF_CHDIR) output -raw queue_url

.PHONY: help deps preflight tf-init tf-plan tf-apply tf-destroy \
        push-images cloud-seed cloud-smoke cloud-test-load cloud-test-load-hot \
        cloud-verify cloud-compare-locking cloud-crash-test scale-experiment \
        charts worker-metrics-url clean-experiments

help:
	@awk 'BEGIN{FS=":.*?## "} /^[a-zA-Z_-]+:.*## / {printf "  %-26s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download Go dependencies
	go mod tidy
	go mod download

preflight: ## Verify current AWS credentials are a LabRole session
	./scripts/preflight_labrole.sh

tf-init: preflight ## terraform init
	terraform -chdir=$(TF_CHDIR) init

tf-plan: preflight ## terraform plan
	terraform -chdir=$(TF_CHDIR) plan

tf-apply: preflight ## terraform apply
	terraform -chdir=$(TF_CHDIR) apply

tf-destroy: preflight ## terraform destroy
	terraform -chdir=$(TF_CHDIR) destroy

push-images: preflight ## Build + push api and worker images to ECR
	./scripts/push_images.sh

cloud-seed: preflight ## Seed accounts into the cloud DynamoDB accounts table
	AWS_REGION=$$($(AWS_REGION_CMD)) \
	DYNAMODB_ACCOUNTS_TABLE=$$($(ACCOUNTS_TABLE_CMD)) \
	go run ./scripts/seed_accounts.go

cloud-smoke: preflight ## Post a single transfer against the ALB and confirm completion
	./scripts/smoke_test.sh "$$($(ALB_URL_CMD))"

cloud-test-load: preflight ## Locust mixed workload against the ALB
	@URL=$$($(ALB_URL_CMD)); \
	RL=$$(pwd)/scripts/run_locust.sh; \
	cd load-tests && "$$RL" -f locustfile.py --host="$$URL" \
		--users 50 --spawn-rate 5 --run-time 5m --headless \
		--csv=results --html=report.html

cloud-test-load-hot: preflight ## Locust hot-account workload (Exp 1) against the ALB
	@URL=$$($(ALB_URL_CMD)); \
	RL=$$(pwd)/scripts/run_locust.sh; \
	cd load-tests && "$$RL" -f locustfile.py HotAccountUser --host="$$URL" \
		--users 50 --spawn-rate 5 --run-time 5m --headless \
		--csv=results-hot --html=report-hot.html

cloud-verify: preflight ## Verify balances on the cloud accounts table
	AWS_REGION=$$($(AWS_REGION_CMD)) \
	DYNAMODB_ACCOUNTS_TABLE=$$($(ACCOUNTS_TABLE_CMD)) \
	$(VERIFY_PYTHON) load-tests/verify_balances.py

cloud-compare-locking: preflight ## Exp 3: sweep locking_mode against the cloud stack
	./scripts/compare_locking_modes.sh

cloud-crash-test: preflight ## Exp 2: kill a live worker task and verify recovery
	./scripts/crash_recovery_test.sh

scale-experiment: preflight ## Exp 4: sweep api/worker replica counts
	./scripts/scale_experiment.sh

charts: ## Render PNG charts for an experiments directory (DIR=load-tests/experiments/<stamp>)
	@if [ -z "$$DIR" ]; then echo "usage: make charts DIR=load-tests/experiments/<stamp>"; exit 1; fi; \
	python3 scripts/render_charts.py "$$DIR"

worker-metrics-url: preflight ## Print the CloudWatch Logs URL that has the periodic METRICS snapshots
	@REGION=$$($(AWS_REGION_CMD)); \
	GROUP=$$(terraform -chdir=$(TF_CHDIR) output -raw worker_log_group); \
	printf 'https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logsV2:log-groups/log-group/%s\n' \
		"$$REGION" "$$REGION" "$$(printf '%s' "$$GROUP" | sed 's|/|$$252F|g')"

clean-experiments: ## Delete local experiment artifacts under load-tests/experiments
	rm -rf load-tests/experiments
