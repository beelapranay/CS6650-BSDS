FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY main.go ./

RUN go build -o /bin/hw8-cart-api ./main.go

FROM alpine:3.20

RUN adduser -D -H appuser

USER appuser

COPY --from=build /bin/hw8-cart-api /bin/hw8-cart-api

EXPOSE 8080

ENTRYPOINT ["/bin/hw8-cart-api"]
