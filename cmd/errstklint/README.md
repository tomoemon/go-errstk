# errstklint

A Go linter that ensures all functions returning errors have `defer errstk.Wrap(&err)` calls.

## Overview

`errstklint` is a static analysis tool built on the `go/analysis` framework. It checks that all functions which return an `error` type include a deferred call to `errstk.Wrap(&err)` at the beginning of the function body.

This ensures proper stack trace capturing when using the [github.com/tomoemon/go-errstk](https://github.com/tomoemon/go-errstk) library.

## Installation

### Standalone CLI tool

```bash
go install github.com/tomoemon/go-errstk/cmd/errstklint@latest
```

### As golangci-lint plugin

See the [golangci-lint plugin usage](#golangci-lint-plugin-usage) section below.

## Usage

### Standalone usage

Check all packages in the current directory:

```bash
errstklint ./...
```

Check a specific package:

```bash
errstklint ./pkg/mypackage
```

### Integration with go vet

```bash
go vet -vettool=$(which errstklint) ./...
```

### golangci-lint plugin usage

Using errstklint as a golangci-lint plugin allows you to run it alongside other linters efficiently, reducing overall CI time.

#### Step 1: Create `.custom-gcl.yml`

Create a `.custom-gcl.yml` file in your project root:

```yaml
version: v1.62.2  # golangci-lint version
plugins:
  - module: 'github.com/tomoemon/go-errstk/errstklint'
    import: 'github.com/tomoemon/go-errstk/errstklint'
    version: v1.0.0  # errstklint version
```

#### Step 2: Build custom golangci-lint

```bash
golangci-lint custom
```

This creates a `.golangci-lint-custom` binary with errstklint integrated.

#### Step 3: Run

```bash
./.golangci-lint-custom run
```

Or add to your `.golangci.yml`:

```yaml
linters:
  enable:
    - errstklint
```

Then run:

```bash
./.golangci-lint-custom run
```

### CI Integration

#### Standalone

Example GitHub Actions workflow:

```yaml
- name: Install errstklint
  run: go install github.com/tomoemon/go-errstk/cmd/errstklint@latest

- name: Run errstklint
  run: errstklint ./...
```

#### With golangci-lint plugin

```yaml
- name: Build custom golangci-lint
  run: golangci-lint custom

- name: Run linters (including errstklint)
  run: ./.golangci-lint-custom run
```

## Example

### Good (passes the linter)

```go
func GetUser(id string) (user *User, err error) {
    defer errstk.Wrap(&err)

    if id == "" {
        return nil, errors.New("id is required")
    }

    return fetchUser(id)
}
```

### Bad (fails the linter)

```go
func GetUser(id string) (user *User, err error) {
    // Missing: defer errstk.Wrap(&err)

    if id == "" {
        return nil, errors.New("id is required")
    }

    return fetchUser(id)
}
```

## What it checks

The analyzer reports functions that:

1. Return `error` type (or multiple values including `error`)
2. Do **not** have a `defer` statement calling `errstk.Wrap(&err)`

## Excluding files

You can exclude files from analysis using glob patterns.

### Standalone usage

Use the `-exclude` flag with comma-separated patterns:

```bash
errstklint -exclude="generated/*.go,**/mock_*.go,**/*_test.go" ./...
```

Pattern examples:
- `generated/*.go` - Excludes all .go files in `generated/` directory
- `**/mock_*.go` - Excludes all files starting with `mock_` in any directory
- `**/*_test.go` - Excludes all test files in any directory
- `some/dir/*.go` - Excludes all .go files in `some/dir/`

### golangci-lint plugin usage

Add the `exclude` setting in `.golangci.yml`:

```yaml
linters-settings:
  errstklint:
    exclude:
      - "generated/*.go"
      - "**/mock_*.go"
      - "**/*_test.go"
```

Then run:

```bash
./.golangci-lint-custom run
```

## Requirements

- Go 1.25 or later
- Functions must use named return values for error (e.g., `err error`)

## Limitations

- Currently only checks for exact package name `errstk` (package aliases are partially supported)
- Assumes the error return variable is named `err` for unnamed returns
- Does not verify if the `Wrap` call is at the correct position (beginning of function)

## Development

### Running tests

```bash
cd cmd/errstklint
go test -v
```

### Building

```bash
cd cmd/errstklint
go build
```

## License

Same as the parent project: [github.com/tomoemon/go-errstk](https://github.com/tomoemon/go-errstk)
