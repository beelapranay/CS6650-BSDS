FROM golang:1.24-bookworm AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/album-store-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/album-store-worker ./cmd/worker

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/album-store-api /app/album-store-api
COPY --from=build /out/album-store-worker /app/album-store-worker

EXPOSE 8000

CMD ["/app/album-store-api"]
