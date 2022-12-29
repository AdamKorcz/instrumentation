package codeoptimizer

import (
	"fmt"
)

func testConditional(a string) {
	if a == "Adam" {
		fmt.Println("We have a winner")
	}
	if a == "Adam2" {
		fmt.Println("We have another winner")
	}
}

