BINARY    := dbpacklogs
BUILD_DIR := ./bin
LDFLAGS   := -s -w

.PHONY: build clean test lint

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
