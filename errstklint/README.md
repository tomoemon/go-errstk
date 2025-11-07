# errstklint

Go analyzer package for checking `defer errstk.Wrap(&err)` usage.

## Overview

This package provides a `go/analysis` analyzer that can be used:
- As a standalone CLI tool (see [cmd/errstklint](../cmd/errstklint))
- As a golangci-lint plugin
- As a library in other Go analysis tools

## Usage as a Library

```go
import "github.com/tomoemon/go-errstk/errstklint"

// Use the analyzer in your tool
analyzer := errstklint.Analyzer
```

## Usage as golangci-lint Plugin

Create `.custom-gcl.yml`:

```yaml
version: v1.62.2
plugins:
  - module: 'github.com/tomoemon/go-errstk/errstklint'
    import: 'github.com/tomoemon/go-errstk/errstklint'
    version: v1.0.0
```

Optionally configure in `.golangci.yml`:

```yaml
linters:
  settings:
    errstklint:
      exclude:
        - "generated/*.go"
        - "**/mock_*.go"
        - "**/*_test.go"
```

Build and run:

```bash
golangci-lint custom
./.golangci-lint-custom run
```

## API

### `Analyzer`

```go
var Analyzer = &analysis.Analyzer{
    Name:     "errstklint",
    Doc:      "checks that functions returning errors have defer errstk.Wrap(&err)",
    Run:      run,
    Requires: []*analysis.Analyzer{inspect.Analyzer},
}
```

### `New(conf any) ([]*analysis.Analyzer, error)`

Factory function for golangci-lint plugin system.

## For more information

See the [main README](../cmd/errstklint/README.md) for detailed usage examples.
