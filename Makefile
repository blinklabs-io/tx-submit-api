BINARY=cardano-submit-api

# Determine root directory
ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Gather all .go files for use in dependencies below
GO_FILES=$(shell find $(ROOT_DIR) -name '*.go')

mod-tidy:
	go mod tidy

# Build our program binary
# Depends on GO_FILES to determine when rebuild is needed
$(BINARY): mod-tidy $(GO_FILES)
	# Needed to fetch new dependencies and add them to go.mod
	go build -o $(BINARY) ./cmd/$(BINARY)

.PHONY: build clean image mod-tidy

# Alias for building program binary
build: $(BINARY)

clean:
	rm -f $(BINARY)

# Build docker image
image: build
	docker build -t $(BINARY) .
