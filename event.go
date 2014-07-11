package main

import "os"

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
