package b

// This file should be excluded when using exclude pattern "**/*_test.go" or "**/mock_*.go"
func MockFunc() error {
	// Missing defer errstk.Wrap(&err) but should be excluded
	return nil
}
