BINARY  := getfgpat
BUILD   := build
PKG     := ./src

# Release targets: Linux x64 and macOS AArch64 (Apple Silicon).
PLATFORMS := linux/amd64 darwin/arm64

GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

.PHONY: all build build-all test vet fmt clean install

all: build

## build: build the binary for the host platform into ./$(BUILD)/$(BINARY)
build:
	@mkdir -p $(BUILD)
	go build -o $(BUILD)/$(BINARY) $(PKG)

## build-all: cross-compile release binaries into ./$(BUILD)
build-all: $(PLATFORMS)

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	@mkdir -p $(BUILD)
	GOOS=$(word 1,$(subst /, ,$@)) GOARCH=$(word 2,$(subst /, ,$@)) \
		go build -trimpath -ldflags "-s -w" \
		-o $(BUILD)/$(BINARY)-$(word 1,$(subst /, ,$@))-$(word 2,$(subst /, ,$@)) $(PKG)
	@echo "built $(BUILD)/$(BINARY)-$(word 1,$(subst /, ,$@))-$(word 2,$(subst /, ,$@))"

## test: run tests
test:
	go test $(PKG)

## vet: run go vet
vet:
	go vet $(PKG)

## fmt: format the code
fmt:
	go fmt $(PKG)

## install: build and install the binary as `$(BINARY)` into $(GOBIN)
install:
	go build -o $(GOBIN)/$(BINARY) $(PKG)

## clean: remove build artifacts
clean:
	rm -rf $(BUILD)
