FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o inventory .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/inventory .
EXPOSE 8081
CMD ["./inventory"]