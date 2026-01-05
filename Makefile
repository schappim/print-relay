.PHONY: build build-server build-client build-admin run-server run-client clean test fmt deps

# Build all binaries
build: build-server build-client build-admin

# Build the server
build-server:
	go build -o bin/printrelay-server ./cmd/server

# Build the client
build-client:
	go build -o bin/printrelay-client ./cmd/client

# Build the admin TUI
build-admin:
	go build -o bin/printrelay-admin ./cmd/admin

# Run the server (generates keys if not provided)
run-server: build-server
	./bin/printrelay-server -verbose

# Run server with specific keys
run-server-keys: build-server
	./bin/printrelay-server -verbose -api-key="$(API_KEY)" -client-key="$(CLIENT_KEY)"

# Run the client (requires CLIENT_KEY and optionally SERVER)
run-client: build-client
	./bin/printrelay-client -verbose -key="$(CLIENT_KEY)" -server="$(SERVER)"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf data/

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	# Server
	GOOS=darwin GOARCH=amd64 go build -o bin/printrelay-server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build -o bin/printrelay-server-darwin-arm64 ./cmd/server
	GOOS=linux GOARCH=amd64 go build -o bin/printrelay-server-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 go build -o bin/printrelay-server-linux-arm64 ./cmd/server
	# Client
	GOOS=darwin GOARCH=amd64 go build -o bin/printrelay-client-darwin-amd64 ./cmd/client
	GOOS=darwin GOARCH=arm64 go build -o bin/printrelay-client-darwin-arm64 ./cmd/client
	GOOS=linux GOARCH=amd64 go build -o bin/printrelay-client-linux-amd64 ./cmd/client
	GOOS=linux GOARCH=arm64 go build -o bin/printrelay-client-linux-arm64 ./cmd/client

# Development: run server and client in separate terminals
# Terminal 1: make dev-server
# Terminal 2: make dev-client CLIENT_KEY=<key from server output>
dev-server: build-server
	./bin/printrelay-server -verbose -port=8080

dev-client: build-client
	@if [ -z "$(CLIENT_KEY)" ]; then echo "Usage: make dev-client CLIENT_KEY=<key>"; exit 1; fi
	./bin/printrelay-client -verbose -key="$(CLIENT_KEY)" -server="http://localhost:8080"
