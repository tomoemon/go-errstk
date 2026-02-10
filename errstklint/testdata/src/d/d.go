package d

import "github.com/tomoemon/go-errstk"

// Case A: Named returns, missing defer only
func NamedReturn() (err error) { // want "function NamedReturn returns error but missing defer errstk.Wrap\\(&err\\)"
	return nil
}

// Case A: Named returns with multiple values
func NamedMultiReturn() (result string, err error) { // want "function NamedMultiReturn returns error but missing defer errstk.Wrap\\(&err\\)"
	return "", nil
}

// Case B: Unnamed single error return
func UnnamedSingleReturn() error { // want "function UnnamedSingleReturn returns error but missing defer errstk.Wrap\\(&err\\)"
	return nil
}

// Case C: Unnamed multiple returns
func UnnamedMultiReturn() (string, error) { // want "function UnnamedMultiReturn returns error but missing defer errstk.Wrap\\(&err\\)"
	return "", nil
}

// Case D: Unnamed multiple error returns (no auto-fix)
func UnnamedMultiError() (error, error) { // want "function UnnamedMultiError returns error but missing defer errstk.Wrap\\(&err\\) \\(auto-fix unavailable: multiple error return values\\)"
	return nil, nil
}

// Good: Should not be touched
func AlreadyCorrect() (err error) {
	defer errstk.Wrap(&err)
	return nil
}
