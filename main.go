package main

import (
	//"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	instrmake "github.com/AdamKorcz/instrumentation/sanitizers/make"
	instrIo "github.com/AdamKorcz/instrumentation/sanitizers/io"
	"github.com/AdamKorcz/instrumentation/utils"
	"github.com/AdamKorcz/instrumentation/codeoptimizer"
)

var (
	devMode      = false // false = overwrite files with new bug detectors
	dummySnippet = "\"NotAvailable\""
)

type Walker struct {
	fset                *token.FileSet
	file                *ast.File
	addNewIoPackage     bool
	addNewIoutilPackage bool
	hasIoReadall        bool
	hasIoutilReadall    bool
	src                 []byte // contents of .go file being analyzed
	typesInfo           *types.Info
	textRewriters       []*utils.TextRewriter
}

// We use the string NEW_LINE instead of "\n"
// This is to not add extra lines in the source file.
// When the message gets printed, we should do a search
// and replace to correctly format the message.
// Todo: Get all locations of interesting calls before
// instrumenting and save these. Once we instrument,
// we get the calls position and use that.
func getStringVersion(n ast.Node, src []byte, fset *token.FileSet) string {
	//return dummySnippet
	start := n.Pos()
	end := n.End()

	//startf := fset.Position(n.Pos())
	fileAtPos := fset.File(n.Pos())
	//offSet := fileAtPos.Offset(n.Pos())

	snippetLength := int(end) - int(start)

	snippetStart := fileAtPos.Offset(n.Pos())	
	snippetEnd := snippetStart+snippetLength

	//fmt.Println(string(src[snippetStart:snippetEnd]))
	startf2 := fset.Position(fileAtPos.Pos(snippetStart))

	//fmt.Println(fileAtPos.Name(), offSet)

	var returnString strings.Builder

	// wrap the codeSnippet in quotes:
	returnString.WriteString("\"")
	returnString.WriteString(fmt.Sprintf("%s (May be slightly inaccurate) NEW_LINE", startf2))
	returnString.WriteString(string(src[snippetStart:snippetEnd]))
	returnString.WriteString("\"")
	return returnString.String()
}


func (walker *Walker) rewriteReadAll(n ast.Node, aa *ast.SelectorExpr) {
	apiName := aa.Sel.Name

	if apiName != "ReadAll" {
		return
	}

	var codeSnippet string
	src := walker.src
	if codeSnippet != "Could not generate code" {
		codeSnippet = getStringVersion(aa, src, walker.fset)
	}

	selectorName := aa.X.(*ast.Ident).Name
	if selectorName == "io" {
		walker.hasIoReadall = true
		aa.X.(*ast.Ident).Name = "io2"
	} else if selectorName == "ioutil" {
		walker.hasIoutilReadall = true
		aa.X.(*ast.Ident).Name = "ioutil2"
	}

	// Add call param
	n.(*ast.CallExpr).Args = append(n.(*ast.CallExpr).Args, ast.NewIdent(codeSnippet))
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
	addCustomBytesImport(walker.fset, walker.file)

	// TODO:Add the code line to the function call

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

func addCustomBytesImport(fset *token.FileSet, file *ast.File) {
	name := "customBytes"
	importPath := "github.com/AdamKorcz/bugdetectors/bytes"
	astutil.AddNamedImport(fset, file, name, importPath)
}

func (walker *Walker) typeName(expr ast.Expr) (string, error) {
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

func selectorIsIo(n ast.Node) bool {
	return n.(*ast.SelectorExpr).X.(*ast.Ident).Name == "io"
}

func selectorIsIoutil(n ast.Node) bool {
	return n.(*ast.SelectorExpr).X.(*ast.Ident).Name == "ioutil"
}

func (walker *Walker) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return walker
	}
	switch n := node.(type) {
	case *ast.CallExpr:
		if aa, ok := n.Fun.(*ast.SelectorExpr); ok {
			if _, ok := aa.X.(*ast.Ident); ok {
				if selectorIsIo(aa) || selectorIsIoutil(aa) {
					walker.rewriteReadAll(n, aa)
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

func (walker *Walker) deleteUnusedImports() {
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
	pkgs := utils.LoadPackages(path)

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			src, err := os.ReadFile(p.GoFiles[0]) // there should only be one
			if err != nil {
				panic(err)
			}
			walker := instrmake.NewWalker(p.Fset, f, p.TypesInfo, src)

			// Now walk and replace
			walker.CollectData()
			walker.SanitizeFile()
		}
	}
}

func (walker *Walker) AddSanitizers() {

	// collect io/ioutil usage
	ioWalker := &IoUsageChecker{}
	ast.Walk(ioWalker, walker.file)

	// Now walk and replace
	ast.Walk(walker, walker.file)

	// add imports
	if walker.hasIoReadall {
		instrIo.AddNewIoImport(walker.fset, walker.file, ioWalker.UsesOtherIo)
	}

	if walker.hasIoutilReadall {
		walker.addNewIoutilPackage = ioWalker.UsesOtherIoutil
		walker.addNewIoutilImport()
	}

	walker.deleteUnusedImports()
}

func (walker *Walker) UpdateSrcFiles(path string) {
	var buf bytes.Buffer
	printer.Fprint(&buf, walker.fset, walker.file)
	if devMode {
		return
	}
	os.Remove(path)
	newFile, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer newFile.Close()
	newFile.Write(buf.Bytes())
}

func addRemainingSanitizers(path string) {
	pkgs := utils.LoadPackages(path)
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
				src:              src,
				typesInfo:        p.TypesInfo,
				textRewriters:    make([]*utils.TextRewriter, 0),
			}

			walker.AddSanitizers()
			walker.UpdateSrcFiles(path)
		}
	}
}

func validateFilePath(path string, info os.FileInfo) error {
	if !isGoFile(info) {
		return fmt.Errorf("Skip file")
	}

	if isTroubledDependency(path) {
		return fmt.Errorf("Skip file")
	}

	// Do low-cost check on imports
	if !utils.CheckImports(path) {
		return fmt.Errorf("Skip file")
	}
	return nil
}

func sanitize(p string) {
	filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		err = validateFilePath(path, info)
		if err != nil {
			return nil
		}

		// Add sanitizers
		addMakeSanitizer(path)
		addRemainingSanitizers(path)

		return nil
	})
}

func optimize(p string) {
	filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		err = validateFilePath(path, info)
		if err != nil {
			return nil
		}

		codeoptimizer.OptimizeConditionals(path)

		return nil
	})
}

func main() {
	if len(os.Args) != 2 {
		panic("A path should be added")
	}
	dir := os.Args[1]

	optimize(dir)

	sanitize(dir)
	return
}
