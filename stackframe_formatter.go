package errstk

import "fmt"

// stackFrameFormatter is a function type that formats a stack frame into a string.
type stackFrameFormatter func(frame *StackFrame) string

// defaultStackFrameFormatter returns the stack frame formatted in the same way as Go does
// in runtime/debug.Stack().
func defaultStackFrameFormatter(frame *StackFrame) string {
	// Format: FunctionName()
	//     file/path.go:123 +0xhex
	return fmt.Sprintf("%s()\n\t%s:%d +0x%x\n", frame.Name, frame.File, frame.LineNumber, frame.ProgramCounter)
}
