package test

import (
	"bytes"
	"io"
)

func Tester(r io.Reader) {
	if 1==2 {
		b := &bytes.Buffer{}
		_, _ = io.ReadAtLeast(r, b, 2)
	}
	_, _ = io.ReadAll(io.LimitReader(r, 2))
}