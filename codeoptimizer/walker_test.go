package codeoptimizer

import (
	"testing"
)

func TestSimpleConditional(t *testing.T) {
	OptimizeConditionals("./testdata/simple.go")
}