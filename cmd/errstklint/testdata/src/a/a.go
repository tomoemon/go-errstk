package a

import "github.com/tomoemon/go-errstk"

// Good: has defer errstk.Wrap(&err)
func GoodNamedReturn() (err error) {
	defer errstk.Wrap(&err)
	return nil
}

// Good: has defer errstk.Wrap(&err) with multiple return values
func GoodMultipleReturns() (result string, err error) {
	defer errstk.Wrap(&err)
	return "ok", nil
}

// Good: no error return, so no need for defer
func NoErrorReturn() string {
	return "ok"
}

// Good: no return value at all
func NoReturn() {
	println("hello")
}

// Bad: missing defer errstk.Wrap(&err)
func BadNamedReturn() (err error) { // want "function BadNamedReturn returns error but missing defer errstk.Wrap\\(&err\\)"
	return nil
}

// Bad: missing defer with multiple returns
func BadMultipleReturns() (result string, err error) { // want "function BadMultipleReturns returns error but missing defer errstk.Wrap\\(&err\\)"
	return "fail", nil
}

// Bad: has defer but wrong function
func BadWrongDeferFunc() (err error) { // want "function BadWrongDeferFunc returns error but missing defer errstk.Wrap\\(&err\\)"
	defer println("not errstk.Wrap")
	return nil
}

// Bad: has defer errstk.Wrap but wrong argument
func BadWrongArgument() (err error) { // want "function BadWrongArgument returns error but missing defer errstk.Wrap\\(&err\\)"
	defer errstk.Wrap(nil) // should be &err
	return nil
}

// Good: unnamed error return with conventional name
// Note: This is a limitation - we assume "err" for unnamed returns
func GoodUnnamedReturn() error {
	var err error
	defer errstk.Wrap(&err)
	return err
}

// Good: function with body that returns error from another function
func GoodWithFunctionCall() (err error) {
	defer errstk.Wrap(&err)
	return GoodNamedReturn()
}

// Interface method declarations have no body, should be skipped
type MyInterface interface {
	MethodReturnsError() error
}
