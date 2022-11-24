package codeoptimizer

import (
	"fmt"
	"go/token"
	"go/types"
	"go/ast"
	"os"
	"strings"
	"github.com/AdamKorcz/instrumentation/utils"
)

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
	rewritten  			[]*rewrittenIndeces
	additionalOffset    int
}

type rewrittenIndeces struct {
	start  				int
	end  				int
}

func NewWalker(fset *token.FileSet, f *ast.File, ti *types.Info, src []byte) *Walker {
	return  &Walker{fset: fset,
					file:             f,
					hasIoReadall:     false,
					hasIoutilReadall: false,
					hasChanged:       false,
					src:              src,
					typesInfo:        ti,
					textRewriters:    make([]*utils.TextRewriter, 0),
					rewritten:  	  make([]*rewrittenIndeces, 0),
					additionalOffset: 0,
				}
}

func (walker *Walker) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return walker
	}
	switch n := node.(type) {
	case *ast.IfStmt:
		if be, ok := n.Cond.(*ast.BinaryExpr); ok {
			if be.Op.String() == "==" {
				if stringLit, ok := be.Y.(*ast.BasicLit); ok {
					if stringLit.Kind == token.STRING {

						yValue := stringLit.Value[1:len(stringLit.Value)-1]

						baseOffset := walker.fset.File(n.Pos()).Base()
						start := int(be.Pos()) - baseOffset + walker.additionalOffset
						end := int(stringLit.End()) - baseOffset + walker.additionalOffset

						fmt.Println(start, end)
						//currentFilePath := walker.fset.File(n.Pos()).Name()
						currentFileContents := walker.src
						fmt.Println(string(currentFileContents))
						conditionalStmtString := string(currentFileContents[start:end])
						xString := strings.Split(conditionalStmtString, " ==")[0]
						fileContentsUntilConditional := string(currentFileContents[:start])
						fileContentsAfterConditional := string(currentFileContents[end:])
						
						var b strings.Builder
						b.WriteString(fileContentsUntilConditional)
						valueLen := len(yValue)

						var condWriter strings.Builder

						for i:=0; i<valueLen; i++ {
							condWriter.WriteString(fmt.Sprintf("string(%s[%d]) == \"%c\"", xString, i, yValue[i]))

							if i != valueLen-1 {
								condWriter.WriteString(" && ")
							}
						}
						walker.additionalOffset = walker.additionalOffset + (len(condWriter.String()) - (end-start))
						b.WriteString(condWriter.String())
						b.WriteString(fileContentsAfterConditional)

						
					    walker.src = []byte(b.String())
					    return nil
					}
				}
			}
		}
	}
	return walker
}

func OptimizeConditionals(path string) {
	pkgs := utils.LoadPackages(path)

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			src, err := os.ReadFile(p.GoFiles[0]) // there should only be one
			fmt.Println(src)
			if err != nil {
				panic(err)
			}
			walker := NewWalker(p.Fset, f, p.TypesInfo, src)
			ast.Walk(walker, f)
			rewrittenFile, err := os.OpenFile(p.GoFiles[0], os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		    if err != nil {
		        panic(err)
		    }
		    defer rewrittenFile.Close()
		    rewrittenFile.Write(walker.src)
		}
	}
	panic("Done")
}