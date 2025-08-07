.PHONY: all
all: format test build lint

.PHONY: format
format:
	go fmt ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: build
build:
	go build -o helmhound.exe .

.PHONY: bench
bench:
	go test -bench=. -benchmem ./...

.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.3.1)
	golangci-lint run
