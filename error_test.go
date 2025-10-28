package errstk

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// customError is a test error type that implements fmt.Formatter
type customError struct {
	msg  string
	code int
}

func (e *customError) Error() string {
	return e.msg
}

func (e *customError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "%s (code: %d)", e.msg, e.code)
		} else {
			_, _ = io.WriteString(s, e.msg)
		}
	case 's':
		_, _ = io.WriteString(s, e.msg)
	}
}

func TestWith(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		result := With(nil)
		if result != nil {
			t.Errorf("With(nil) = %v, want nil", result)
		}
	})

	t.Run("adds stack trace to error", func(t *testing.T) {
		originalErr := errors.New("test error")
		wrappedErr := With(originalErr)

		if wrappedErr == nil {
			t.Fatal("With should not return nil for non-nil error")
		}

		// Check if it's a withStack type
		var stackErr *withStack
		if !errors.As(wrappedErr, &stackErr) {
			t.Error("With should return *withStack type")
		}

		// Check if the original error is preserved
		if !errors.Is(wrappedErr, originalErr) {
			t.Error("With should preserve original error")
		}

		// Check if stack trace is present in formatted output
		formatted := fmt.Sprintf("%+v", wrappedErr)
		if !strings.Contains(formatted, "test error") {
			t.Error("Formatted output should contain original error message")
		}
		if !strings.Contains(formatted, "error_test.go") {
			t.Error("Formatted output should contain stack trace")
		}
	})

	t.Run("does not double wrap error with stack", func(t *testing.T) {
		originalErr := errors.New("test error")
		wrappedOnce := With(originalErr)
		wrappedTwice := With(wrappedOnce)

		// Should return the same error, not wrap again
		if wrappedOnce != wrappedTwice {
			t.Error("With should not double wrap an error that already has a stack")
		}
	})

	t.Run("unwrap returns original error", func(t *testing.T) {
		originalErr := errors.New("original error")
		wrappedErr := With(originalErr)

		unwrapped := errors.Unwrap(wrappedErr)
		if unwrapped != originalErr {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
		}
	})

	t.Run("format with %%v shows error message", func(t *testing.T) {
		originalErr := errors.New("test error")
		wrappedErr := With(originalErr)

		formatted := fmt.Sprintf("%v", wrappedErr)
		if formatted != "test error" {
			t.Errorf("Format(%%v) = %q, want %q", formatted, "test error")
		}
	})

	t.Run("format with %%s shows error message", func(t *testing.T) {
		originalErr := errors.New("test error")
		wrappedErr := With(originalErr)

		formatted := fmt.Sprintf("%s", wrappedErr)
		if formatted != "test error" {
			t.Errorf("Format(%%s) = %q, want %q", formatted, "test error")
		}
	})

	t.Run("format with %%q shows quoted error message", func(t *testing.T) {
		originalErr := errors.New("test error")
		wrappedErr := With(originalErr)

		formatted := fmt.Sprintf("%q", wrappedErr)
		if formatted != `"test error"` {
			t.Errorf("Format(%%q) = %q, want %q", formatted, `"test error"`)
		}
	})

	t.Run("format with %%+v shows error with stack trace", func(t *testing.T) {
		originalErr := errors.New("test error")
		wrappedErr := With(originalErr)

		formatted := fmt.Sprintf("%+v", wrappedErr)

		// Check that it contains the error message
		if !strings.Contains(formatted, "test error") {
			t.Error("Format with +v flag should contain error message")
		}

		// Check that it contains file and line information
		if !strings.Contains(formatted, "error_test.go") {
			t.Error("Format with +v flag should contain stack trace with file information")
		}

		// Check that it contains the test function name
		if !strings.Contains(formatted, "TestWith") {
			t.Error("Format with +v flag should contain function name in stack trace")
		}
	})

	t.Run("preserves error chain", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
		stackErr := With(wrappedErr)

		// Should be able to find both errors in the chain
		if !errors.Is(stackErr, baseErr) {
			t.Error("Should preserve error chain to base error")
		}
		if !errors.Is(stackErr, wrappedErr) {
			t.Error("Should preserve error chain to wrapped error")
		}
	})

	t.Run("does not double wrap after wrapping with fmt.Errorf", func(t *testing.T) {
		// With -> fmt.Errorf wrap -> With again
		originalErr := errors.New("original error")
		firstStack := With(originalErr)
		wrappedErr := fmt.Errorf("context: %w", firstStack)
		secondStack := With(wrappedErr)

		// Check if secondStack is the same as wrappedErr (not wrapped again)
		if secondStack != wrappedErr {
			t.Error("With should not wrap again when error chain already contains *withStack")
		}

		// Verify the error chain is preserved
		if !errors.Is(secondStack, originalErr) {
			t.Error("Should preserve error chain to original error")
		}

		// Verify we can still find the withStack type in the chain
		var stackErr *withStack
		if !errors.As(secondStack, &stackErr) {
			t.Error("Should be able to find *withStack in error chain")
		}

		// Verify that the found withStack has the original error
		if stackErr.error != originalErr {
			t.Error("The withStack in the chain should contain the original error")
		}

		// Verify basic error message
		if secondStack.Error() != "context: original error" {
			t.Errorf("Error() = %q, want %q", secondStack.Error(), "context: original error")
		}
	})

	t.Run("respects underlying error's Format method", func(t *testing.T) {
		customErr := &customError{msg: "custom error", code: 500}
		stackErr := With(customErr)

		// Format with %+v should show both custom formatting and stack trace
		formatted := fmt.Sprintf("%+v", stackErr)

		// Should contain custom error's formatted output
		if !strings.Contains(formatted, "custom error") {
			t.Errorf("Should respect custom error's Format method, got: %s", formatted)
		}

		// Should also contain stack trace
		if !strings.Contains(formatted, "error_test.go") {
			t.Error("Should also append stack trace")
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		testFunc := func() (err error) {
			defer Wrap(&err)
			return nil
		}
		result := testFunc()
		if result != nil {
			t.Errorf("Wrap(nil) = %v, want nil", result)
		}
	})

	t.Run("wraps error with stack trace", func(t *testing.T) {
		funcThatReturnsError := func() (err error) {
			defer Wrap(&err)
			return errors.New("wrapped error")
		}

		err := funcThatReturnsError()
		if err == nil {
			t.Fatal("expected non-nil error")
		}

		// Check if it's a withStack type
		var stackErr *withStack
		if !errors.As(err, &stackErr) {
			t.Error("Wrap should return *withStack type")
		}

		// Check if stack trace is present
		formatted := fmt.Sprintf("%+v", err)
		if !strings.Contains(formatted, "wrapped error") {
			t.Error("Formatted output should contain error message")
		}
		if !strings.Contains(formatted, "error_test.go") {
			t.Error("Formatted output should contain stack trace")
		}
	})

	t.Run("captures correct line number in defer", func(t *testing.T) {
		funcWithError := func() (err error) {
			defer Wrap(&err)
			err = errors.New("test error")
			return err // This line should be captured in stack trace
		}

		err := funcWithError()
		formatted := fmt.Sprintf("%+v", err)

		// The stack trace should point to the return statement
		// We can verify this by checking that the function name appears
		if !strings.Contains(formatted, "TestWrap") {
			t.Error("Stack trace should contain test function name")
		}
		// Should contain file and line info
		if !strings.Contains(formatted, "error_test.go") {
			t.Error("Stack trace should contain file information")
		}
	})

	t.Run("does not double wrap", func(t *testing.T) {
		funcWithDoubleWrap := func() (err error) {
			defer Wrap(&err)
			err = With(errors.New("original error"))
			return err
		}

		err := funcWithDoubleWrap()

		// Should not double wrap
		var stackErr *withStack
		if !errors.As(err, &stackErr) {
			t.Error("Should contain *withStack")
		}

		// The underlying error should be the original error, not another withStack
		if _, ok := stackErr.error.(*withStack); ok {
			t.Error("Should not double wrap with *withStack")
		}
	})

	t.Run("preserves error chain", func(t *testing.T) {
		baseErr := errors.New("base error")
		funcWithChain := func() (err error) {
			defer Wrap(&err)
			err = fmt.Errorf("wrapped: %w", baseErr)
			return err
		}

		err := funcWithChain()

		// Should preserve error chain
		if !errors.Is(err, baseErr) {
			t.Error("Should preserve error chain to base error")
		}

		formatted := fmt.Sprintf("%v", err)
		if !strings.Contains(formatted, "wrapped: base error") {
			t.Errorf("Should preserve wrapped message, got: %s", formatted)
		}
	})

	t.Run("preserves original error in error chain", func(t *testing.T) {
		originalErr := errors.New("original error")
		funcWithWrap := func() (err error) {
			defer Wrap(&err)
			return originalErr
		}

		err := funcWithWrap()

		// Should be able to find original error
		if !errors.Is(err, originalErr) {
			t.Error("Should preserve original error in chain")
		}

		// Unwrap should return original error
		unwrapped := errors.Unwrap(err)
		if unwrapped != originalErr {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
		}
	})

	t.Run("multiple return paths capture correct location", func(t *testing.T) {
		funcWithMultipleReturns := func(shouldError bool) (err error) {
			defer Wrap(&err)
			if shouldError {
				return errors.New("early return error")
			}
			return errors.New("late return error")
		}

		// Both error paths should have stack traces
		err1 := funcWithMultipleReturns(true)
		formatted1 := fmt.Sprintf("%+v", err1)
		if !strings.Contains(formatted1, "early return error") {
			t.Error("Should contain early return error message")
		}
		if !strings.Contains(formatted1, "error_test.go") {
			t.Error("Should contain stack trace for early return")
		}

		err2 := funcWithMultipleReturns(false)
		formatted2 := fmt.Sprintf("%+v", err2)
		if !strings.Contains(formatted2, "late return error") {
			t.Error("Should contain late return error message")
		}
		if !strings.Contains(formatted2, "error_test.go") {
			t.Error("Should contain stack trace for late return")
		}
	})
}

func TestWrapWithVariableRedeclaration(t *testing.T) {
	t.Run("short variable declaration reuses named return value", func(t *testing.T) {
		// This is the common pattern - short declaration reuses the named return value
		funcWithShortDecl := func() (err error) {
			defer Wrap(&err)

			// This is NOT redeclaration - it's assignment to the named return value
			_, err = func() (string, error) {
				return "", errors.New("expected error")
			}()
			if err != nil {
				return err
			}
			return nil
		}

		err := funcWithShortDecl()
		if err == nil {
			t.Fatal("expected non-nil error")
		}

		// Should have stack trace
		var stackErr *withStack
		if !errors.As(err, &stackErr) {
			t.Error("Should have stack trace")
		}
	})

	t.Run("actual variable shadowing breaks Wrap", func(t *testing.T) {
		// This demonstrates what happens with actual shadowing (anti-pattern)
		funcWithShadowing := func() (err error) {
			defer Wrap(&err)

			// Create a new scoped err variable (shadowing)
			{
				err := errors.New("shadowed error")
				if err != nil {
					// This return assigns to the OUTER err, not the shadowed one
					return err
				}
			}

			return nil
		}

		err := funcWithShadowing()
		if err == nil {
			t.Fatal("expected non-nil error")
		}

		// Should have stack trace because we returned the error properly
		var stackErr *withStack
		if !errors.As(err, &stackErr) {
			t.Error("Should have stack trace")
		}
	})

	t.Run("shadowing without return does not trigger Wrap", func(t *testing.T) {
		// This demonstrates when Wrap doesn't capture the error
		funcWithUnreturnedShadow := func() (err error) {
			defer Wrap(&err)

			// Create a new scoped err variable
			{
				err := errors.New("this error is lost")
				_ = err // Use it but don't return
			}

			// The outer err is still nil
			return nil
		}

		err := funcWithUnreturnedShadow()
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("common pattern with multiple short declarations", func(t *testing.T) {
		// This is the pattern used in the selected code
		funcLikeGetHeader := func() (result string, err error) {
			defer Wrap(&err)

			// First short declaration
			value1, err := func() (string, error) {
				return "", errors.New("first error")
			}()
			if err != nil {
				return "", err
			}

			// Second short declaration - still the same err
			value2, err := func() (string, error) {
				return "", errors.New("second error")
			}()
			if err != nil {
				return "", err
			}

			return value1 + value2, nil
		}

		_, err := funcLikeGetHeader()
		if err == nil {
			t.Fatal("expected non-nil error")
		}

		// Should have stack trace
		var stackErr *withStack
		if !errors.As(err, &stackErr) {
			t.Error("Should have stack trace from Wrap")
		}

		if err.Error() != "first error" {
			t.Errorf("expected 'first error', got %v", err)
		}
	})

	t.Run("ErrorStack extracts trace from deeply nested fmt.Errorf wrapping", func(t *testing.T) {
		layer1 := func() error {
			// Original error with stack trace
			return With(errors.New("database connection failed"))
		}

		layer2 := func() error {
			err := layer1()
			if err != nil {
				// Wrap with additional context using fmt.Errorf
				return fmt.Errorf("failed to initialize: %w", err)
			}
			return nil
		}

		layer3 := func() error {
			err := layer2()
			if err != nil {
				// Wrap again with more context
				return fmt.Errorf("service startup failed: %w", err)
			}
			return nil
		}

		err := layer3()

		// Error message includes all wrapping context
		errorMsg := fmt.Sprintf("%+v", err)
		if !strings.Contains(errorMsg, "service startup failed") {
			t.Error("Should contain outermost context")
		}
		if !strings.Contains(errorMsg, "failed to initialize") {
			t.Error("Should contain middle context")
		}
		if !strings.Contains(errorMsg, "database connection failed") {
			t.Error("Should contain original error message")
		}

		// ErrorStack can still find the stack trace deep in the error chain
		stackTrace := ErrorStack(err)
		if stackTrace == "" {
			t.Error("ErrorStack should extract stack trace from deeply nested error")
		}

		// Should contain the original error information
		if !strings.Contains(stackTrace, "database connection failed") {
			t.Error("Stack trace should contain original error message")
		}

		// Should contain stack frame information
		if !strings.Contains(stackTrace, "error_test.go") {
			t.Error("Stack trace should contain file information")
		}
	})

	t.Run("ErrorStack works with errors.Join", func(t *testing.T) {
		// Simulate a function that processes a file and may have cleanup errors
		processFile := func() (err error) {
			var closeErr error
			defer func() {
				// Simulate cleanup error (e.g., file.Close())
				closeErr = errors.New("failed to close file")
				if err != nil && closeErr != nil {
					err = errors.Join(err, closeErr)
				} else if closeErr != nil {
					err = closeErr
				}
			}()

			// Main operation fails with stack trace
			return With(errors.New("failed to read file"))
		}

		err := processFile()

		// Error message should contain both errors
		errorMsg := err.Error()
		if !strings.Contains(errorMsg, "failed to read file") {
			t.Error("Should contain main error message")
		}
		if !strings.Contains(errorMsg, "failed to close file") {
			t.Error("Should contain cleanup error message")
		}

		// ErrorStack should still extract the stack trace from the joined errors
		stackTrace := ErrorStack(err)
		if stackTrace == "" {
			t.Error("ErrorStack should extract stack trace from errors.Join")
		}

		// Should contain the original error information
		if !strings.Contains(stackTrace, "failed to read file") {
			t.Error("Stack trace should contain main error message")
		}

		// Should contain stack frame information
		if !strings.Contains(stackTrace, "error_test.go") {
			t.Error("Stack trace should contain file information")
		}
	})

	t.Run("ErrorStack with nested errors.Join and fmt.Errorf", func(t *testing.T) {
		// Layer 1: Original error with stack trace
		layer1 := func() error {
			return With(errors.New("database query failed"))
		}

		// Layer 2: Multiple operations that may fail
		layer2 := func() (err error) {
			defer func() {
				cleanupErr := With(errors.New("transaction rollback failed"))
				err = errors.Join(err, cleanupErr)
			}()

			// Main operation
			err = layer1()
			if err != nil {
				return err
			}
			return nil
		}

		// Layer 3: Wrap with additional context
		layer3 := func() error {
			err := layer2()
			if err != nil {
				return fmt.Errorf("service operation failed: %w", err)
			}
			return nil
		}

		err := layer3()

		// Error message includes all contexts
		errorMsg := fmt.Sprintf("%v", err)
		if !strings.Contains(errorMsg, "service operation failed") {
			t.Error("Should contain outermost context")
		}
		if !strings.Contains(errorMsg, "database query failed") {
			t.Error("Should contain original error")
		}
		if !strings.Contains(errorMsg, "transaction rollback failed") {
			t.Error("Should contain cleanup error")
		}

		// ErrorStack should find the stack trace through errors.Join and fmt.Errorf
		stackTrace := ErrorStack(err)
		if stackTrace == "" {
			t.Error("ErrorStack should extract stack trace from complex error chain")
		}

		// Should contain the original error with stack
		if !strings.Contains(stackTrace, "database query failed") {
			t.Error("Stack trace should contain original error message")
		}

		// Should contain stack frame information
		if !strings.Contains(stackTrace, "error_test.go") {
			t.Error("Stack trace should contain file information")
		}

		// Sample stacktrace for joinedError
		//t.Log("-----")
		//t.Log(stackTrace)
		//t.Log("-----")
		/*
					service operation failed: database query failed
			        transaction rollback failed
					database query failed
					error_test.go:586 (0x10435aca5)
						TestWrapWithVariableRedeclaration.func7.1: return With(errors.New("database query failed"))
					error_test.go:597 (0x10435acbd)
						TestWrapWithVariableRedeclaration.func7.2: err = layer1()
					error_test.go:607 (0x10435a95c)
						TestWrapWithVariableRedeclaration.func7.3: if err != nil {
					error_test.go:613 (0x10435a98d)
						TestWrapWithVariableRedeclaration.func7: err := layer3()
					src/testing/testing.go:1934 (0x10431cc08)
						tRunner: fn(t)
					src/runtime/asm_arm64.s:1268 (0x1042c0ed4)
						goexit: MOVD	R0, R0	// NOP

				   transaction rollback failed
				   error_test.go:592 (0x10435ad75)
						TestWrapWithVariableRedeclaration.func7.2.1: cleanupErr := With(errors.New("transaction rollback failed"))
				   error_test.go:599 (0x10435acd4)
						TestWrapWithVariableRedeclaration.func7.2: return err
				   error_test.go:607 (0x10435a95c)
						TestWrapWithVariableRedeclaration.func7.3: if err != nil {
				   error_test.go:613 (0x10435a98d)
						TestWrapWithVariableRedeclaration.func7: err := layer3()
				   src/testing/testing.go:1934 (0x10431cc08)
						tRunner: fn(t)
				   src/runtime/asm_arm64.s:1268 (0x1042c0ed4)
						goexit: MOVD	R0, R0	// NOP
		*/
	})
}

func TestWalkStack(t *testing.T) {
	t.Run("nil error does nothing", func(t *testing.T) {
		called := false
		WalkStack(nil, func(err error, frames []StackFrame) {
			called = true
		})
		if called {
			t.Error("WalkStack should not call callback for nil error")
		}
	})

	t.Run("single error with stack trace", func(t *testing.T) {
		originalErr := With(errors.New("test error"))

		callCount := 0
		var capturedErr error
		var capturedFrames []StackFrame

		WalkStack(originalErr, func(err error, frames []StackFrame) {
			callCount++
			capturedErr = err
			capturedFrames = frames
		})

		if callCount != 1 {
			t.Errorf("WalkStack should call callback once, got %d calls", callCount)
		}
		if capturedErr == nil {
			t.Error("Callback should receive non-nil error")
		}
		if len(capturedFrames) == 0 {
			t.Error("Callback should receive non-empty stack frames")
		}
		if !strings.Contains(capturedErr.Error(), "test error") {
			t.Errorf("Error message should contain 'test error', got: %s", capturedErr.Error())
		}
	})

	t.Run("error without stack trace is skipped", func(t *testing.T) {
		plainErr := errors.New("plain error")

		called := false
		WalkStack(plainErr, func(err error, frames []StackFrame) {
			called = true
		})

		if called {
			t.Error("WalkStack should not call callback for error without stack trace")
		}
	})

	t.Run("fmt.Errorf wrapped error", func(t *testing.T) {
		innerErr := With(errors.New("inner error"))
		wrappedErr := fmt.Errorf("outer context: %w", innerErr)

		callCount := 0
		var capturedErrs []error

		WalkStack(wrappedErr, func(err error, frames []StackFrame) {
			callCount++
			capturedErrs = append(capturedErrs, err)
		})

		if callCount != 1 {
			t.Errorf("WalkStack should call callback once for wrapped error, got %d calls", callCount)
		}
		// The callback should receive the inner error with stack trace
		if len(capturedErrs) > 0 && !strings.Contains(capturedErrs[0].Error(), "inner error") {
			t.Errorf("Should capture inner error, got: %s", capturedErrs[0].Error())
		}
	})

	t.Run("errors.Join with multiple errors", func(t *testing.T) {
		err1 := With(errors.New("error 1"))
		err2 := With(errors.New("error 2"))
		err3 := errors.New("error 3") // no stack trace
		joinedErr := errors.Join(err1, err2, err3)

		callCount := 0
		var capturedErrs []error

		WalkStack(joinedErr, func(err error, frames []StackFrame) {
			callCount++
			capturedErrs = append(capturedErrs, err)
		})

		if callCount != 2 {
			t.Errorf("WalkStack should call callback twice (err1 and err2), got %d calls", callCount)
		}
		if len(capturedErrs) != 2 {
			t.Fatalf("Should capture 2 errors, got %d", len(capturedErrs))
		}
		if !strings.Contains(capturedErrs[0].Error(), "error 1") {
			t.Errorf("First error should be 'error 1', got: %s", capturedErrs[0].Error())
		}
		if !strings.Contains(capturedErrs[1].Error(), "error 2") {
			t.Errorf("Second error should be 'error 2', got: %s", capturedErrs[1].Error())
		}
	})

	t.Run("deeply nested error chain", func(t *testing.T) {
		innerErr := With(errors.New("inner"))
		middleErr := fmt.Errorf("middle: %w", innerErr)
		outerErr := fmt.Errorf("outer: %w", middleErr)

		callCount := 0
		WalkStack(outerErr, func(err error, frames []StackFrame) {
			callCount++
		})

		if callCount != 1 {
			t.Errorf("WalkStack should call callback once for nested chain, got %d calls", callCount)
		}
	})

	t.Run("custom formatting with callback", func(t *testing.T) {
		err := With(errors.New("test error"))

		var output strings.Builder
		WalkStack(err, func(err error, frames []StackFrame) {
			output.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
			for i, frame := range frames {
				if i >= 3 {
					break // Only show first 3 frames for test
				}
				output.WriteString(fmt.Sprintf("  at %s:%d\n", frame.File, frame.LineNumber))
			}
		})

		result := output.String()
		if !strings.Contains(result, "Error: test error") {
			t.Error("Custom format should contain error message")
		}
		if !strings.Contains(result, "error_test.go") {
			t.Error("Custom format should contain file name")
		}
		if !strings.Contains(result, "at ") {
			t.Error("Custom format should contain 'at' prefix")
		}
	})

	t.Run("collect stack frames programmatically", func(t *testing.T) {
		err1 := With(errors.New("error 1"))
		err2 := With(errors.New("error 2"))
		joinedErr := errors.Join(err1, err2)

		type ErrorInfo struct {
			Message string
			Frames  []StackFrame
		}
		var collected []ErrorInfo

		WalkStack(joinedErr, func(err error, frames []StackFrame) {
			collected = append(collected, ErrorInfo{
				Message: err.Error(),
				Frames:  frames,
			})
		})

		if len(collected) != 2 {
			t.Fatalf("Should collect 2 errors, got %d", len(collected))
		}
		if len(collected[0].Frames) == 0 {
			t.Error("First error should have stack frames")
		}
		if len(collected[1].Frames) == 0 {
			t.Error("Second error should have stack frames")
		}
	})

	t.Run("mixed errors.Join and fmt.Errorf", func(t *testing.T) {
		innerErr := With(errors.New("inner"))
		wrappedErr := fmt.Errorf("wrapped: %w", innerErr)

		anotherErr := With(errors.New("another"))
		joinedErr := errors.Join(wrappedErr, anotherErr)

		callCount := 0
		WalkStack(joinedErr, func(err error, frames []StackFrame) {
			callCount++
		})

		// Should find 2 errors with stack traces: innerErr and anotherErr
		if callCount != 2 {
			t.Errorf("WalkStack should call callback twice, got %d calls", callCount)
		}
	})
}
