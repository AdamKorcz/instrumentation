package test

import (
	"bytes"
	"io"
)

func Tester(r io.Reader) {
	_, _ = io.ReadAll(io.LimitReader(r, 2))
}

func Tester2() {
	var b bytes.Buffer
	_ = bytes.NewBuffer(b.Bytes())
}

func Tester3() {
	var b bytes.Buffer
	_ = bytes.NewBuffer(b.Bytes())
}
