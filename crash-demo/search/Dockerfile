FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o search .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/search .
EXPOSE 8080
CMD ["./search"]