package codeoptimizer

import (
	"fmt"
)

func testConditional(a string) {
	if string(a[0]) == "A" && string(a[1]) == "d" && string(a[2]) == "a" && string(a[3]) == "m" {
		fmt.Println("We have a winner")
	}
	if string(a[0]) == "A" && string(a[1]) == "d" && string(a[2]) == "a" && string(a[3]) == "m" && string(a[4]) == "2" {
		fmt.Println("We have another winner")
	}
}

