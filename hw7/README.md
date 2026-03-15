# Homework 7 Parts II and III

This repo scaffolds the Part II and Part III deliverables from `Instructions.md`:

- Go order receiver with `POST /orders/sync`, `POST /orders/async`, and `GET /health`
- SQS-backed order processor for Part II
- Lambda handler subscribed to SNS for Part III
- Terraform for VPC, ALB, ECS, SNS, SQS, and Lambda
- Locust load test for both sync and async endpoints

## Repo Layout

- `cmd/order-receiver`: HTTP API
- `cmd/order-processor`: SQS worker service
- `cmd/order-lambda`: SNS-triggered Lambda handler
- `internal/orders`: shared order models and payment-delay simulator
- `terraform`: AWS infrastructure
- `locust/locustfile.py`: load test
- `report/part2_part3_template.md`: report starting point

## Local Build

```bash
make fmt
make test
make build
```

Install the Locust dependency in a virtual environment before load testing:

```bash
python3 -m venv .venv
source .venv/bin/activate
python3 -m pip install -r requirements.txt
```

Run the receiver locally:

```bash
LISTEN_ADDR=:8080 PAYMENT_DELAY=3s SYNC_PAYMENT_WORKERS=1 go run ./cmd/order-receiver
```

Run the processor locally once you have an SQS queue:

```bash
ORDER_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/order-processing-queue \
PROCESSOR_WORKERS=1 \
PAYMENT_DELAY=3s \
go run ./cmd/order-processor
```

Build the Lambda deployment artifact:

```bash
make build-lambda
```

## Docker Images

Build the ECS images:

```bash
make docker-receiver
make docker-processor
```

Push those images to ECR, then pass the image URIs into Terraform variables:

```bash
terraform -chdir=terraform apply \
  -var receiver_image_uri=ACCOUNT.dkr.ecr.REGION.amazonaws.com/hw7-receiver:latest \
  -var processor_image_uri=ACCOUNT.dkr.ecr.REGION.amazonaws.com/hw7-processor:latest
```

## Terraform Notes

The Terraform stack creates:

- VPC `10.0.0.0/16`
- Public subnets `10.0.1.0/24`, `10.0.2.0/24`
- Private subnets `10.0.10.0/24`, `10.0.11.0/24`
- Internet gateway and NAT gateway
- Public ALB
- ECS/Fargate receiver and processor services
- SNS topic `order-processing-events`
- SQS queue `order-processing-queue`
- SNS subscription from topic to queue
- SNS subscription from topic to Lambda

Terraform expects:

- `receiver_image_uri`
- `processor_image_uri`
- `dist/order-lambda.zip` built by `make build-lambda`

## Important Safety Note

Do not deploy the Lambda subscriber during Part II load testing. SNS fans out to every subscription, so if Lambda is attached while you run Locust against `POST /orders/async`, every async request will invoke both SQS and Lambda.

This repo is set up to make Lambda opt-in:

- `terraform/part2.tfvars`: keeps Lambda disabled for Part II and worker-scaling tests
- `terraform/part3.tfvars`: enables Lambda and sets the ECS processor desired count to `0` for Part III manual testing

Use one of these exact apply commands:

```bash
terraform -chdir=terraform apply \
  -var-file=part2.tfvars \
  -var receiver_image_uri=ACCOUNT.dkr.ecr.REGION.amazonaws.com/hw7-receiver:latest \
  -var processor_image_uri=ACCOUNT.dkr.ecr.REGION.amazonaws.com/hw7-processor:latest
```

```bash
terraform -chdir=terraform apply \
  -var-file=part3.tfvars \
  -var receiver_image_uri=ACCOUNT.dkr.ecr.REGION.amazonaws.com/hw7-receiver:latest \
  -var processor_image_uri=ACCOUNT.dkr.ecr.REGION.amazonaws.com/hw7-processor:latest
```

## Load Testing

Sync test example:

```bash
ORDER_ENDPOINT=/orders/sync locust -f locust/locustfile.py --headless -u 5 -r 1 -t 30s --host http://YOUR-ALB
```

Async test example:

```bash
ORDER_ENDPOINT=/orders/async locust -f locust/locustfile.py --headless -u 20 -r 10 -t 60s --host http://YOUR-ALB
```

The assignment text mixes `60 orders/second` with a later `20 concurrent users / 20 orders/second` test. Keep your report consistent about which target load you actually used.
