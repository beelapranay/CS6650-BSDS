# Album Store Application

This repository now uses a Go implementation for the ChaosArena `v1-album-store` contract.

## Current Structure

- `cmd/api/main.go`: HTTP API entrypoint.
- `cmd/worker/main.go`: SQS worker entrypoint.
- `internal/albumstore/`: service logic, HTTP handlers, local SQLite/filesystem backend, and AWS DynamoDB/S3/SQS backend.
- `infra/`: Terraform for the AWS deployment footprint, still using `LabRole` and the existing ECS/Fargate + ALB design.
- `Dockerfile`: multi-stage Go build that produces the API and worker binaries.

## Run Locally

```bash
go run ./cmd/api
```

Optional environment variables:

- `ALBUM_STORE_BACKEND`: `local` or `aws`. Defaults to `local`.
- `DATA_DIR`: overrides the local data directory.
- `PUBLIC_BASE_URL`: forces the completed photo URL base.
- `PHOTO_WORKERS`: local background worker count.
- `MAX_UPLOAD_BYTES`: override upload size limit.
- `HOST`: API bind host. Defaults to `0.0.0.0`.
- `PORT`: API bind port. Defaults to `8000`.

## AWS Runtime Variables

When `ALBUM_STORE_BACKEND=aws`, the Go application expects:

- `AWS_REGION` or `AWS_DEFAULT_REGION`
- `ALBUMS_TABLE_NAME`
- `PHOTOS_TABLE_NAME`
- `PHOTO_COUNTERS_TABLE_NAME`
- `PHOTO_BUCKET_NAME`
- `PHOTO_QUEUE_URL`

Optional AWS runtime variables:

- `PHOTO_MEDIA_PREFIX`: defaults to `media`
- `S3_PUBLIC_BASE_URL`: use this when the bucket or CDN exposes direct public object URLs
- `WORKER_THREADS`: worker concurrency, defaults to `20`
- `SQS_WAIT_TIME_SECONDS`: defaults to `1`
- `SQS_VISIBILITY_TIMEOUT`: defaults to `60`

## Run Worker

```bash
go run ./cmd/worker
```

Single-message mode:

```bash
go run ./cmd/worker --once
```

## Terraform

Terraform remains in `infra/` and keeps the same deployment decisions:

- `LabRole` only
- ECS/Fargate
- public ALB
- same DynamoDB/S3/SQS resources

The only runtime-related Terraform change is that the worker task now executes the Go worker binary instead of `python3 album_store_worker.py`.

## Container Build

The same image is still used for both the API and worker services. The image default command runs the API binary, and the ECS worker task overrides the command to run `/app/album-store-worker`.

## Run Tests

```bash
go test ./...
```
