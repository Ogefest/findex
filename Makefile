MODULE := github.com/ogefest/findex
BIN_DIR := bin
GO := go

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%d %H:%M:%S")

# Build flags with version info
LDFLAGS := -s -w \
	-X '$(MODULE)/version.Version=$(VERSION)' \
	-X '$(MODULE)/version.Commit=$(COMMIT)' \
	-X '$(MODULE)/version.BuildDate=$(BUILD_DATE)'

GOFLAGS := -ldflags="$(LDFLAGS)"

.PHONY: all clean build version

all: build

build: $(BIN_DIR)/findex $(BIN_DIR)/findex-webserver

$(BIN_DIR)/findex:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/findex $(MODULE)/cmd/findex

$(BIN_DIR)/findex-webserver:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/findex-webserver $(MODULE)/cmd/webserver

clean:
	rm -rf $(BIN_DIR)

version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
