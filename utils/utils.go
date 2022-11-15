package utils

import (
	"go/token"
	"go/parser"
	"golang.org/x/tools/go/packages"
)


// LoadMode controls the amount of details to return when loading the packages
const LoadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedTypes |
	packages.NeedTypesSizes |
	packages.NeedTypesInfo |
	packages.NeedSyntax |
	packages.NeedModule

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

func LoadPackages(path string) []*packages.Package {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Mode: LoadMode,
		Fset: fset,
	}, "file="+path)
	if err != nil {
		panic(err)
	}
	return pkgs
}