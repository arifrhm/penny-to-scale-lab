.PHONY: all build run test bench bench-changelog clean

BINARY_NAME=bin/server

all: build test

build:
	@mkdir -p bin
	go build -o $(BINARY_NAME) cmd/server/main.go
	@echo "✅ Server built: $(BINARY_NAME)"

run: build
	@PORT=8080 ./$(BINARY_NAME)

test:
	go test -v -race ./internal/...

bench:
	@bash scripts/bench.sh

bench-changelog:
	@bash scripts/bench.sh --append-changelog $(VERSION) "$(TITLE)"

clean:
	rm -rf bin/
	@echo "🧹 Cleaned build artifacts"
