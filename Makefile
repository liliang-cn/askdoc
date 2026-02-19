.PHONY: build run clean test

# Binary name
BINARY=askdoc

# Build the binary
build:
	go build -o bin/$(BINARY) ./cmd/askdoc

# Run the server
run: build
	./bin/$(BINARY) --config config/config.yaml

# Run in development mode with hot reload (requires air)
dev:
	air

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf data/

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate embedding models
generate:
	go generate ./...

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 ./cmd/askdoc
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 ./cmd/askdoc
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 ./cmd/askdoc
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe ./cmd/askdoc

# Docker build
docker-build:
	docker build -t askdoc:latest .

# Docker run
docker-run:
	docker run -p 8080:8080 -v ./data:/app/data askdoc:latest
