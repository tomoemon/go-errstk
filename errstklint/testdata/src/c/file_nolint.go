//nolint:errstklint
package c

// This entire file is ignored by file-level nolint directive

func FileLevel1() (err error) {
	// No defer required - entire file ignored
	return nil
}

func FileLevel2() (err error) {
	// No defer required - entire file ignored
	return nil
}
