.PHONY: all build test lint clean

GOOS  ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
LDFLAGS = -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# On darwin, force the external linker so binaries get a valid LC_UUID
# (required by dyld on macOS 26+). The internal Go linker omits it.
ifeq ($(GOOS),darwin)
LDFLAGS += -linkmode=external
endif

all: build

build:
	mkdir -p bin
	go build -ldflags="$(LDFLAGS)" -o bin/hint ./cmd/hint
	go build -ldflags="$(LDFLAGS)" -o bin/hint-build ./cmd/hint-build

# Test binaries also need LC_UUID on macOS 26+; force external linker on darwin.
TEST_LDFLAGS =
ifeq ($(GOOS),darwin)
TEST_LDFLAGS = -ldflags=-linkmode=external
endif

test:
	go test $(TEST_LDFLAGS) ./...

test-verbose:
	go test $(TEST_LDFLAGS) -v ./...

lint:
	go vet ./...
	gofmt -l . | grep -v vendor | tee /dev/stderr | (! read)

clean:
	rm -rf bin dist

.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       internal/plugins/proto/hint_plugin.proto

.PHONY: build-test-plugin
build-test-plugin:
	cd testdata/hint-echo && go build -ldflags="$(LDFLAGS)" -o hint-echo .
