package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
)

type eventPage []*FileEvent

func (p *eventPage) progress() progress {
	prog := make(progress)

	for _, event := range *p {
		if event.Source == "-" || event.Source == "" || event.Rotated {
			continue
		}

		ino, dev := file_ids(event.fileinfo)
		prog[event.Source] = &FileState{
			Source: event.Source,
			Offset: event.Offset + int64(len(event.Text)) + 1,
			Inode:  ino,
			Device: dev,
		}
	}

	return prog
}

func (p *eventPage) counts() (map[string]int, map[string]int) {
	total, bad := make(map[string]int), make(map[string]int)
	for _, event := range *p {
		if event.Source == "" {
			continue
		}
		total[event.Source] += 1
		if event.Rotated {
			bad[event.Source] += 1
		}
	}
	return total, bad
}

func (p *eventPage) countString() string {
	var buf bytes.Buffer
	counts, _ := p.counts()
	for path, count := range counts {
		fmt.Fprintf(&buf, "%s: %d, ", path, count)
	}
	s := buf.String()
	return s[0 : len(s)-2]
}

func (p *eventPage) empty() bool {
	return len(*p) == 0
}

// compress the event page into the destination buffer
func (p *eventPage) compress(sequenceId uint32, buf *bytes.Buffer) error {
	buf.Reset()
	z, err := zlib.NewWriterLevel(buf, 3)
	if err != nil {
		return fmt.Errorf("unable to compress eventPage: %v", err)
	}

	for i, e := range *p {
		e.writeFrame(z, sequenceId+uint32(i))
	}
	if err := z.Flush(); err != nil {
		return fmt.Errorf("unable to compress eventPage: %v", err)
	}
	if err := z.Close(); err != nil {
		return fmt.Errorf("unable to compress eventPage: %v", err)
	}
	return nil
}
