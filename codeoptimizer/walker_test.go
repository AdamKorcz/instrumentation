package codeoptimizer

import (
	"fmt"
	"github.com/AdamKorcz/instrumentation/utils"
	"go/ast"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSimpleConditional(t *testing.T) {
	OptimizeConditionals("./testdata/simple.go")
}

func TestSimpleCodeOptimization(t *testing.T) {
	codeBeforeOptimization := `package codeoptimizer

import (
    "fmt"
)

func testConditional(a string) {
    if a == "adam" {
        fmt.Println("We have a winner")
    }
    if a == "adam2" {
        fmt.Println("We have another winner")
    }
}

`
	codeAfterOptimization := `package codeoptimizer

import (
    "fmt"
)

func testConditional(a string) {
    if (len(a) >= 1 && string(a[0]) == "a") && (len(a) >= 2 && string(a[1]) == "d") && (len(a) >= 3 && string(a[2]) == "a") && (len(a) >= 4 && string(a[3]) == "m") {
        fmt.Println("We have a winner")
    }
    if (len(a) >= 1 && string(a[0]) == "a") && (len(a) >= 2 && string(a[1]) == "d") && (len(a) >= 3 && string(a[2]) == "a") && (len(a) >= 4 && string(a[3]) == "m") && (len(a) >= 5 && string(a[4]) == "2") {
        fmt.Println("We have another winner")
    }
}

`
	pathDir := t.TempDir()
	filePath := fmt.Sprintf("%s/simple.go", pathDir)
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal("Should not fail")
	}
	defer f.Close()
	_, err = f.Write([]byte(codeBeforeOptimization))
	if err != nil {
		t.Fatal("Should not fail")
	}

	pkgs := utils.LoadPackages(filePath)

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			src, err := os.ReadFile(p.GoFiles[0]) // there should only be one
			if err != nil {
				panic(err)
			}
			walker := NewWalker(p.Fset, f, p.TypesInfo, src)
			ast.Walk(walker, f)

			rewrittenFile := string(walker.src)
			if rewrittenFile != codeAfterOptimization {
				diff := cmp.Diff(codeAfterOptimization, rewrittenFile)
				t.Fatalf("Wrong output. \n %s\n", diff)
			}
		}
	}
}

// Runtime.GOOS should not be rewritten
func TestRuntimeGoos(t *testing.T) {
	codeBeforeOptimization := `package codeoptimizer

import (
    "fmt"
    "runtime"
)

func testConditional(a string) {
    if runtime.GOOS == "adam" {
        fmt.Println("We have a winner")
    }
    if runtime.GOOS == "adam2" {
        fmt.Println("We have another winner")
    }
}

`
	codeAfterOptimization := `package codeoptimizer

import (
    "fmt"
    "runtime"
)

func testConditional(a string) {
    if runtime.GOOS == "adam" {
        fmt.Println("We have a winner")
    }
    if runtime.GOOS == "adam2" {
        fmt.Println("We have another winner")
    }
}

`
	pathDir := t.TempDir()
	filePath := fmt.Sprintf("%s/simple.go", pathDir)
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal("Should not fail")
	}
	defer f.Close()
	_, err = f.Write([]byte(codeBeforeOptimization))
	if err != nil {
		t.Fatal("Should not fail")
	}

	pkgs := utils.LoadPackages(filePath)

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			src, err := os.ReadFile(p.GoFiles[0]) // there should only be one
			if err != nil {
				panic(err)
			}
			walker := NewWalker(p.Fset, f, p.TypesInfo, src)
			ast.Walk(walker, f)

			rewrittenFile := string(walker.src)
			if rewrittenFile != codeAfterOptimization {
				diff := cmp.Diff(codeAfterOptimization, rewrittenFile)
				t.Fatalf("Wrong output. \n %s\n", diff)
			}
		}
	}
}
