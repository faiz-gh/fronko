# Build Stage
FROM golang:1.27-alpine AS builder
WORKDIR /app
# Install golang-migrate (pinned so builds are reproducible)
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/fronko ./cmd/fronko

# Run Stage
FROM alpine:3
RUN apk --no-cache add ca-certificates && adduser -D -H -u 10001 app
WORKDIR /app

COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/bin/fronko .
COPY --from=builder /app/migrations /app/migrations
COPY --from=builder --chmod=755 /app/entrypoint.sh /app/entrypoint.sh

USER app
EXPOSE 8080
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./fronko"]
