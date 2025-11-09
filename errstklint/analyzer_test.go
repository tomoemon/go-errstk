package errstklint

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "a")
}

func TestAnalyzerWithNolint(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "c")
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		patterns []string
		want     bool
	}{
		{
			name:     "no patterns",
			filename: "/path/to/file.go",
			patterns: []string{},
			want:     false,
		},
		{
			name:     "wildcard directory match",
			filename: "/path/to/generated.go",
			patterns: []string{"**/generated.go"},
			want:     true,
		},
		{
			name:     "base name match with star",
			filename: "/path/to/file_gen.go",
			patterns: []string{"*_gen.go"},
			want:     true,
		},
		{
			name:     "double star prefix match .yo.go",
			filename: "/path/to/deep/model.yo.go",
			patterns: []string{"**/*.yo.go"},
			want:     true,
		},
		{
			name:     "double star prefix match .pb.go",
			filename: "/path/to/proto/user.pb.go",
			patterns: []string{"**/*.pb.go"},
			want:     true,
		},
		{
			name:     "double star prefix no match",
			filename: "/path/to/file.go",
			patterns: []string{"**/*.yo.go"},
			want:     false,
		},
		{
			name:     "double star with directory path match",
			filename: "/project/service/api/handler/user.go",
			patterns: []string{"**/service/api/**/*.go"},
			want:     true,
		},
		{
			name:     "double star with directory path no match",
			filename: "/project/infra/persistence/dao/user.go",
			patterns: []string{"**/service/api/**/*.go"},
			want:     false,
		},
		{
			name:     "multiple patterns first matches",
			filename: "/path/to/file.pb.go",
			patterns: []string{"**/*.pb.go", "**/*.yo.go"},
			want:     true,
		},
		{
			name:     "multiple patterns second matches",
			filename: "/path/to/file.yo.go",
			patterns: []string{"**/*.pb.go", "**/*.yo.go"},
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			filename: "/path/to/file.go",
			patterns: []string{"**/*.pb.go", "**/*.yo.go"},
			want:     false,
		},
		{
			name:     "regression: usernotification.go vs *.yo.go",
			filename: "./infra/persistence/spanner_dao/usernotification.go",
			patterns: []string{"**/*.yo.go"},
			want:     false,
		},
		{
			name:     "regression: usernotification.go vs *.pb.go",
			filename: "./infra/persistence/spanner_dao/usernotification.go",
			patterns: []string{"**/*.pb.go"},
			want:     false,
		},
		{
			name:     "regression: usernotification.go vs service/api",
			filename: "./infra/persistence/spanner_dao/usernotification.go",
			patterns: []string{"**/service/api/**/*.go"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExclude(tt.filename, tt.patterns)
			if got != tt.want {
				t.Errorf("shouldExclude(%q, %v) = %v, want %v",
					tt.filename, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestMatchDoubleStarPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		pattern  string
		want     bool
	}{
		{
			name:     "simple double star suffix match",
			filename: "path/to/file.pb.go",
			pattern:  "**/*.pb.go",
			want:     true,
		},
		{
			name:     "simple double star suffix no match",
			filename: "path/to/file.go",
			pattern:  "**/*.pb.go",
			want:     false,
		},
		{
			name:     "double star with middle path match",
			filename: "project/service/api/handler/user.go",
			pattern:  "**/service/api/**/*.go",
			want:     true,
		},
		{
			name:     "double star with middle path no match - wrong directory",
			filename: "project/infra/persistence/user.go",
			pattern:  "**/service/api/**/*.go",
			want:     false,
		},
		{
			name:     "regression test: usernotification.go should not match *.yo.go",
			filename: "./infra/persistence/spanner_dao/usernotification.go",
			pattern:  "**/*.yo.go",
			want:     false,
		},
		{
			name:     "regression test: model.yo.go should match *.yo.go",
			filename: "./infra/persistence/model.yo.go",
			pattern:  "**/*.yo.go",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchDoubleStarPattern(tt.filename, tt.pattern)
			if got != tt.want {
				t.Errorf("matchDoubleStarPattern(%q, %q) = %v, want %v",
					tt.filename, tt.pattern, got, tt.want)
			}
		})
	}
}
