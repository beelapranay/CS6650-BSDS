FROM golang:1.25 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mapper ./cmd/mapper

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /app/mapper /mapper
EXPOSE 8080
ENV PORT=8080
CMD ["/mapper"]
