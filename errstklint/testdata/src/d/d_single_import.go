package d

import "fmt"

// Case G: Single import without errstk
func SingleImportFunc() error { // want "function SingleImportFunc returns error but missing defer errstk.Wrap\\(&err\\)"
	fmt.Println("hello")
	return nil
}
