package main

import (
	"github.com/tomoemon/go-errstk/errstklint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(errstklint.Analyzer)
}
