.PHONY: run build test clean install

# Run the MCP server directly
run:
	go run cmd/mcp-lmstudio/main.go

# Build the MCP server binary
build:
	go build -o mcp-lmstudio cmd/mcp-lmstudio/main.go

# Install dependencies
install:
	go mod download
	go mod tidy

# Run the test client (built-in handshake and tool calls)
test: build
	go run cmd/test-client/main.go

# Clean up binaries and logs
clean:
	rm -f mcp-lmstudio
	rm -f lmstudio_audit.log
