package c

import "github.com/tomoemon/go-errstk"

// Test nolint:errstklint directive

//nolint:errstklint
func IgnoredWithNolint() (err error) {
	// No defer required - ignored by nolint
	return nil
}

//nolint:all
func IgnoredWithNolintAll() (err error) {
	// No defer required - ignored by nolint:all
	return nil
}

//nolint:unused,errstklint
func IgnoredWithMultipleLinters() (err error) {
	// No defer required - errstklint is in the list
	return nil
}

//nolint:unused
func NotIgnored() (err error) { // want "function NotIgnored returns error but missing defer errstk.Wrap"
	// Should be reported - different linter
	return nil
}

// Test lint:ignore directive

//lint:ignore errstklint this is a test helper
func IgnoredWithLintIgnore() (err error) {
	// No defer required - ignored by lint:ignore
	return nil
}

// Test that correct usage still passes

func CorrectUsage() (err error) {
	defer errstk.Wrap(&err)
	return nil
}
