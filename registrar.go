package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type progress map[string]*FileState

func (p *progress) load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to open progress file: %s", err.Error())
	}
	defer f.Close()

	*p = make(progress, 32)
	if err := json.NewDecoder(f).Decode(p); err != nil {
		return fmt.Errorf("unable to decode progress file: %s", err.Error())
	}
	return nil
}

// records positions of files read
func Registrar(input chan eventPage) {
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
			WriteRegistry(state, *history_path)
		}
	}
}
