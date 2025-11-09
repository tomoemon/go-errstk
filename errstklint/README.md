# errstklint

Go analyzer package for checking `defer errstk.Wrap(&err)` usage.

## Overview

This package provides a `go/analysis` analyzer that can be used:
- As a standalone CLI tool (see [cmd/errstklint](../cmd/errstklint))
- As a golangci-lint plugin
- As a library in other Go analysis tools

## Standalone CLI Usage

```bash
# Install
go install github.com/tomoemon/go-errstk/cmd/errstklint@latest

# Run
errstklint ./...

# Exclude files
errstklint -exclude="generated/*.go,**/mock_*.go" ./...
```

See [cmd/errstklint/README.md](../cmd/errstklint/README.md) for more CLI options.

## Usage as golangci-lint Plugin

### Step 1: Create `.custom-gcl.yml`

```yaml
version: v1.62.2
plugins:
  - module: 'github.com/tomoemon/go-errstk/errstklint'
    import: 'github.com/tomoemon/go-errstk/errstklint'
    version: v1.0.0
```

### Step 2: Configure `.golangci.yml` (optional)

```yaml
linters:
  settings:
    errstklint:
      exclude:
        - "generated/*.go"
        - "**/mock_*.go"
        - "**/*_test.go"
```

### Step 3: Build and run

```bash
golangci-lint custom
./.golangci-lint-custom run
```

## Usage as a Library

```go
import "github.com/tomoemon/go-errstk/errstklint"

// Use the analyzer in your tool
analyzer := errstklint.Analyzer
```

## What it checks

The analyzer reports functions that:
1. Return `error` type (or multiple values including `error`)
2. Do **not** have a `defer` statement calling `errstk.Wrap(&err)`

### Example

**Good (passes):**
```go
func GetUser(id string) (user *User, err error) {
    defer errstk.Wrap(&err)
    // function implementation
}
```

**Bad (fails):**
```go
func GetUser(id string) (user *User, err error) {
    // Missing: defer errstk.Wrap(&err)
    // function implementation
}
```

## File Exclusion

### CLI

```bash
errstklint -exclude="generated/*.go,**/mock_*.go,**/*_test.go" ./...
```

### golangci-lint

```yaml
linters:
  settings:
    errstklint:
      exclude:
        - "generated/*.go"
        - "**/mock_*.go"
        - "**/*_test.go"
```

### Pattern Examples

- `generated/*.go` - All .go files in `generated/` directory
- `**/mock_*.go` - All files starting with `mock_` in any directory
- `**/*_test.go` - All test files in any directory

## API Reference

### `Analyzer`

```go
var Analyzer = &analysis.Analyzer{
    Name:     "errstklint",
    Doc:      "checks that functions returning errors have defer errstk.Wrap(&err)",
    Run:      run,
    Requires: []*analysis.Analyzer{inspect.Analyzer},
}
```

The main analyzer instance that can be used directly or via `singlechecker.Main()`.

### `New(conf any) ([]*analysis.Analyzer, error)`

Factory function for golangci-lint plugin system. Automatically called by golangci-lint when loading the plugin.

### `Config`

```go
type Config struct {
    Exclude []string `json:"exclude" yaml:"exclude"`
}
```

Configuration structure for the analyzer.

## Requirements

- Go 1.25 or later
- Functions must use named return values for error (e.g., `err error`)

## Module Structure

This package is part of the `github.com/tomoemon/go-errstk` module (single module repository). The linter dependencies are included in the root `go.mod`, but they only affect projects that import this analyzer package directly.
