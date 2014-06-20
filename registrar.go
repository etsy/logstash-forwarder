package main

import (
	"bytes"
	"fmt"
	"log"
)

func Registrar(input chan []*FileEvent) {
	for page := range input {
		state := make(map[string]*FileState)
		counts := make(map[string]int)
		// Take the last event found for each file source
		for _, event := range page {
			// skip stdin
			if *event.Source == "-" {
				continue
			}

			if event.Source != nil {
				counts[*event.Source] += 1
			}

			// have to dereference the FileInfo here because os.FileInfo is an
			// interface, not a struct, so Go doesn't have smarts to call the Sys()
			// method on a pointer to os.FileInfo. :(
			ino, dev := file_ids(event.fileinfo)
			state[*event.Source] = &FileState{
				Source: event.Source,
				// take the offset + length of the line + newline char and
				// save it as the new starting offset.
				Offset: event.Offset + int64(len(*event.Text)) + 1,
				Inode:  ino,
				Device: dev,
			}
			//log.Printf("State %s: %d\n", *event.Source, event.Offset)
		}

		var buf bytes.Buffer
		for name, count := range counts {
			fmt.Fprintf(&buf, "%s: %d ", name, count)
		}

		log.Printf("Registrar received %d events. %s\n", len(page), buf.String())

		if len(state) > 0 {
			WriteRegistry(state, ".lumberjack")
		}
	}
}
