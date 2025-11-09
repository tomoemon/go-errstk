package b

import "github.com/tomoemon/go-errstk"

// This file should NOT be excluded
func GoodFunc() (err error) {
	defer errstk.Wrap(&err)
	return nil
}

func BadFunc() (err error) { // want "function BadFunc returns error but missing defer errstk.Wrap\\(&err\\)"
	// Missing defer errstk.Wrap(&err)
	return nil
}
