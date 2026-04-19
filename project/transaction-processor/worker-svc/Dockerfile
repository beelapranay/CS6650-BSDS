FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /worker-svc ./worker-svc

FROM alpine:latest
WORKDIR /app
COPY --from=builder /worker-svc .
EXPOSE 9090
CMD ["./worker-svc"]
