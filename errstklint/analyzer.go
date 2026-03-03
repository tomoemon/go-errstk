package errstklint

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
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
			message := fmt.Sprintf(
				"function %s returns error but missing defer errstk.Wrap(&%s)",
				funcDecl.Name.Name, errorReturnName)

			// Skip auto-fix for functions with multiple error return values
			if countErrorReturns(funcDecl, pass.TypesInfo) > 1 {
				pass.Report(analysis.Diagnostic{
					Pos:     funcDecl.Pos(),
					Message: message + " (auto-fix unavailable: multiple error return values)",
				})
				return
			}

			var textEdits []analysis.TextEdit

			// If returns are unnamed, add edits to name them
			if !isNamedReturns(funcDecl.Type.Results) {
				textEdits = append(textEdits, buildReturnNamingEdits(funcDecl, pass)...)
			}

			// Add the defer statement
			textEdits = append(textEdits, buildDeferTextEdit(funcDecl, errorReturnName, pass))

			// Add import if needed
			file := findFileForPos(pass, funcDecl.Pos())
			if file != nil {
				if importEdit := buildImportTextEdit(file); importEdit != nil {
					textEdits = append(textEdits, *importEdit)
				}
			}

			pass.Report(analysis.Diagnostic{
				Pos:     funcDecl.Pos(),
				Message: message,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   "Add defer errstk.Wrap(&" + errorReturnName + ")",
					TextEdits: textEdits,
				}},
			})
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

// findFileForPos returns the *ast.File containing the given position.
func findFileForPos(pass *analysis.Pass, pos token.Pos) *ast.File {
	for _, f := range pass.Files {
		if f.FileStart <= pos && pos < f.FileEnd {
			return f
		}
	}
	return nil
}

// countErrorReturns counts how many return values have the error type.
func countErrorReturns(funcDecl *ast.FuncDecl, info *types.Info) int {
	if funcDecl.Type == nil || funcDecl.Type.Results == nil {
		return 0
	}
	count := 0
	for _, field := range funcDecl.Type.Results.List {
		typ := info.TypeOf(field.Type)
		if typ != nil && isErrorType(typ) {
			count++
		}
	}
	return count
}

// isNamedReturns checks whether the function's return parameters are all named.
func isNamedReturns(results *ast.FieldList) bool {
	if results == nil || len(results.List) == 0 {
		return false
	}
	for _, field := range results.List {
		if len(field.Names) == 0 {
			return false
		}
	}
	return true
}

// buildReturnNamingEdits returns TextEdits to convert unnamed returns to named returns.
func buildReturnNamingEdits(funcDecl *ast.FuncDecl, pass *analysis.Pass) []analysis.TextEdit {
	results := funcDecl.Type.Results
	if results == nil {
		return nil
	}

	if results.Opening.IsValid() {
		// Has parentheses: (string, error) -> (_ string, err error)
		var parts []string
		for _, field := range results.List {
			typeName := sourceText(pass, field.Type.Pos(), field.Type.End())
			if isErrorType(pass.TypesInfo.TypeOf(field.Type)) {
				parts = append(parts, "err "+typeName)
			} else {
				parts = append(parts, "_ "+typeName)
			}
		}
		newText := strings.Join(parts, ", ")
		return []analysis.TextEdit{{
			Pos:     results.Opening + 1,
			End:     results.Closing,
			NewText: []byte(newText),
		}}
	}

	// No parentheses: single `error` return -> `(err error)`
	field := results.List[0]
	return []analysis.TextEdit{{
		Pos:     field.Type.Pos(),
		End:     field.Type.End(),
		NewText: []byte("(err error)"),
	}}
}

// buildDeferTextEdit returns a TextEdit to insert the defer statement.
// For one-liner functions (opening and closing brace on the same line),
// the entire body content is reformatted to multi-line.
func buildDeferTextEdit(funcDecl *ast.FuncDecl, errorVarName string, pass *analysis.Pass) analysis.TextEdit {
	lbrace := funcDecl.Body.Lbrace
	rbrace := funcDecl.Body.Rbrace

	lbracePos := pass.Fset.Position(lbrace)
	rbracePos := pass.Fset.Position(rbrace)

	if lbracePos.Line == rbracePos.Line {
		// One-liner: extract body content and reformat to multi-line
		bodyContent := strings.TrimSpace(sourceText(pass, lbrace+1, rbrace))
		newBody := "\n\tdefer errstk.Wrap(&" + errorVarName + ")\n\t" + bodyContent + "\n"
		return analysis.TextEdit{
			Pos:     lbrace + 1,
			End:     rbrace,
			NewText: []byte(newBody),
		}
	}

	return analysis.TextEdit{
		Pos:     lbrace + 1,
		End:     lbrace + 1,
		NewText: []byte("\n\tdefer errstk.Wrap(&" + errorVarName + ")"),
	}
}

// hasErrstkImport checks if the file already imports errstk.
func hasErrstkImport(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == `"github.com/tomoemon/go-errstk"` {
			return true
		}
	}
	return false
}

// buildImportTextEdit returns a TextEdit to add the errstk import, or nil if already imported.
func buildImportTextEdit(file *ast.File) *analysis.TextEdit {
	if hasErrstkImport(file) {
		return nil
	}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		if gd.Lparen.IsValid() {
			// Grouped import: insert before closing paren
			return &analysis.TextEdit{
				Pos:     gd.Rparen,
				End:     gd.Rparen,
				NewText: []byte("\t\"github.com/tomoemon/go-errstk\"\n"),
			}
		}
		// Single-line import: insert after the import decl
		return &analysis.TextEdit{
			Pos:     gd.End(),
			End:     gd.End(),
			NewText: []byte("\nimport \"github.com/tomoemon/go-errstk\""),
		}
	}

	// No imports at all: insert after package name
	return &analysis.TextEdit{
		Pos:     file.Name.End(),
		End:     file.Name.End(),
		NewText: []byte("\n\nimport \"github.com/tomoemon/go-errstk\""),
	}
}

// sourceText extracts the source code between two positions.
func sourceText(pass *analysis.Pass, start, end token.Pos) string {
	tf := pass.Fset.File(start)
	if tf == nil {
		return ""
	}
	var content []byte
	var err error
	if pass.ReadFile != nil {
		content, err = pass.ReadFile(tf.Name())
	} else {
		content, err = os.ReadFile(tf.Name())
	}
	if err != nil {
		return ""
	}
	startOff := tf.Offset(start)
	endOff := tf.Offset(end)
	return string(content[startOff:endOff])
}
