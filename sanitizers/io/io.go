package io

import (
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
)

func AddNewIoImport(fset *token.FileSet, file *ast.File, addNewIoPackage bool) {
	// Add new package:
	if addNewIoPackage {
		astutil.AddNamedImport(fset, file, "io2", "github.com/AdamKorcz/bugdetectors/io")
		return
	}

	// Change "io" to the new package
	astutil.AddNamedImport(fset, file, "io2", "github.com/AdamKorcz/bugdetectors/io")

	return
}