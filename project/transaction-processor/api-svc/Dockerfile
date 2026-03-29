FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /api-svc ./api-svc

FROM alpine:latest
WORKDIR /app
COPY --from=builder /api-svc .
EXPOSE 8080
CMD ["./api-svc"]
