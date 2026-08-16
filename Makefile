BINARY := unfurl
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install test vet check clean demo

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

vet:
	go vet ./...

check: vet test

demo: build
	./$(BINARY) testdata/demo/api.yaml --explain

clean:
	rm -f $(BINARY)
	rm -rf dist
