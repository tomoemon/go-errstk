package errstk

import "fmt"

// stackFrameFormatter is a function type that formats a stack frame into a string.
type stackFrameFormatter func(frame *StackFrame) string

// defaultStackFrameFormatter returns the stack frame formatted in the same way as Go does
// in runtime/debug.Stack().
func defaultStackFrameFormatter(frame *StackFrame) string {
	str := fmt.Sprintf("%s:%d (0x%x)\n", frame.File, frame.LineNumber, frame.ProgramCounter)
	source, err := frame.SourceLine()
	if err != nil {
		return str
	}
	return str + fmt.Sprintf("\t%s: %s\n", frame.Name, source)
}
