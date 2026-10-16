FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o attendance-api ./cmd/server

FROM alpine:3.19
WORKDIR /app

COPY --from=builder /app/attendance-api /app/attendance-api

EXPOSE 8080

CMD ["/app/attendance-api"]
