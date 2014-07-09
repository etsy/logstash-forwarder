package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os" // for File and friends
	"path/filepath"
	"strings"
	"time"
)

const (
	h_Rewind = 1 << iota
	h_NoRegister
)

// type Harvester is responsible for tailing a single log file and emitting FileEvents.
type Harvester struct {
	Path   string
	Fields map[string]string
	Offset int64

	moved     bool // this is set when the file has been moved by logrotate
	file      *os.File
	fi        os.FileInfo
	lastRead  time.Time
	lines     chan string
	out       chan *FileEvent
	lineCount uint64 // do we really need this?

	truncLine   string
	truncOffset int64
}

func (h *Harvester) readlines(timeout time.Duration) {
	defer close(h.lines)
	r := bufio.NewReader(h.file)
	var last string
READING:
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			last = line
		}
		fuck := h.Offset
		switch err {
		case io.EOF:
			if line != "" {
				log.Printf("harvester hit EOF in %s with line", h.Path)
				h.lastRead = time.Now()
				h.lines <- line
				time.Sleep(1 * time.Second)
				log.Printf("hit EOF in %s", h.Path)
				continue READING
			}
			if _, err := h.autoRewind(fuck, last); err != nil {
				log.Printf("stopping harvest of %s because of error on autorewind: %s", h.Path, err.Error())
				return
			}
			if time.Since(h.lastRead) > timeout {
				log.Println("harvester timed out")
				return
			}
			time.Sleep(1 * time.Second)
			continue READING
		case nil:
			h.lastRead = time.Now()
			h.lines <- line
		default:
			log.Printf("unable to read line in harvester: %s", err.Error())
			return
		}
	}
}

func (h *Harvester) emit(text string) {
	rawTextWidth := int64(len(text))
	text = strings.TrimSpace(text)
	h.lineCount++

	event := &FileEvent{
		Source:   &h.Path,
		Offset:   h.Offset,
		Line:     h.lineCount,
		Text:     &text,
		Fields:   h.Fields,
		Rotated:  h.moved,
		fileinfo: h.fi,
	}
	if h.moved {
		event.Fields["rotated"] = "true"
	} else {
		event.Fields["rotated"] = "false"
	}

	h.Offset += rawTextWidth

	if text == "" {
		return
	}

	h.out <- event
}

func (h *Harvester) Harvest(opt int) {
	if !(opt&h_NoRegister > 0) {
		registerHarvester(h)
	}
	watchDir(filepath.Dir(h.Path))
	if h.Offset > 0 {
		log.Printf("Starting harvester at position %d: %s\n", h.Offset, h.Path)
	} else {
		log.Printf("Starting harvester: %s\n", h.Path)
	}

	h.open(opt)
	defer h.file.Close()

	h.lineCount = 0

	// get current offset in file
	var err error
	h.Offset, err = h.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		log.Printf("ERROR: unable to seek in file %s: %v\n", h.Path, err)
	}

	log.Printf("Current file offset: %d\n", h.Offset)

	h.lastRead = time.Now()
	h.lines = make(chan string)
	go h.readlines(24 * time.Hour)

	for line := range h.lines {
		h.emit(line)
	}
}

// checks to see if the file has been truncated, and if so, rewinds the file
// handle.
func (h *Harvester) autoRewind(fuck int64, line string) (bool, error) {
	trunc, err := h.truncated()
	if err != nil {
		return false, fmt.Errorf("unable to autoRewind: %s", err.Error())
	}
	if trunc {
		log.Printf("rewind of truncated file: %s", h.Path)
		h.truncOffset = fuck
		h.truncLine = line
		return true, h.rewind()
	}
	return false, nil
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
	if err == nil {
		log.Printf("rewind %s", h.Path)
	}
	return err
}

func (h *Harvester) open(opt int) *os.File {
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
	} else if *from_beginning || opt&h_Rewind > 0 {
		h.file.Seek(0, os.SEEK_SET)
	} else {
		h.file.Seek(0, os.SEEK_END)
	}

	var err error
	h.fi, err = h.file.Stat()
	if err != nil {
		log.Printf("unable to stat file: %s", err.Error())
	}

	return h.file
}

func (h *Harvester) resume(path string) {
	if h.truncLine == "" {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		log.Printf("cannot resume: %s", err.Error())
		return
	}
	if fi.Size() < h.truncOffset {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Printf("cannot resume: %s", err.Error())
		return
	}
	defer f.Close()

	b := make([]byte, len(h.truncLine))
	_, err = f.ReadAt(b, h.truncOffset-int64(len(h.truncLine)))
	if err != nil {
		log.Printf("couldn't read that shit: %s", err.Error())
		return
	}
	// log.Printf("read %d bytes at truncoffset %d", n, h.truncOffset)
	// log.Printf("%s | %s", h.truncLine, b)
	if h.truncLine == string(b) {
		// log.Println("HOLY SHIT")
		newh := Harvester{Path: path, Fields: h.Fields, out: h.out, Offset: h.truncOffset}
		go newh.Harvest(0)
	}
}
