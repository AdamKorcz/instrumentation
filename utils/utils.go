package utils

import (
	"go/token"
	"go/parser"
)


type TextRewriter struct {
	FilePath     string
	FileContents []byte
	ReplaceFrom  string
	ReplaceTo    string
	StartOffset  int
	EndOffset    int
}

func isInterestingImport(impName string) bool {
	interestingImports := []string{
		"\"io\"", "io", "\"io/ioutil\"", "io/ioutil", "\"bytes\"", "bytes",
	}
	for _, imp := range interestingImports {
		if impName == imp {
			return true
		}
	}
	return false
}

func CheckImports(path string) bool {
	fsetCheck := token.NewFileSet()
	fCheck, err := parser.ParseFile(fsetCheck, path, nil, parser.ImportsOnly)
	if err != nil {
		return false
	}

	var hasInterestingImport bool
	hasInterestingImport = false
	for _, imp := range fCheck.Imports {
		if isInterestingImport(imp.Path.Value) {
			hasInterestingImport = true
			break
		}
	}
	if !hasInterestingImport {
		return false
	}
	return true
}