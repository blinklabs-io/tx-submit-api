BINARY=tx-submit-api

# Determine root directory
ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Gather all .go files for use in dependencies below
GO_FILES=$(shell find $(ROOT_DIR) -name '*.go')

# Extract Go module name from go.mod
GOMODULE=$(shell grep ^module $(ROOT_DIR)/go.mod | awk '{ print $$2 }')

# Set version strings based on git tag and current ref
GO_LDFLAGS=-ldflags "-s -w -X '$(GOMODULE)/internal/version.Version=$(shell git describe --tags --exact-match 2>/dev/null)' -X '$(GOMODULE)/internal/version.CommitHash=$(shell git rev-parse --short HEAD)'"

# Alias for building program binary
build: $(BINARY)

mod-tidy:
	go mod tidy

# Build our program binary
# Depends on GO_FILES to determine when rebuild is needed
$(BINARY): mod-tidy $(GO_FILES)
	CGO_ENABLED=0 go build \
		$(GO_LDFLAGS) \
		-o $(BINARY) \
		./cmd/$(BINARY)

.PHONY: build clean image mod-tidy

clean:
	rm -f $(BINARY)

format: mod-tidy
	go fmt ./...
	gofmt -s -w $(GO_FILES)

golines:
	golines -w --ignore-generated --chain-split-dots --max-len=80 --reformat-tags .

swagger:
	swag f -g api.go -d internal/api
	swag i -g api.go -d internal/api

test: mod-tidy
	go test -v -race ./...

# Build docker image
image: build
	docker build -t $(BINARY) .
