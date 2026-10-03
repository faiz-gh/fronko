.PHONY: build run test clean

build:
	go build -o bin/credensync ./cmd/credensync

run: build
	./bin/credensync

test:
	go test -v -race ./...

clean:
	rm -rf bin/
