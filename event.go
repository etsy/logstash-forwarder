package main

import (
	"encoding/binary"
	"io"
	"os"
	"strconv"
)

// type FileEvent represents a single event in a log file.  I.e., it represents
// a single line in a logfile.
type FileEvent struct {
	Source  string
	Offset  int64
	Text    string
	Fields  map[string]string
	Rotated bool

	fileinfo os.FileInfo
}

func (e *FileEvent) writeFrame(w io.Writer, id uint32) {
	w.Write([]byte("1D"))
	binary.Write(w, binary.BigEndian, id)
	binary.Write(w, binary.BigEndian, uint32(len(e.Fields)+4))

	writeKV("file", e.Source, w)
	writeKV("host", hostname, w)
	writeKV("offset", strconv.FormatInt(e.Offset, 10), w)
	writeKV("line", e.Text, w)
	for k, v := range e.Fields {
		writeKV(k, v, w)
	}
}

func writeKV(key string, value string, output io.Writer) {
	binary.Write(output, binary.BigEndian, uint32(len(key)))
	output.Write([]byte(key))
	binary.Write(output, binary.BigEndian, uint32(len(value)))
	output.Write([]byte(value))
}
