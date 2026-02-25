.PHONY: build run dev test clean deps generate build-all build-linux deploy

# Binary name
BINARY=askdoc
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Build the binary
build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/askdoc

# Build for production (smaller binary)
build-prod:
	CGO_ENABLED=0 go build $(LDFLAGS) -ldflags "-s -w" -o bin/$(BINARY) ./cmd/askdoc

# Run the server
run: build
	./bin/$(BINARY)

# Run in development mode with hot reload (requires air)
dev:
	air

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f $(BINARY)

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate embedding models
generate:
	go generate ./...

# Build for multiple platforms (for distribution)
build-all:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64 ./cmd/askdoc
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -ldflags "-s -w" -o dist/$(BINARY)-linux-arm64 ./cmd/askdoc
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -ldflags "-s -w" -o dist/$(BINARY)-darwin-amd64 ./cmd/askdoc
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -ldflags "-s -w" -o dist/$(BINARY)-darwin-arm64 ./cmd/askdoc

# Build for Linux amd64 only
build-linux:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64 ./cmd/askdoc

# Deploy to code server (requires SSH host 'code' configured)
DEPLOY_HOST ?= code
DEPLOY_BIN  ?= /usr/local/bin/askdoc

deploy: build-linux
	scp dist/$(BINARY)-linux-amd64 $(DEPLOY_HOST):/tmp/$(BINARY)-new
	ssh $(DEPLOY_HOST) "sudo systemctl stop askdoc && sudo cp /tmp/$(BINARY)-new $(DEPLOY_BIN) && sudo systemctl start askdoc && systemctl is-active askdoc"

# Create release tarballs
release: build-all
	cd dist && tar -czvf $(BINARY)-$(VERSION)-linux-amd64.tar.gz $(BINARY)-linux-amd64
	cd dist && tar -czvf $(BINARY)-$(VERSION)-linux-arm64.tar.gz $(BINARY)-linux-arm64
	cd dist && tar -czvf $(BINARY)-$(VERSION)-darwin-amd64.tar.gz $(BINARY)-darwin-amd64
	cd dist && tar -czvf $(BINARY)-$(VERSION)-darwin-arm64.tar.gz $(BINARY)-darwin-arm64

# Docker build
docker-build:
	docker build -t askdoc:$(VERSION) -t askdoc:latest .

# Docker run
docker-run:
	docker run -p 43510:43510 -v ./data:/app/data askdoc:latest
