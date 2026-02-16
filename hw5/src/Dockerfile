FROM golang:1.21-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN go build -o server ./

FROM alpine:3.19
WORKDIR /app
COPY --from=build /src/server ./server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
