MODULE := github.com/ogefest/findex
BINS := findex webserver
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

build: $(BINS:%=$(BIN_DIR)/%)

$(BIN_DIR)/%:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$* $(MODULE)/cmd/$*

clean:
	rm -rf $(BIN_DIR)

version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
