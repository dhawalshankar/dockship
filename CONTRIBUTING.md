# Contributing to Dockship

Thank you for contributing to Dockship!

## Development Setup

### Prerequisites
- Go 1.22+
- Docker or Podman
- (Optional) `hadolint` and `actionlint`

### Building
```bash
go build -o bin/dockship ./cmd/dockship
```

### Running Tests
```bash
go test -v ./...
```

### Code Guidelines
- Keep tool allowlist strictly sandboxed to the project directory.
- Never write secrets, passwords, or credentials into artifacts or logs.
- Follow idiomatic Go conventions (`gofmt`, `go vet`).
