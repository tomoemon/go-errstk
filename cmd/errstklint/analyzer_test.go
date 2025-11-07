package main

import (
	"testing"

	"github.com/tomoemon/go-errstk/errstklint"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, errstklint.Analyzer, "a")
}
