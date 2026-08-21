.PHONY: run jwt test tidy build

run:
	go run ./cmd/console

jwt:
	go run ./cmd/jwt-cli $(ARGS)

test:
	go test ./...

tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/api-console ./cmd/console
	go build -o bin/jwt-cli ./cmd/jwt-cli
