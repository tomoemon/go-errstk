# errstk

A lightweight Go error handling package that adds stack traces to errors while maintaining compatibility with Go 1.13+ error wrapping.

## Design Philosophy

errstk is designed with the following principles:

1. **Minimal API**: Focus solely on adding stack traces, nothing more
2. **Standard library first**: Encourage use of `errors.New`, `fmt.Errorf` and `errors.Join`
3. **Compatibility**: Full support for Go 1.13+ error handling features
4. **Defer-friendly**: Accurate line number capture with defer usage
5. **Enforced correctness**: Includes a linter to ensure proper stack trace capture

## Installation

```bash
go get github.com/tomoemon/go-errstk
```

## Quick Start

### Basic Usage

```go
package main

import (
    "errors"
    "fmt"

    "github.com/tomoemon/go-errstk"
)

func main() {
    err := doSomething()
    if err != nil {
        // Print error with stack trace
        fmt.Printf("%+v\n", err)
    }
}

func doSomething() error {
    err := processFile()
    if err != nil {
        return errstk.With(err)
    }
    return nil
}

func processFile() error {
    return errors.New("file not found")
}
```

### Using with defer (Recommended)

```go
func GetUser(id string) (user *User, err error) {
    defer errstk.Wrap(&err)

    user, err = db.Query(id)
    if err != nil {
        return nil, err
    }

    err = user.Validate()
    if err != nil {
        return nil, err
    }

    return user, nil
}
```

The `defer errstk.Wrap(&err)` pattern captures the exact line where the error is returned, making debugging much easier.

## API Reference

### `With`

```go
func With(err error) error
```

Annotates an error with a stack trace at the point `With` was called.

- Returns `nil` if the input error is `nil`
- Avoids double-wrapping if the error already has a stack trace
- Preserves the error chain for `errors.Is` and `errors.As`

**Example:**

```go
err := errors.New("database error")
wrappedErr := errstk.With(err)
fmt.Printf("%+v", wrappedErr)
// Output:
// database error
// main.myFunction()
//     /path/to/file.go:42 +0x1234567
// main.main()
//     /path/to/file.go:30 +0x7654321
```

### `Wrap`

```go
func Wrap(errp *error)
```

Wraps the error pointed to by `errp` with a stack trace. Designed for use with `defer` and named return values.

- Does nothing if `*errp` is `nil`
- Captures the stack trace at the return point when used with `defer`
- Avoids double-wrapping

**Example:**

```go
func processData() (err error) {
    defer errstk.Wrap(&err)

    // Your code here
    if someCondition {
        return errors.New("validation failed")  // Stack trace points here
    }

    return nil
}
```

### `ErrorStack`

```go
func ErrorStack(err error) string
```

Returns a string containing both the error message and callstack. Searches through the error chain to find all stack traces and combines them, working even when the error has been wrapped multiple times with `fmt.Errorf` or `errors.Join`.

- Preserves complete error context from wrapped errors
- Returns empty string if no stack trace is found
- Automatically handles message duplication

**Example:**

```go
// Layer 1: Original error with stack trace
err1 := errstk.With(errors.New("database connection failed"))

// Layer 2: Wrap with fmt.Errorf
err2 := fmt.Errorf("failed to initialize: %w", err1)

// Layer 3: Wrap again
err3 := fmt.Errorf("service startup failed: %w", err2)

// Get complete stack trace with full context
stackTrace := errstk.ErrorStack(err3)
fmt.Println(stackTrace)
// Output:
// service startup failed: failed to initialize: database connection failed
//
// database connection failed
// main.layer1()
//     /path/to/main.go:10 +0x1234567
// ...
```

### `WalkStack`

```go
func WalkStack(err error, f func(error, []StackFrame))
```

Walks through the error chain and calls the callback function for each error that has a stack trace. Supports both single error chains (via `errors.Unwrap`) and multiple error chains (via `errors.Join`).

This is useful when you need custom formatting or processing of error stack traces. For standard formatted output, use `ErrorStack()` instead.

**Example - Custom formatting:**

```go
errstk.WalkStack(err, func(err error, frames []StackFrame) {
    fmt.Printf("Error: %s\n", err.Error())
    for _, frame := range frames {
        fmt.Printf("  at %s:%d in %s\n", frame.File, frame.LineNumber, frame.Name)
    }
})
```

**Example - JSON serialization:**

```go
type ErrorTrace struct {
    Message string       `json:"message"`
    Stack   []StackFrame `json:"stack"`
}

var traces []ErrorTrace
errstk.WalkStack(err, func(err error, frames []StackFrame) {
    traces = append(traces, ErrorTrace{
        Message: err.Error(),
        Stack:   frames,
    })
})
data, _ := json.Marshal(traces)
```

**Example - Collecting only first N frames:**

```go
var topFrames []StackFrame
errstk.WalkStack(err, func(_ error, frames []StackFrame) {
    if len(topFrames) == 0 && len(frames) > 5 {
        topFrames = frames[:5] // Get first 5 frames
    }
})
```

## Formatting Options

errstk supports standard Go format verbs:

- `%s`, `%v`: Error message only
- `%q`: Quoted error message
- `%+v`: Error message with full stack trace

**Example:**

```go
err := errstk.With(errors.New("something went wrong"))

fmt.Printf("%s\n", err)   // something went wrong
fmt.Printf("%v\n", err)   // something went wrong
fmt.Printf("%q\n", err)   // "something went wrong"
fmt.Printf("%+v\n", err)  // *errors.errorString something went wrong
                          // main.myFunction()
                          //     /path/to/file.go:42 +0x1234567
                          // ...
```

## Working with Error Chains

errstk preserves Go 1.13+ error chains, allowing you to use `errors.Is` and `errors.As`:

```go
var ErrNotFound = errors.New("not found")

func findUser(id string) error {
    err := db.Find(id)
    if err != nil {
        return errstk.With(ErrNotFound)
    }
    return nil
}

func main() {
    err := findUser("123")

    // errors.Is works correctly
    if errors.Is(err, ErrNotFound) {
        fmt.Println("User not found")
    }

    // Extract stack trace from wrapped errors
    stackTrace := errstk.ErrorStack(err)
    if stackTrace != "" {
        fmt.Println("Stack trace:")
        fmt.Println(stackTrace)
    }

    // Or use %+v format verb
    fmt.Printf("%+v\n", err)
}
```

### `ErrorStack` Works with `fmt.Errorf` Wrapping

`ErrorStack` can extract stack traces even when the error has been wrapped multiple times with `fmt.Errorf`:

```go
func layer1() error {
    // Original error with stack trace
    return errstk.With(errors.New("database connection failed"))
}

func layer2() error {
    err := layer1()
    if err != nil {
        // Wrap with additional context using fmt.Errorf
        return fmt.Errorf("failed to initialize: %w", err)
    }
    return nil
}

func layer3() error {
    err := layer2()
    if err != nil {
        // Wrap again with more context
        return fmt.Errorf("service startup failed: %w", err)
    }
    return nil
}

func main() {
    err := layer3()

    // Error message includes all wrapping context
    fmt.Printf("Error: %v\n", err)
    // Output: service startup failed: failed to initialize: database connection failed

    // ErrorStack can still find the stack trace deep in the error chain
    stackTrace := errstk.ErrorStack(err)
    if stackTrace != "" {
        fmt.Println("\nStack trace from original error:")
        fmt.Println(stackTrace)
        // Output shows the stack trace from layer1() where errstk.With was called
    }

    // Note: Using %+v on the fmt.Errorf-wrapped error won't show the stack trace
    // because fmt.Errorf creates a plain error type.
    // Use ErrorStack() instead to extract the stack trace from the error chain.
}
```

### `ErrorStack` Works with `errors.Join`

`ErrorStack` can extract and combine stack traces from multiple errors joined with `errors.Join`:

```go
func processFile() error {
    var errs []error

    // Read operation fails with stack trace
    if err := readFile(); err != nil {
        errs = append(errs, errstk.With(fmt.Errorf("read failed: %w", err)))
    }

    // Write operation also fails with stack trace
    if err := writeFile(); err != nil {
        errs = append(errs, errstk.With(fmt.Errorf("write failed: %w", err)))
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}

func main() {
    err := processFile()
    if err != nil {
        // Error message includes all joined errors
        fmt.Printf("Errors: %v\n", err)
        // Output:
        // read failed: file not found
        // write failed: permission denied

        // ErrorStack extracts stack traces from all errors
        stackTrace := errstk.ErrorStack(err)
        fmt.Println("\nComplete stack traces:")
        fmt.Println(stackTrace)
        // Output:
        // read failed: file not found
        // write failed: permission denied
        //
        // read failed: file not found
        // processFile
        //     /path/to/file.go:10
        // ...
        //
        // write failed: permission denied
        // processFile
        //     /path/to/file.go:15
        // ...
    }
}
```

**Cleanup error pattern with `errors.Join`:**

```go
func processResource() (err error) {
    var cleanupErr error
    defer func() {
        // Cleanup operation that may fail
        if cerr := cleanup(); cerr != nil {
            cleanupErr = errstk.With(fmt.Errorf("cleanup failed: %w", cerr))
            err = errors.Join(err, cleanupErr)
        }
    }()

    // Main operation with stack trace
    if err := doWork(); err != nil {
        return errstk.With(fmt.Errorf("work failed: %w", err))
    }

    return nil
}

func main() {
    err := processResource()
    if err != nil {
        // ErrorStack will show stack traces from both the main error
        // and the cleanup error
        fmt.Println(errstk.ErrorStack(err))
        // Output includes both stack traces with their contexts
    }
}
```

## Best Practices

### 1. Use deferred `Wrap` for Functions with Multiple Return Points

```go
func complexOperation() (result *Result, err error) {
    defer errstk.Wrap(&err)

    if err := validateInput(); err != nil {
        return nil, err  // Stack trace captured here
    }

    if err := processData(); err != nil {
        return nil, err  // Stack trace captured here
    }

    return &Result{}, nil
}
```

### 2. Use `With` for Immediate Wrapping

```go
func simpleOperation() error {
    err := doSomething()
    if err != nil {
        return errstk.With(err)
    }
    return nil
}
```

### 3. Use Standard `errors.New`

This package intentionally does not provide a `New` function to encourage using the standard library:

```go
// Good
return errstk.With(errors.New("invalid input"))

// Avoid - no such function
return errstk.New("invalid input")
```

## Configuration

### Maximum Stack Depth

You can configure the maximum stack depth globally:

```go
errstk.DefaultMaxStackDepth = 50  // Default is 32
```

### Skip Stack Frames

You can configure the number of stack frames to skip when capturing a stack trace. This is useful when you wrap `With` or `Wrap` in your own helper functions.

**Example - Custom wrapper function:**

```go
package myapp

import (
    "fmt"
	
    "github.com/tomoemon/go-errstk"
)

func init() {
    // Skip one additional frame for our custom wrapper
    errstk.DefaultSkipFrames = 1
}

// WrapWithContext Custom wrapper that adds context
func WrapWithContext(err error, ctx string) error {
    if err == nil {
        return nil
    }
    return errstk.With(fmt.Errorf("%s: %w", ctx, err))
}
```

### Stack Frame Formatter

You can customize how stack frames are formatted:

```go
func init() {
    errstk.DefaultStackFrameFormatter = func(frame *errstk.StackFrame) string {
        return fmt.Sprintf("%s:%d", frame.File, frame.LineNumber)
    }
}
```

## Linter Tool

**errstklint** is a linter that ensures all functions returning errors include `defer errstk.Wrap(&err)` for proper stack trace capture.

### Installation

```bash
go install github.com/tomoemon/go-errstk/cmd/errstklint@latest
```

### Quick Usage

```bash
# Standalone
errstklint ./...

# With exclusions for generated code
errstklint -exclude="generated/*.go,**/mock_*.go" ./...

# As golangci-lint plugin
golangci-lint custom  # See documentation for setup
./.golangci-lint-custom run
```

### Excluding Specific Functions

Use nolint directives compatible with golangci-lint:

```go
//nolint:errstklint
func HelperFunction() (err error) {
    // This function is excluded from linting
    return nil
}

// File-level exclusion
//nolint:errstklint
package testhelpers
```

See [full documentation](errstklint/README.md) for more exclusion options including `//lint:ignore` and `exclude-rules`.

### Documentation

For detailed usage and golangci-lint integration:
- [Standalone CLI documentation](cmd/errstklint/README.md)
- [golangci-lint plugin documentation](errstklint/README.md)

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## Credits

Inspired by:

- [pkg/errors](https://github.com/pkg/errors)
- [go-errors/errors](https://github.com/go-errors/errors)
- [golang/pkgsite:derrors](https://github.com/golang/pkgsite/blob/c20a88edadfbe20d624856081ccf9de2a2e6b945/internal/derrors/derrors.go)
