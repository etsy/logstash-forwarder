package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os" // for File and friends
	"strings"
	"time"
)

// type Harvester is responsible for tailing a single log file and emitting FileEvents.
type Harvester struct {
	Path   string
	Fields map[string]string
	Offset int64

	file     *os.File
	lastRead time.Time
}

func (h *Harvester) Harvest(output chan *FileEvent) {
	if h.Offset > 0 {
		log.Printf("Starting harvester at position %d: %s\n", h.Offset, h.Path)
	} else {
		log.Printf("Starting harvester: %s\n", h.Path)
	}

	h.open()
	info, err := h.file.Stat()
	if err != nil {
		log.Printf("ERROR: unable to stat file %s: %v\n", h.Path, err)
	}
	defer h.file.Close()

	var line uint64 = 0 // Ask registrar about the line number

	// get current offset in file
	h.Offset, err = h.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		log.Printf("ERROR: unable to seek in file %s: %v\n", h.Path, err)
	}

	log.Printf("Current file offset: %d\n", h.Offset)

	// TODO(sissel): Make the buffer size tunable at start-time
	reader := bufio.NewReaderSize(h.file, 16<<10) // 16kb buffer by default

	var read_timeout = 10 * time.Second
	h.lastRead = time.Now()

Tail:
	for {
		text, err := h.readline(reader, read_timeout)

		switch err {
		case io.EOF:
			if h.dead() {
				log.Printf("stopping harvest of %s because it is dead\n", h.Path)
				return
			}
			if err := h.autoRewind(); err != nil {
				log.Printf("stopping harvest of %s: %s", h.Path, err.Error())
			}
			continue Tail
		case nil:
		default:
			log.Printf("Unexpected state reading from %s; error: %s\n", h.Path, err)
			return
		}

		rawTextWidth := int64(len(text))
		text = strings.TrimSpace(text)
		line++

		event := &FileEvent{
			Source:   &h.Path,
			Offset:   h.Offset,
			Line:     line,
			Text:     &text,
			Fields:   &h.Fields,
			fileinfo: &info,
		}

		h.Offset += rawTextWidth

		if text == "" {
			continue
		}

		output <- event
	}
}

func (h *Harvester) dead() bool {
	return time.Since(h.lastRead) > 24*time.Hour
}

func (h *Harvester) autoRewind() error {
	trunc, err := h.truncated()
	if err != nil {
		return fmt.Errorf("unable to autoRewind: %s", err.Error())
	}
	if trunc {
		return h.rewind()
	}
	return nil
}

func (h *Harvester) truncated() (bool, error) {
	info, err := h.file.Stat()
	if err != nil {
		return false, fmt.Errorf("unable to stat file in harvester: %s", err.Error())
	}
	return info.Size() < h.Offset, nil
}

func (h *Harvester) rewind() error {
	_, err := h.file.Seek(0, os.SEEK_SET)
	h.Offset = 0
	return err
}

func (h *Harvester) open() *os.File {
	// Special handling that "-" means to read from standard input
	if h.Path == "-" {
		h.file = os.Stdin
		return h.file
	}

	for {
		var err error
		h.file, err = os.Open(h.Path)

		if err != nil {
			// retry on failure.
			log.Printf("Failed opening %s: %s\n", h.Path, err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	// TODO(sissel): Only seek if the file is a file, not a pipe or socket.
	if h.Offset > 0 {
		h.file.Seek(h.Offset, os.SEEK_SET)
	} else if *from_beginning {
		h.file.Seek(0, os.SEEK_SET)
	} else {
		h.file.Seek(0, os.SEEK_END)
	}

	return h.file
}

func (h *Harvester) readline(reader *bufio.Reader, eof_timeout time.Duration) (string, error) {
	start_time := time.Now()
ReadLines:
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if line != "" {
				h.lastRead = time.Now()
				return line, nil
			}
			time.Sleep(1 * time.Second)
			if time.Since(start_time) > eof_timeout {
				return "", err
			}
			continue ReadLines
		case nil:
		default:
			return "", fmt.Errorf("unable to read line in harvester: %s", err.Error())
		}
		h.lastRead = time.Now()
		return line, nil
	}
}
