package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	"os"
	"path/filepath"
	"strings"
)

type Walker struct {
	fset            *token.FileSet
	file            *ast.File
	addNewIoPackage bool
	hasReadAll		bool
}

func (walker *Walker) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return walker
	}
	switch n := node.(type) {
	case *ast.CallExpr:
		if aa, ok := n.Fun.(*ast.SelectorExpr); ok {
			if _, ok := aa.X.(*ast.Ident); ok {
				if aa.X.(*ast.Ident).Name == "io" {
					//fmt.Println("Counter")
					if aa.Sel.Name == "ReadAll" {
						// Now we have found an io.ReadAll()
						walker.hasReadAll = true
						aa.X.(*ast.Ident).Name = "io2"
						return nil
						err := printer.Fprint(os.Stdout, walker.fset, walker.file)
						if err != nil {
							fmt.Println(err)
						}
					}
				}

			}
		}
	default:
		//fmt.Println(reflect.TypeOf(n))
	}
	return walker
}

// This type is only used to check whether a file uses other
// Apis of the "io" package besides "ReadAll".
type IoUsageChecker struct {
	UsesOtherIo bool
}

// Checks whether a file uses any other Apis from the "io"
// besides "ReadAll"
func (walker *IoUsageChecker) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return walker
	}
	switch n := node.(type) {
	case *ast.SelectorExpr:
		if pack, ok := n.X.(*ast.Ident); ok {
			if pack.Name == "io" && n.Sel.Name != "ReadAll" {
				walker.UsesOtherIo = true
			}
		}
	}
	return walker
}

// Checks whether a path is a non-test go file
func isGoFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	ext := filepath.Ext(info.Name())
	if ext != ".go" || strings.Contains(info.Name(), "_test.go") {
		return false
	}
	return true
}

// Check whether a parsed file uses the "io" package
func (walker *Walker) usesIoPackage(file *ast.File) bool {
	return astutil.UsesImport(walker.file, "io")
}

func (walker *Walker) addNewIoImport() {
	// Add new package:
	if walker.addNewIoPackage {
		astutil.AddNamedImport(walker.fset, walker.file, "io2", "github.com/AdamKorcz/bugdetectors/io")
		return
	}

	// Change "io" to the new package
	astutil.DeleteImport(walker.fset, walker.file, "io")
	astutil.AddNamedImport(walker.fset, walker.file, "io2", "github.com/AdamKorcz/bugdetectors/io")
	return
}

func isTroubledDependency(path string) bool {
	// Build tags in std lib cause troubles
	if strings.Contains(path, "golang.org") {
		return true
	}
	// C bindings cause trouble
	if strings.Contains(path, "github.com/mattn/go-sqlite3") {
		return true
	}
	return false
}

func rewrite(p string) {
	filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		if !isGoFile(info) {
			return nil
		}

		if isTroubledDependency(path) {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		walker := &Walker{fset: fset, file: f, hasReadAll: false}

		// Check whether a file the "io" import.
		// Skip if it doesn't
		if !walker.usesIoPackage(f) {
			return nil
		}

		// Check whether a file uses any other parts of the
		// "io" package besides ReadAll(). This is to know
		// later whether "io" should be replaced or new
		// test package should be added
		ioWalker := &IoUsageChecker{}
		ast.Walk(ioWalker, f)

		// Now walk and replace
		ast.Walk(walker, walker.file)

		if walker.hasReadAll {
			// add imports
			walker.addNewIoPackage = ioWalker.UsesOtherIo
			walker.addNewIoImport()
		}
		var buf bytes.Buffer
		printer.Fprint(&buf, walker.fset, walker.file)

		os.Remove(path)
		newFile, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer newFile.Close()
		newFile.Write(buf.Bytes())
		return nil
	})
}

func main() {
	if len(os.Args)!=2 {
		panic("A path should be added")
	}
	dir := os.Args[1]
	rewrite(dir)
	return
}
