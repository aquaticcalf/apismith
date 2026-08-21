.PHONY: run jwt ls test tidy build

run:
	go run ./cmd/apismith ui

jwt:
	go run ./cmd/apismith jwt $(ARGS)

ls:
	go run ./cmd/apismith ls

test:
	go test ./...

tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/apismith ./cmd/apismith
