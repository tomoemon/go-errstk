package errstk

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// DefaultMaxStackDepth is the maximum number of stack frames to capture on any error.
// Typically this should remain at 32, which is sufficient for most use cases.
// Advanced users can set this at package initialization time if needed.
var DefaultMaxStackDepth = 32

// DefaultSkipFrames is the default number of stack frames to skip when capturing a stack trace.
// Typically this should remain 0.
// Advanced users can set this at package initialization time if needed.
var DefaultSkipFrames = 0

// DefaultStackFrameFormatter is the default function used to format stack frames.
// By default, it formats frames in the same way as runtime/debug.Stack().
// Advanced users can replace this with a custom formatter at package initialization time.
//
// Example:
//
//	func init() {
//	    errstk.DefaultStackFrameFormatter = func(frame *errstk.StackFrame) string {
//	        return fmt.Sprintf("%s:%d", frame.File, frame.LineNumber)
//	    }
//	}
//
// Note: This setting is global and affects all stack frame formatting.
// It should be set at package initialization time only to avoid race conditions.
var DefaultStackFrameFormatter stackFrameFormatter = defaultStackFrameFormatter

// Wrap wraps the error pointed to by errp with a stack trace.
// Designed for use with defer and named return values.
//
// Does nothing if *errp is nil.
// Avoids double-wrapping if the error already has a stack trace.
// When used with defer, captures the stack trace at the return point.
//
// Example:
//
//	func processData() (err error) {
//	    defer errstk.Wrap(&err)
//	    if someCondition {
//	        return errors.New("validation failed")  // Stack trace captured here
//	    }
//	    return nil
//	}
func Wrap(errp *error) {
	if *errp != nil {
		// Skip 4 frames: Wrap -> innerWithStack -> callers -> runtime.Callers
		const innerSkip = 4
		*errp = innerWithStack(*errp, DefaultSkipFrames+innerSkip)
	}
}

// With annotates err with a stack trace at the point With was called.
//
// Returns nil if err is nil.
// Avoids double-wrapping if the error already has a stack trace.
// Preserves the error chain for errors.Is and errors.As.
//
// Example:
//
//	err := doSomething()
//	if err != nil {
//	    return errstk.With(err)
//	}
func With(err error) error {
	// Skip 4 frames: With -> innerWithStack -> callers -> runtime.Callers
	const innerSkip = 4
	return innerWithStack(err, DefaultSkipFrames+innerSkip)
}

func innerWithStack(err error, skip int) error {
	if err == nil {
		return nil
	}
	var stackError *withStack
	if errors.As(err, &stackError) {
		return err
	}
	return &withStack{
		err,
		callers(skip, DefaultMaxStackDepth),
	}
}

type withStack struct {
	error
	stack []uintptr
}

func (w *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, w.ErrorStack())
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, w.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", w.Error())
	}
}

// Stack returns the callstack formatted the same way that go does
// in runtime/debug.Stack()
func (w *withStack) Stack() []byte {
	return formatStackFrames(w.StackFrames())
}

// StackFrames returns the stack frames captured when this error was wrapped.
// Each StackFrame contains information about the file, line number, and function name.
func (w *withStack) StackFrames() []StackFrame {
	return stackFramesFromPC(w.stack)
}

// Callers satisfies the bugsnag ErrorWithCallerS() interface
// so that the stack can be read out.
func (w *withStack) Callers() []uintptr {
	return w.stack
}

// ErrorStack returns a string that contains both the
// error message and the callstack.
func (w *withStack) ErrorStack() string {
	return w.Error() + "\n" + string(w.Stack())
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withStack) Unwrap() error {
	return w.error
}

// ErrorStack returns a string that contains both the error message and the callstack.
// It searches through the error chain to find all stack traces and combines them.
// This works even when the error has been wrapped multiple times with fmt.Errorf or errors.Join.
//
// If the original error wraps other errors (via fmt.Errorf("%w") or errors.Join),
// the complete error message from the original error is prepended to preserve
// the full context. This avoids losing wrapper messages while preventing duplication
// when the error itself has a stack trace.
//
// Returns an empty string if no stack trace is found in the error chain.
//
// Examples:
//   - Unwrapped error with stack: "error msg\nstack trace"
//   - fmt.Errorf wrapped: "outer: inner\n\ninner\nstack trace"
//   - errors.Join: "err1\nerr2\n\nerr1\nstack1\n\nerr2\nstack2"
func ErrorStack(originalErr error) string {
	var accum []string
	var wrapped bool

	WalkStack(originalErr, func(err error, frames []StackFrame) {
		wrapped = originalErr != err
		accum = append(accum, fmt.Sprintf("%s\n%s", err.Error(), string(formatStackFrames(frames))))
	})

	if wrapped {
		accum = append([]string{originalErr.Error() + "\n"}, accum...)
	}
	return strings.Join(accum, "\n")
}

// WalkStack walks through the error chain and calls f for each error that has a stack trace.
// It supports both single error chains (via errors.Unwrap) and multiple error chains
// (via errors.Join / Unwrap() []error interface).
//
// The callback function f is called with:
//   - err: the error that contains the stack trace
//   - frames: the stack frames captured at the point where the error was wrapped
//
// WalkStack is useful when you need custom formatting or processing of error stack traces.
// For standard formatted output, use ErrorStack() instead.
//
// Example - Custom formatting:
//
//	errstk.WalkStack(err, func(err error, frames []StackFrame) {
//	    fmt.Printf("Error: %s\n", err.Error())
//	    for _, frame := range frames {
//	        fmt.Printf("  at %s:%d in %s\n", frame.File, frame.Line, frame.Name)
//	    }
//	})
//
// Example - JSON serialization:
//
//	type ErrorTrace struct {
//	    Message string `json:"message"`
//	    Stack   []StackFrame `json:"stack"`
//	}
//	var traces []ErrorTrace
//	errstk.WalkStack(err, func(err error, frames []StackFrame) {
//	    traces = append(traces, ErrorTrace{
//	        Message: err.Error(),
//	        Stack:   frames,
//	    })
//	})
//	json.Marshal(traces)
func WalkStack(err error, f func(error, []StackFrame)) {
	if err == nil {
		return
	}
	// Check if this error has stack trace information
	if caller, ok := err.(interface{ Callers() []uintptr }); ok {
		f(err, stackFramesFromPC(caller.Callers()))
	}
	// Handle errors.Join (multiple wrapped errors)
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		errs := u.Unwrap()
		for _, e := range errs {
			WalkStack(e, f)
		}
	} else if u := errors.Unwrap(err); u != nil {
		// Handle standard error wrapping (single wrapped error)
		WalkStack(u, f)
	}
}

func stackFramesFromPC(stack []uintptr) []StackFrame {
	if stack == nil {
		return nil
	}
	frames := make([]StackFrame, len(stack))
	for i, pc := range stack {
		frames[i] = newStackFrame(pc)
	}
	return frames
}

// formatStackFrames returns the callstack formatted the same way that go does
// in runtime/debug.Stack()
func formatStackFrames(frames []StackFrame) []byte {
	buf := bytes.Buffer{}

	for _, frame := range frames {
		buf.WriteString(frame.String())
	}

	return buf.Bytes()
}
