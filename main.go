package main

import (
	//"bufio"
	"bytes"
	"fmt"
	"go/ast"
	//"go/importer"
	//"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
	//"io"
	"io/ioutil"
	//"io/fs"
	"os"
	"path/filepath"
	//"runtime"
	//"reflect"
	"strings"

	instrmake "github.com/AdamKorcz/instrumentation/sanitizers/make"
	"github.com/AdamKorcz/instrumentation/utils"
	"golang.org/x/tools/go/packages"
)

var (
	devMode      = false // false = overwrite files with new bug detectors
	dummySnippet = "\"NotAvailable\""
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

type Walker struct {
	fset                *token.FileSet
	file                *ast.File
	addNewIoPackage     bool
	addNewIoutilPackage bool
	hasIoReadall        bool
	hasIoutilReadall    bool
	hasChanged          bool
	src                 []byte // contents of .go file being analyzed
	typesInfo           *types.Info
	textRewriters       []*utils.TextRewriter
}

// We use the string NEW_LINE instead of "\n"
// This is to not add extra lines in the source file.
// When the message gets printed, we should do a search
// and replace to correctly format the message.
func getStringVersion(n ast.Node, src []byte, fset *token.FileSet) string {
	return dummySnippet
	start := n.Pos()
	end := n.End()
	startf := fset.Position(n.Pos())

	var returnString strings.Builder

	// wrap the codeSnippet in quotes:
	returnString.WriteString("\"")
	returnString.WriteString(fmt.Sprintf("%sNEW_LINE", startf))
	returnString.WriteString(string(src[start-1 : end-1]))
	returnString.WriteString("\"")
	return returnString.String()
}

func (walker *Walker) rewriteIoReadAll(n ast.Node, aa *ast.SelectorExpr) {
	if aa.Sel.Name == "ReadAll" {
		// Now we have found an io.ReadAll()

		// First we obtain the line number
		// and code.
		var codeSnippet string
		src := walker.src
		if codeSnippet != "Could not generate code" {
			codeSnippet = getStringVersion(aa, src, walker.fset)
		}
		walker.hasIoReadall = true
		aa.X.(*ast.Ident).Name = "io2"

		// Add the code line to the function call
		n.(*ast.CallExpr).Args = append(n.(*ast.CallExpr).Args, ast.NewIdent(codeSnippet))
		walker.hasChanged = true

		return
		// This prints out the end result.
		// It is useful for testing.
		err := printer.Fprint(os.Stdout, walker.fset, walker.file)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (walker *Walker) rewriteIoutilReadAll(n ast.Node, aa *ast.SelectorExpr) {
	if aa.Sel.Name == "ReadAll" {
		// Now we have found an ioutil.ReadAll()

		// First we obtain the line number
		// and code.
		var codeSnippet string
		src := walker.src
		if codeSnippet != "Could not generate code" {
			codeSnippet = getStringVersion(n, src, walker.fset)
		}
		walker.hasIoutilReadall = true
		aa.X.(*ast.Ident).Name = "ioutil2"

		// Add the code line to the function call
		n.(*ast.CallExpr).Args = append(n.(*ast.CallExpr).Args, ast.NewIdent(codeSnippet))
		walker.hasChanged = true

		return
		// This prints out the end result.
		// It is useful for testing.
		err := printer.Fprint(os.Stdout, walker.fset, walker.file)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (walker *Walker) rewriteBufferBytes(n ast.Node, aa *ast.SelectorExpr) {
	// Now we have found a Buffer.Bytes()

	// First we obtain the line number
	// and code.
	var codeSnippet string
	src := walker.src
	if codeSnippet != "Could not generate code" {
		codeSnippet = getStringVersion(n, src, walker.fset)
	}
	astutil.AddNamedImport(walker.fset, walker.file, "customBytes", "github.com/AdamKorcz/bugdetectors/bytes")

	// Add the code line to the function call

	// Copy the existing function call
	x := aa.X.(*ast.Ident).Name
	name := aa.Sel.Name

	// This naming is a bit hacky. TODO: Clean it up.
	args := []ast.Expr{&ast.SelectorExpr{X: ast.NewIdent(x), Sel: ast.NewIdent(name + "()")}}
	args = append(args, ast.NewIdent(codeSnippet))

	// Wrap the existing function call in customBytes.CheckLen()
	aa.X.(*ast.Ident).Name = "customBytes"
	aa.Sel.Name = "CheckLen"
	n.(*ast.CallExpr).Args = args

	// This prints out the end result.
	// It is useful for testing.
	if !devMode {
		return
	}
	err := printer.Fprint(os.Stdout, walker.fset, walker.file)
	if err != nil {
		fmt.Println(err)
	}
}

func (walker *Walker) typeName(expr ast.Expr) (string, error) {
	//fmt.Println("walker.typesInfo: ", walker.typesInfo)
	//fmt.Println("walker.typesInfo.TypeOf(expr) = ", walker.typesInfo.TypeOf(expr))
	if walker.typesInfo.TypeOf(expr) == nil {
		return "", fmt.Errorf("type not found")
	}
	return walker.typesInfo.TypeOf(expr).String(), nil
}

func (walker *Walker) typeBeingCreated(n ast.Node) string {
	if _, ok := n.(*ast.ArrayType); ok {
		return walker.typesInfo.TypeOf(n.(*ast.ArrayType)).String()
	}
	return ""
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
					walker.rewriteIoReadAll(n, aa)
				}
				if aa.X.(*ast.Ident).Name == "ioutil" {
					walker.rewriteIoutilReadAll(n, aa)
				}
				if aa.Sel.Name == "Bytes" {
					if typeName, err := walker.typeName(aa.X); err == nil {
						if typeName == "bytes.Buffer" || typeName == "*bytes.Buffer" {
							walker.rewriteBufferBytes(n, aa)
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
	UsesOtherIo     bool
	UsesOtherIoutil bool
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
			if pack.Name == "ioutil" && n.Sel.Name != "ReadAll" {
				walker.UsesOtherIoutil = true
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
	astutil.AddNamedImport(walker.fset, walker.file, "io2", "github.com/AdamKorcz/bugdetectors/io")

	return
}

func (walker *Walker) addNewIoutilImport() {
	// Add new package:
	if walker.addNewIoPackage {
		astutil.AddNamedImport(walker.fset, walker.file, "ioutil2", "github.com/AdamKorcz/bugdetectors/ioutil")

		return
	}

	// Change "io" to the new package
	astutil.AddNamedImport(walker.fset, walker.file, "ioutil2", "github.com/AdamKorcz/bugdetectors/ioutil")

	return
}

func (walker *Walker) deleteImports() {
	if !astutil.UsesImport(walker.file, "io") {
		astutil.DeleteImport(walker.fset, walker.file, "io")

	}
	if !astutil.UsesImport(walker.file, "io/ioutil") {
		astutil.DeleteImport(walker.fset, walker.file, "io/ioutil")

	}
}

// Some packages will require a little more work
// There are different reasons for this, with eg. C
// bindings and build tags. For now we just ignore
// these dependencies.
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

func getAllGoFilesInDir(path string) []string {
	listOfFiles := make([]string, 0)
	files, err := ioutil.ReadDir(path)
	if err != nil {

		panic(err)
	}

	for _, file := range files {
		if !isGoFile(file) {
			continue
		}
		
		fileName := filepath.Join(path, file.Name())
		listOfFiles = append(listOfFiles, fileName)
	}
	return listOfFiles
}

func addMakeSanitizer(path string) {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Mode: LoadMode,
		Fset: fset,
	}, "file="+path)
	if err != nil {
		panic(err)
	}

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			src, err := os.ReadFile(p.GoFiles[0]) // there should only be one
			if err != nil {
				panic(err)
			}
			walker := instrmake.NewWalker(p.Fset, f, p.TypesInfo, src)

			// Now walk and replace
			walker.CollectData()
			fmt.Println("Sanitizing...")
			walker.SanitizeFile()
		}
	}
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

		// Do low-cost check on imports
		if !utils.CheckImports(path) {
			return nil
		}

		addMakeSanitizer(path)

		fset := token.NewFileSet()
		pkgs, err := packages.Load(&packages.Config{
			Mode: LoadMode,
			Fset: fset,
		}, "file="+path)

		for _, p := range pkgs {
			for _, f := range p.Syntax {

				src, err := os.ReadFile(p.GoFiles[0]) // there should only be one
				if err != nil {
					panic(err)
				}
				walker := &Walker{fset: p.Fset,
					file:             f,
					hasIoReadall:     false,
					hasIoutilReadall: false,
					hasChanged:       false,
					src:              src,
					typesInfo:        p.TypesInfo,
					textRewriters:    make([]*utils.TextRewriter, 0),
				}
				ioWalker := &IoUsageChecker{}
				ast.Walk(ioWalker, f)

				// Now walk and replace
				ast.Walk(walker, walker.file)
				// Should also add the import here

				if walker.hasIoReadall {
					// add imports
					walker.addNewIoPackage = ioWalker.UsesOtherIo
					walker.addNewIoImport()
				}

				if walker.hasIoutilReadall {
					// add imports
					walker.addNewIoutilPackage = ioWalker.UsesOtherIoutil
					walker.addNewIoutilImport()
				}

				walker.deleteImports()

				var buf bytes.Buffer
				printer.Fprint(&buf, walker.fset, walker.file)
				if devMode {
					return nil
				}
				os.Remove(path)
				newFile, err := os.Create(path)
				if err != nil {
					panic(err)
				}
				defer newFile.Close()
				newFile.Write(buf.Bytes())
				return nil
			}
		}
		return nil
	})
}

func main() {
	if len(os.Args) != 2 {
		panic("A path should be added")
	}
	dir := os.Args[1]
	rewrite(dir)
	return
}
