package errstklint

import (
	"flag"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
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

Excluding specific functions:

You can use nolint directives to exclude specific functions or files:

  // Function-level exclusion
  //nolint:errstklint
  func HelperFunc() (err error) { ... }

  // Alternative format
  //lint:ignore errstklint reason for exclusion
  func HelperFunc() (err error) { ... }

  // File-level exclusion (before package declaration)
  //nolint:errstklint
  package mypackage

  // Alternative file-level format
  //lint:file-ignore errstklint reason for exclusion
  package mypackage

Flags:
  -exclude  Comma-separated list of glob patterns to exclude (e.g., "generated/*.go,**/mock_*.go")
`

// Config holds the configuration for the analyzer
type Config struct {
	Exclude []string `json:"exclude" yaml:"exclude"`
}

// ignoredRange represents a range of lines to ignore
type ignoredRange struct {
	start int
	end   int
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

	// Parse nolint directives for each file
	ignoredRanges := make(map[string][]ignoredRange)
	for _, f := range pass.Files {
		filename := pass.Fset.Position(f.Pos()).Filename
		ignoredRanges[filename] = parseNolintDirectives(f, pass.Fset)
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

		// Check if this position is ignored by nolint directive
		if isPositionIgnored(pos, ignoredRanges[pos.Filename]) {
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

var (
	nolintPattern    = regexp.MustCompile(`^nolint(?::([\w,]+))?(?:\s|$)`)
	lintIgnorePattern = regexp.MustCompile(`^lint:(ignore|file-ignore)\s+(\S+)(?:\s+(.+))?$`)
)

// parseNolintDirectives parses nolint and lint:ignore directives from file comments
func parseNolintDirectives(file *ast.File, fset *token.FileSet) []ignoredRange {
	var ranges []ignoredRange
	fileStart := fset.Position(file.Pos()).Line
	fileEnd := fset.Position(file.End()).Line

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			text := strings.TrimPrefix(c.Text, "//")
			text = strings.TrimSpace(text)

			// Check for nolint directive
			if matches := nolintPattern.FindStringSubmatch(text); matches != nil {
				linters := matches[1]
				if shouldIgnoreLinter(linters) {
					commentLine := fset.Position(c.Pos()).Line
					ranges = append(ranges, createIgnoredRange(commentLine, cg, file, fset, fileStart, fileEnd))
				}
				continue
			}

			// Check for lint:ignore or lint:file-ignore directive
			if matches := lintIgnorePattern.FindStringSubmatch(text); matches != nil {
				directiveType := matches[1] // "ignore" or "file-ignore"
				checkName := matches[2]

				if checkName == "errstklint" {
					commentLine := fset.Position(c.Pos()).Line
					if directiveType == "file-ignore" {
						// File-level ignore: ignore entire file
						ranges = append(ranges, ignoredRange{start: fileStart, end: fileEnd})
					} else {
						// Function-level ignore
						ranges = append(ranges, createIgnoredRange(commentLine, cg, file, fset, fileStart, fileEnd))
					}
				}
				continue
			}
		}
	}

	return ranges
}

// shouldIgnoreLinter checks if the linter list includes "errstklint" or "all"
func shouldIgnoreLinter(linters string) bool {
	if linters == "" || linters == "all" {
		return true
	}
	for _, l := range strings.Split(linters, ",") {
		if strings.TrimSpace(l) == "errstklint" {
			return true
		}
	}
	return false
}

// createIgnoredRange creates an ignored range based on the comment position
func createIgnoredRange(commentLine int, cg *ast.CommentGroup, file *ast.File, fset *token.FileSet, fileStart, fileEnd int) ignoredRange {
	// Check if this is a file-level directive (before package declaration)
	pkgLine := fset.Position(file.Package).Line
	if commentLine < pkgLine {
		return ignoredRange{start: fileStart, end: fileEnd}
	}

	// Find the next node after this comment to determine the range
	commentPos := cg.Pos()
	var nextNode ast.Node

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		// Skip the comment itself
		if n.Pos() <= commentPos {
			return true
		}
		// Find the first node after the comment
		if nextNode == nil && n.Pos() > commentPos {
			// Only consider function declarations and other top-level declarations
			switch n.(type) {
			case *ast.FuncDecl, *ast.GenDecl:
				nextNode = n
				return false
			}
		}
		return nextNode == nil
	})

	if nextNode != nil {
		// Ignore the range from comment to end of the next node
		start := fset.Position(nextNode.Pos()).Line
		end := fset.Position(nextNode.End()).Line
		return ignoredRange{start: start, end: end}
	}

	// If no next node found, ignore just the comment line
	return ignoredRange{start: commentLine, end: commentLine}
}

// isPositionIgnored checks if a position is within any ignored range
func isPositionIgnored(pos token.Position, ranges []ignoredRange) bool {
	for _, r := range ranges {
		if pos.Line >= r.start && pos.Line <= r.end {
			return true
		}
	}
	return false
}
