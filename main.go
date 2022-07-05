package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	//"strconv"

	"runtime/debug"
	//"reflect"
	//bytesize "github.com/inhies/go-bytesize"
)

var (
	maxBufferSize = 1000000000
)

type Walker struct {
	fset            *token.FileSet
	file            *ast.File
	addNewIoPackage bool
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
						aa.X.(*ast.Ident).Name = "io2"
						aa.Sel.Name = "IoReadAll"
						//fmt.Println("We have a match")
						//fmt.Println("SELECTOR: ", reflect.TypeOf(aa.X), reflect.TypeOf(aa.Sel))
						//fmt.Println(aa.X.(*ast.Ident))
						//fmt.Println(aa.Sel.Name)
						return nil
						err := printer.Fprint(os.Stdout, walker.fset, walker.file)
						if err != nil {
							fmt.Println(err)
						}
					}
				}

			}
		}
	/*case *ast.SelectorExpr:
	if pack, ok := n.X.(*ast.Ident); ok {
		if pack.Name == "io" && n.Sel.Name != "ReadAll" {
			fmt.Println("We have a call to", n.Sel.Name)
		}
	}
	fmt.Println(reflect.TypeOf(n.X), n.X.(*ast.Ident).Name)*/
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

func two(r io.Reader) {
	start := time.Now()
	buf := new(bytes.Buffer)
	buf.ReadFrom(io.LimitReader(r, 4500000000))
	//fmt.Println("len: ", bytesize.New(float64(buf.Len())).String())
	//fmt.Println("len2: ", buf.Len())
	bufferLength := buf.Len()
	elapsed := time.Since(start)
	fmt.Println(elapsed)
	if bufferLength > maxBufferSize {
		debug.PrintStack()
		panic("A large buffer can be passed to an API that will exhaust this machines memory")
	}

}

func one(r io.Reader) {
	two(r)
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
	if len(file.Imports) == 0 {
		return false
	}
	for _, i := range file.Imports {
		if i != nil {
			if i.Path.Value == "\"io\"" {
				return true
			}
		}
	}
	return false
}

func (walker *Walker) addNewIoImport() {
	// if a new package should be added:
	if walker.addNewIoPackage {
		fmt.Println("heree")
		astutil.AddImport(walker.fset, walker.file, "io2")
		return
	}

	// if we should change "io" to the new package
	astutil.RewriteImport(walker.fset, walker.file, "io", "io2")
	return
}

func main() {
	filepath.Walk("test", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		if !isGoFile(info) {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		walker := &Walker{fset: fset, file: f}

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

		// If the file ueses other "io" apis, then we set that
		// we should add the new package instead of replacing
		walker.addNewIoPackage = ioWalker.UsesOtherIo
		walker.addNewIoImport()

		// Now walk and replace
		ast.Walk(walker, walker.file)
		printer.Fprint(os.Stdout, walker.fset, walker.file)
		return nil
	})
	return
	b := []byte{100, 101}
	r := bytes.NewReader(bytes.Repeat(b, 1000000000))
	one(r)
}
