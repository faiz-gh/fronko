.PHONY: build run test test-integration clean

build:
	go build -o bin/fronko ./cmd/fronko

run: build
	./bin/fronko

test:
	go test -v -race ./...

# Needs a disposable database: these tests TRUNCATE every table.
# e.g. TEST_DATABASE_URL=postgres://fronko:...@localhost:55000/fronko_test?sslmode=disable
test-integration:
	go test -v -race -count=1 -tags=integration ./internal/repository/...

clean:
	rm -rf bin/
