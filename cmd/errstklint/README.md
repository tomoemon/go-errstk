# errstklint

A linter that ensures all functions returning errors include `defer errstk.Wrap(&err)`.

## Installation

```bash
go install github.com/tomoemon/go-errstk/cmd/errstklint@latest
```

## Usage

```bash
# Check all packages
errstklint ./...

# Exclude generated files
errstklint -exclude="generated/*.go,**/mock_*.go" ./...

# With go vet
go vet -vettool=$(which errstklint) ./...
```

## Options

- `-exclude`: Comma-separated glob patterns to exclude files
  - Example: `generated/*.go`, `**/mock_*.go`, `**/*_test.go`

## Documentation

For detailed documentation including golangci-lint plugin integration, see:
- [Full documentation](../../errstklint/README.md)
- [Main project](../../README.md)
