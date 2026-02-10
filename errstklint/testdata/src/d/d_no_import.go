package d

// Case E: No import in file
func NoImportFunc() error { // want "function NoImportFunc returns error but missing defer errstk.Wrap\\(&err\\)"
	return nil
}
