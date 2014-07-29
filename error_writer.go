package main

import (
	"io"
)

type errorWriter struct {
	io.Writer
	err error
	sum int
}

func (e *errorWriter) Write(b []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	n, err := e.Writer.Write(b)
	e.sum += n
	e.err = err
	return n, err
}

func (e *errorWriter) Sum() int {
	return e.sum
}

func (e *errorWriter) Err() error {
	return e.err
}
