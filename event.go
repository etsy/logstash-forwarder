package main

import "os"

// type FileEvent represents a single event in a log file.  I.e., it represents
// a single line in a logfile.
type FileEvent struct {
	Source  *string `json:"source,omitempty"`
	Offset  int64   `json:"offset,omitempty"`
	Line    uint64  `json:"line,omitempty"`
	Text    *string `json:"text,omitempty"`
	Fields  map[string]string
	Rotated bool

	fileinfo os.FileInfo
}
