# go-backend/Dockerfile
# --- ビルドステージ ---
FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-prom-status-monitor

# --- 実行ステージ ---
FROM alpine:latest

RUN apk add --no-cache nginx
RUN apk add --no-cache docker-cli


WORKDIR /

COPY --from=builder /go-prom-status-monitor /go-prom-status-monitor
COPY nginx/templates /app/nginx/templates

EXPOSE 8080

CMD ["/go-prom-status-monitor"]