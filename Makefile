MODULE := github.com/ogefest/findex
BINS := findex cli tui
BIN_DIR := bin
GO := go
GOFLAGS := -ldflags="-s -w"

.PHONY: all clean build

all: build

build: $(BINS:%=$(BIN_DIR)/%)

$(BIN_DIR)/%:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$* $(MODULE)/cmd/$*

clean:
	rm -rf $(BIN_DIR)
