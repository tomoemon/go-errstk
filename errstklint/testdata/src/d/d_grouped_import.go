package d

import (
	"fmt"
)

// Case F: Grouped import without errstk
func GroupedImportFunc() (string, error) { // want "function GroupedImportFunc returns error but missing defer errstk.Wrap\\(&err\\)"
	return fmt.Sprintf("hello"), nil
}
