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

**Requirements:** golangci-lint v2.0.0 or later

### Step 1: Create `.custom-gcl.yml`

```yaml
version: v2.6.1
plugins:
  - module: 'github.com/tomoemon/go-errstk'
    import: 'github.com/tomoemon/go-errstk/errstklint'
    version: v0.2.3
```

For local development, use `path` instead of `version`:

```yaml
version: v2.6.1
plugins:
  - module: 'github.com/tomoemon/go-errstk'
    import: 'github.com/tomoemon/go-errstk/errstklint'
    path: /path/to/go-errstk
```

### Step 2: Configure `.golangci.yml`

```yaml
version: "2"

linters:
  enable:
    - errstklint
  settings:
    custom:
      errstklint:
        type: "module"
        description: "checks that functions returning errors have defer errstk.Wrap(&err)"
        settings:
          exclude:
            - "generated/*.go"
            - "**/mock_*.go"
            - "**/*_test.go"
```

### Step 3: Build and run

```bash
# Build custom golangci-lint with errstklint
golangci-lint custom

# Run the custom binary
./custom-gcl run ./...
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

## Excluding Specific Functions or Files

### Using nolint Directives (Recommended)

You can exclude specific functions using comment directives compatible with golangci-lint:

**Function-level exclusion:**
```go
//nolint:errstklint
func HelperFunction() (err error) {
    // No defer required - this function is excluded
    return nil
}

// Alternative format (staticcheck style)
//lint:ignore errstklint performance-critical code path
func FastPath() (err error) {
    return nil
}
```

**File-level exclusion:**
```go
//nolint:errstklint
package testhelpers

// All functions in this file are excluded

func Helper1() (err error) { return nil }
func Helper2() (err error) { return nil }
```

**Alternative file-level format:**
```go
//lint:file-ignore errstklint generated code
package generated
```

**Multiple linters:**
```go
//nolint:errstklint,unused,staticcheck
func TemporaryCode() (err error) {
    return nil
}
```

### Using exclusion rules in golangci-lint

When using with golangci-lint, you can also use exclusion rules in `.golangci.yml`:

```yaml
linters:
  exclusions:
    rules:
      # Disable for test files
      - path: '(.+)_test\.go'
        linters:
          - errstklint

      # Disable for specific directories
      - path: 'internal/legacy/'
        linters:
          - errstklint

      # Disable for generated code
      - path: '.*\.pb\.go$'
        linters:
          - errstklint
```

Note: Exclusion rules are handled by golangci-lint core, not by errstklint itself.

## File Pattern Exclusion

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
