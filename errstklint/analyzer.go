package errstklint

import (
	"flag"
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = `checks that functions returning errors have defer errstk.Wrap(&err)

This analyzer ensures that all functions which return an error type
include a deferred call to errstk.Wrap(&err) at the beginning of the
function body. This is required to properly capture stack traces for
errors in code using the github.com/tomoemon/go-errstk library.

Example of correct usage:

	func GetUser(id string) (user *User, err error) {
		defer errstk.Wrap(&err)
		// function implementation
	}

The analyzer will report functions that:
- Return error (or multiple values including error)
- Do not have a defer statement calling errstk.Wrap with the error variable

Flags:
  -exclude  Comma-separated list of glob patterns to exclude (e.g., "generated/*.go,**/mock_*.go")
`

// Config holds the configuration for the analyzer
type Config struct {
	Exclude []string `json:"exclude" yaml:"exclude"`
}

var (
	excludeFlag string
	config      = &Config{}
)

var Analyzer = &analysis.Analyzer{
	Name:     "errstklint",
	Doc:      Doc,
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func init() {
	Analyzer.Flags.Init("errstklint", flag.ExitOnError)
	Analyzer.Flags.StringVar(&excludeFlag, "exclude", "", "comma-separated list of glob patterns to exclude")
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Parse exclude flag if provided
	excludePatterns := config.Exclude
	if excludeFlag != "" {
		excludePatterns = parseExcludeFlag(excludeFlag)
	}

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		funcDecl := n.(*ast.FuncDecl)

		// Skip functions without bodies (interface methods, external declarations)
		if funcDecl.Body == nil {
			return
		}

		// Get the file path for this function
		pos := pass.Fset.Position(funcDecl.Pos())
		if shouldExclude(pos.Filename, excludePatterns) {
			return
		}

		// Check if function returns error
		errorReturnName := getErrorReturnName(funcDecl, pass.TypesInfo)
		if errorReturnName == "" {
			return // No error return
		}

		// Check for defer errstk.Wrap()
		if !hasDeferErrStkWrap(funcDecl, errorReturnName) {
			pass.Reportf(funcDecl.Pos(),
				"function %s returns error but missing defer errstk.Wrap(&%s)",
				funcDecl.Name.Name, errorReturnName)
		}
	})

	return nil, nil
}

// getErrorReturnName returns the name of the error return variable,
// or empty string if function doesn't return error.
// For named returns, it uses the declared name.
// For unnamed returns, it returns "err" as the conventional name.
func getErrorReturnName(funcDecl *ast.FuncDecl, info *types.Info) string {
	if funcDecl.Type == nil || funcDecl.Type.Results == nil {
		return ""
	}

	for _, field := range funcDecl.Type.Results.List {
		typ := info.TypeOf(field.Type)
		if typ == nil {
			continue
		}

		if isErrorType(typ) {
			// Named return: use the declared name
			if len(field.Names) > 0 {
				return field.Names[0].Name
			}
			// Unnamed return: use conventional name "err"
			// Note: Without named returns, we can't verify the variable name
			// in defer, so we assume "err" convention
			return "err"
		}
	}

	return ""
}

// isErrorType checks if the type is Go's built-in error interface type
func isErrorType(t types.Type) bool {
	// Handle named types
	if n, ok := t.(*types.Named); ok {
		obj := n.Obj()
		// error is predeclared (no package) and named "error"
		return obj != nil && obj.Pkg() == nil && obj.Name() == "error"
	}
	return false
}

// hasDeferErrStkWrap checks if the function has a defer statement
// calling errstk.Wrap(&errorVar) or similar pattern
func hasDeferErrStkWrap(funcDecl *ast.FuncDecl, errorVar string) bool {
	if funcDecl.Body == nil {
		return false
	}

	for _, stmt := range funcDecl.Body.List {
		deferStmt, ok := stmt.(*ast.DeferStmt)
		if !ok {
			continue
		}

		if isDeferErrStkWrap(deferStmt, errorVar) {
			return true
		}
	}

	return false
}

// isDeferErrStkWrap checks if a defer statement is calling errstk.Wrap(&err)
func isDeferErrStkWrap(deferStmt *ast.DeferStmt, errorVar string) bool {
	callExpr, ok := deferStmt.Call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if method name is "Wrap"
	if callExpr.Sel.Name != "Wrap" {
		return false
	}

	// Check if package identifier looks like errstk
	ident, ok := callExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Accept "errstk" or any identifier (could be an alias)
	// For more strict checking, we could use types.Info to verify
	// the actual package, but checking the name is usually sufficient
	if ident.Name != "errstk" {
		// Allow other names (package aliases), but at least verify
		// the method name is "Wrap"
		// We could add configuration to allow other package names
	}

	// Check if the argument is &errorVar
	if len(deferStmt.Call.Args) == 0 {
		return false
	}

	unary, ok := deferStmt.Call.Args[0].(*ast.UnaryExpr)
	if !ok {
		return false
	}

	if unary.Op.String() != "&" {
		return false
	}

	argIdent, ok := unary.X.(*ast.Ident)
	if !ok {
		return false
	}

	return argIdent.Name == errorVar
}

// parseExcludeFlag parses the comma-separated exclude flag
func parseExcludeFlag(flag string) []string {
	if flag == "" {
		return nil
	}
	patterns := strings.Split(flag, ",")
	result := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// shouldExclude checks if the file should be excluded based on glob patterns
func shouldExclude(filename string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	// Normalize the filename to use forward slashes
	filename = filepath.ToSlash(filename)

	for _, pattern := range patterns {
		// Normalize pattern to use forward slashes
		pattern = filepath.ToSlash(pattern)

		// Try exact match
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}

		// Try matching against the base name
		matched, err = filepath.Match(pattern, filepath.Base(filename))
		if err == nil && matched {
			return true
		}

		// Handle ** patterns (match any number of directories)
		if strings.Contains(pattern, "**") {
			if matchDoubleStarPattern(filename, pattern) {
				return true
			}
		}
	}

	return false
}

// matchDoubleStarPattern handles glob patterns with ** (matches any number of directories)
func matchDoubleStarPattern(filename, pattern string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")
	if len(parts) == 0 {
		return false
	}

	// Check if filename starts with the first part (if not empty)
	currentPos := 0
	for i, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}

		if i == 0 {
			// First part: must match from the beginning or after any directory
			if !strings.HasPrefix(filename, part) {
				// Try matching after any directory separator
				idx := strings.Index(filename[currentPos:], part)
				if idx == -1 {
					return false
				}
				currentPos += idx + len(part)
			} else {
				currentPos = len(part)
			}
		} else if i == len(parts)-1 {
			// Last part: must match at the end
			matched, _ := filepath.Match(part, filepath.Base(filename))
			if matched {
				return true
			}
			if strings.HasSuffix(filename, part) {
				return true
			}
			// Last part didn't match
			return false
		} else {
			// Middle parts: must appear in order
			idx := strings.Index(filename[currentPos:], part)
			if idx == -1 {
				return false
			}
			currentPos += idx + len(part)
		}
	}

	// If we get here, all parts were empty (shouldn't happen in practice)
	return false
}

// SetConfig sets the configuration for the analyzer (used by golangci-lint plugin)
func SetConfig(cfg *Config) {
	if cfg != nil {
		config = cfg
	}
}
