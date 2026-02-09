FROM golang:1.25 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o reducer ./cmd/reducer

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /app/reducer /reducer
EXPOSE 8082
ENV PORT=8082
CMD ["/reducer"]
