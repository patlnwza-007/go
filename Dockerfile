FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o order-service .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Bangkok

WORKDIR /root/
COPY --from=builder /app/order-service .
COPY --from=builder /app/.env.example .env

EXPOSE 8083

CMD ["./order-service"]
