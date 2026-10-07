.PHONY: build run test test-integration lint seed clean

build:
	go build -o bin/fronko ./cmd/fronko

run: build
	./bin/fronko

test:
	go test -v -race ./...

# Needs a disposable database with the migrations applied: these tests
# TRUNCATE every table.
# e.g. TEST_DATABASE_URL=postgres://fronko:...@localhost:55000/fronko_test?sslmode=disable
test-integration:
	go test -v -race -count=1 -tags=integration ./internal/integrationtest/...

# golangci-lint v2: https://golangci-lint.run/welcome/install/
lint:
	golangci-lint run ./...

# Fills a development database (DATABASE_URL) with demo accounts and data.
# Add ARGS=--reset to wipe it first.
seed:
	FRONKO_ENV=development go run ./cmd/fronko seed $(ARGS)

clean:
	rm -rf bin/
