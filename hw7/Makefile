APP_NAME := hw7
AWS_REGION ?= us-east-1
RECEIVER_IMAGE ?= $(APP_NAME)-receiver:latest
PROCESSOR_IMAGE ?= $(APP_NAME)-processor:latest

.PHONY: fmt test build build-receiver build-processor build-lambda docker-receiver docker-processor terraform-fmt

fmt:
	go fmt ./...
	terraform -chdir=terraform fmt

test:
	go test ./...

build: build-receiver build-processor build-lambda

build-receiver:
	mkdir -p bin
	go build -o bin/order-receiver ./cmd/order-receiver

build-processor:
	mkdir -p bin
	go build -o bin/order-processor ./cmd/order-processor

build-lambda:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap ./cmd/order-lambda
	zip -j dist/order-lambda.zip bootstrap
	rm -f bootstrap

docker-receiver:
	docker build -f Dockerfile.receiver -t $(RECEIVER_IMAGE) .

docker-processor:
	docker build -f Dockerfile.processor -t $(PROCESSOR_IMAGE) .

terraform-fmt:
	terraform -chdir=terraform fmt
