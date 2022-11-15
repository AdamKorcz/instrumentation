package make

import (
	"fmt"
	"go/token"
	"go/types"
	"go/ast"
	"go/printer"
	"go/parser"
	"bytes"
	"strings"
	"os"
	"sort"
	"github.com/AdamKorcz/instrumentation/utils"
	"golang.org/x/tools/go/ast/astutil"
)

type MakeWalker struct {
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

func NewWalker(fset *token.FileSet, f *ast.File, ti *types.Info, src []byte) *MakeWalker {
	return  &MakeWalker{fset: fset,
					file:             f,
					hasIoReadall:     false,
					hasIoutilReadall: false,
					hasChanged:       false,
					src:              src,
					typesInfo:        ti,
					textRewriters:    make([]*utils.TextRewriter, 0),
				}
}

func isSelectorExpr(node ast.Node) bool {
	if _, ok := node.(*ast.SelectorExpr); ok {
		return true
	}
	return false
}

func (walker *MakeWalker) isSecondArgFileInfo(secondArg ast.Node) bool {
	typeName, err := walker.typeName(secondArg.(*ast.SelectorExpr).X.(*ast.Ident))
	if err != nil {
		return false
	}
	if typeName == "io/fs.FileInfo" {
		return true
	}
	return false
}

func (walker *MakeWalker) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return walker
	}
	switch n := node.(type) {
	case *ast.CallExpr:
		if _, ok := n.Fun.(*ast.Ident); ok {
			// functions we are interested in:
			// 1: make([]byte)
			if n.Fun.(*ast.Ident).Name == "make" {
				// for now we just support the "len" arg of make():
				if len(n.Args) == 2 {

					// Check type being created
					firstArg := n.Args[0]
					if walker.typeBeingCreated(firstArg) != "[]byte" {
						return walker
					}
					secondArg := n.Args[1]
					if isSelectorExpr(secondArg) {
						if _, ok := secondArg.(*ast.SelectorExpr).X.(*ast.Ident); !ok {
							return walker
						}
						if walker.isSecondArgFileInfo(secondArg) {
							// TODO: Refactor this part so it can be testec
							currentFilePath := walker.fset.File(secondArg.Pos()).Name()
							currentFileContents, err := os.ReadFile(currentFilePath)
							if err != nil {
								panic(err)
							}
							baseOffset := walker.fset.File(secondArg.Pos()).Base()
							start := int(n.Pos()) - baseOffset
							end := int(n.End()) - baseOffset
							replaceFrom := currentFileContents[start:end]

							secondParamStart := int(secondArg.Pos()) - baseOffset
							secondParamEnd := int(secondArg.End()) - baseOffset
							//updated2ndParam := strings.Split(string(replaceFrom), ",")[1]
							//updated2ndParam = strings.Split(updated2ndParam, ")")[0]
							//updated2ndParam = strings.TrimSpace(updated2ndParam)

							var b strings.Builder
							b.WriteString(string(currentFileContents[start:secondParamStart]))
							b.WriteString("lengthchecker.CheckLength(")
							b.WriteString(string(currentFileContents[secondParamStart:secondParamEnd]))
							b.WriteString("))")

							tr := &utils.TextRewriter{
								FilePath:    currentFilePath,
								StartOffset: int(n.Pos()) - baseOffset,
								EndOffset:   int(n.End()) - baseOffset,
								ReplaceFrom: string(replaceFrom),
								ReplaceTo:   b.String(),
							}
							walker.textRewriters = append(walker.textRewriters, tr)
						}
					}
				}
			}
		}
	}
	return walker
}

func (walker *MakeWalker) typeName(expr ast.Expr) (string, error) {
	if walker.typesInfo.TypeOf(expr) == nil {
		return "", fmt.Errorf("type not found")
	}
	return walker.typesInfo.TypeOf(expr).String(), nil
}

// Returns a type of a node as a string.
func (walker *MakeWalker) typeBeingCreated(n ast.Node) string {
	if n == nil {
		return ""
	}
	if walker.typesInfo == nil {
		return ""
	}
	if _, ok := n.(*ast.ArrayType); ok {
		if walker.typesInfo.TypeOf(n.(*ast.ArrayType)) == nil {
			return ""
		}
		return walker.typesInfo.TypeOf(n.(*ast.ArrayType)).String()
	}
	return ""
}

// Takes existing file contents and adds calls to bugdetector.
func (walker *MakeWalker) createNewFileBytes(tr *utils.TextRewriter, oldFileBytes []byte) []byte {
	var b strings.Builder
	b.WriteString(string(oldFileBytes[:tr.StartOffset]))
	b.WriteString(tr.ReplaceTo)
	b.WriteString(string(oldFileBytes[tr.EndOffset:]))

	return []byte(b.String())
}

// Overwrites the target file with updated file bytes containing sanitization.
func (walker *MakeWalker) createSanitizedFile(fileBytes []byte) error {
	fset := token.NewFileSet()
	newParsedFile, err := parser.ParseFile(fset, "", fileBytes, 0)
	if err != nil {
		return err
	}
	astutil.AddNamedImport(fset, newParsedFile, "lengthchecker", "github.com/AdamKorcz/bugdetectors/other")

	buf := new(bytes.Buffer)
	err = printer.Fprint(buf, fset, newParsedFile)
	if err != nil {
		return err
	}
	os.Remove(walker.textRewriters[0].FilePath)
	newFile, err := os.Create(walker.textRewriters[0].FilePath)
	if err != nil {
		return err
	}
	defer newFile.Close()
	newFile.Write(buf.Bytes())

	return nil
}

func (walker *MakeWalker) SanitizeFile() {
	if len(walker.textRewriters) != 0 {
		fmt.Println("There are files")
		fileBytes, err := os.ReadFile(walker.textRewriters[0].FilePath)
		if err != nil {
			panic(err)
		}
		for _, tr := range walker.textRewriters {
			// Do the rewriting here
			fileBytes = walker.createNewFileBytes(tr, fileBytes)
		}
		err = walker.createSanitizedFile(fileBytes)
		if err != nil {
			panic(err)
		}		
	}
}

func (walker *MakeWalker) CollectData() {
	ast.Walk(walker, walker.file)// Sort textRewriters by highest first

	// Sort rewriting data
	sort.Slice(walker.textRewriters[:], func(i, j int) bool {
		return walker.textRewriters[i].StartOffset > walker.textRewriters[j].StartOffset
	})
}