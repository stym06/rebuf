# Contributing to rebuf

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/stym06/rebuf.git
   cd rebuf
   ```

2. Make sure you have Go 1.22+ installed.

## Running Tests

```bash
make test
```

To run tests with the race detector:

```bash
go test -race ./...
```

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Add godoc comments to all exported types and functions.
- Keep the project dependency-free (standard library only).

## Pull Requests

1. Create a feature branch from `main`.
2. Make your changes with clear commit messages.
3. Add or update tests for any new functionality.
4. Ensure all tests pass and the race detector is clean.
5. Open a PR with a description of your changes.
