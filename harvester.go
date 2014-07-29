package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	h_Rewind = 1 << iota
	h_NoRegister
	h_StartAtEnd
)

// harvester file handle status
type hfStatus int

const (
	hf_Err hfStatus = iota
	hf_Ok
	hf_Trunc
	hf_Gone
)

// type Harvester is responsible for tailing a single log file and emitting
// FileEvents.  A harvester never changes which inode it points to.
type Harvester struct {
	Path   string
	Fields map[string]string
	join   *joinspec

	moved      bool // this is set when the file has been moved by logrotate
	file       *os.File
	fi         os.FileInfo
	lastRead   time.Time
	out        chan *FileEvent
	lastLine   string
	lastOffset int64

	nextPath string
}

func (h *Harvester) MarshalJSON() ([]byte, error) {
	type t struct {
		Path   string            `json:"path"`
		Id     fileId            `json:"id"`
		Fields map[string]string `json:"fields"`
	}
	id, err := h.fileId()
	if err != nil {
		return nil, err
	}
	v := t{
		Path:   h.Path,
		Id:     id,
		Fields: h.Fields,
	}
	return json.Marshal(v)
}

// readlines reads lines from the harvester's existing file handle.  readlines
// does not open or seek a file on its own.
func (h *Harvester) readlines(timeout time.Duration) {
	if err := registry.register(h); err != nil {
		log.Printf("readlines unable to register: %v", err)
		return
	}
	defer registry.unregister(h)

	r := bufio.NewReader(h.file)
	var last string

	offset, err := h.fileOffset()
	if err != nil {
		log.Printf("unable to read file offset in readlines: %v", err)
		return
	}

	for {
		h.lastRead = time.Now()
		line, err := r.ReadString('\n')
		if line != "" {
			last = line
		}
		switch err {
		case io.EOF:
			if line != "" {
				log.Printf("harvester hit EOF in %s with line", h.Path)
				h.emit(line, offset)
				time.Sleep(1 * time.Second)
				break
			}
			if rewound, err := h.autoRewind(offset, last); err != nil {
				log.Printf("harvester for file %s stopping: %v", h.Path, err)
				return
			} else if rewound {
				offset = 0
			}
			if time.Since(h.lastRead) > timeout {
				log.Printf("harvester timed out: %s", h.Path)
				return
			}
			time.Sleep(1 * time.Second)
		case nil:
			h.emit(line, offset)
		default:
			log.Printf("unable to read line in harvester: %v", err)
			return
		}
		offset += int64(len(line))
	}
}

// the event method takes a line of text found at a byte offset in the
// harvester's current file and wraps it in a *FileEvent object, adding some
// file-level context to the FileEvent.
func (h *Harvester) event(text string, offset int64) *FileEvent {
	e := &FileEvent{
		Source:   h.Path,
		Offset:   offset,
		Text:     strings.TrimSpace(text),
		Fields:   h.Fields,
		Rotated:  h.moved,
		fileinfo: h.fi,
	}
	if h.moved {
		e.Fields["rotated"] = "true"
	} else {
		e.Fields["rotated"] = "false"
	}
	return e
}

func (h *Harvester) emit(text string, offset int64) {
	if h.join == nil {
		h.out <- h.event(text, offset)
		return
	}
	if h.join.with == "previous" {
		if h.join.match != nil {
			if h.join.match.MatchString(text) {
				h.lastLine += text
				return
			}
		}
		if h.join.not != nil {
			if !h.join.not.MatchString(text) {
				h.lastLine += text
				return
			}
		}
	}

	if h.lastLine != "" {
		h.out <- h.event(h.lastLine, h.lastOffset)
	}
	h.lastLine = text
	h.lastOffset = offset
}

func (h *Harvester) fileOffset() (int64, error) {
	return h.file.Seek(0, os.SEEK_CUR)
}

func (h *Harvester) fileId() (fileId, error) {
	if h.fi == nil {
		return "", fmt.Errorf("harvester has no file handle")
	}
	return filestring(h.fi), nil
}

func (h *Harvester) Harvest(offset int64, opt int) {
	defer log.Printf("harvester done reading file %s", h.Path)
	watchDir(filepath.Dir(h.Path))
	log.Printf("Starting harvester: %s\n", h.Path)

	h.open(offset, opt)
	defer h.file.Close()

	h.readlines(24 * time.Hour)
}

func (h *Harvester) resume(offset int64, line string) {
	defer log.Printf("harvester done reading file %s", h.Path)
	log.Printf("trying to resume %s at offset %d", h.Path, offset)
	if h.Path == "-" {
		log.Printf("illegal attempt to resume stdin at offset %d", offset)
		return
	}

	h.open(offset, 0)
	defer h.file.Close()

	b := make([]byte, len(line))
	_, err := h.file.ReadAt(b, offset-int64(len(line)))
	if err != nil {
		log.Printf("couldn't read resume line: %v", err)
		return
	}
	if line == string(b) {
		h.readlines(24 * time.Hour)
	}
}

// checks to see if the file has been truncated, and if so, rewinds the file
// handle.
func (h *Harvester) autoRewind(offset int64, line string) (bool, error) {
	s, err := h.status(offset)
	switch s {
	case hf_Err:
		return false, fmt.Errorf("unable to autoRewind: %v", err)
	case hf_Ok:
		return false, nil
	case hf_Trunc:
		if h.nextPath != "" {
			newh := Harvester{Path: h.nextPath, Fields: h.Fields, out: h.out}
			go newh.resume(offset, line)
			h.nextPath = ""
		}
		return true, h.rewind()
	case hf_Gone:
		return false, fmt.Errorf("file is gone: %s", h.Path)
	default:
		return false, fmt.Errorf("unknown harvester file status: %v", s)
	}
}

func (h *Harvester) status(offset int64) (hfStatus, error) {
	info, err := h.file.Stat()
	if err != nil {
		return hf_Err, fmt.Errorf("unable to stat file in harvester: %s", err.Error())
	}
	if info.Sys() != nil {
		raw, ok := info.Sys().(*syscall.Stat_t)
		if ok && raw.Nlink == 0 {
			if info.Size() > offset {
				log.Printf("deleted file has more data.  size: %d, our offset: %d", info.Size(), offset)
				return hf_Ok, nil
			}
			return hf_Gone, nil
		}
	}
	if info.Size() < offset {
		log.Printf("file %s is at offset %d but size is %d", h.Path, offset, info.Size())
		return hf_Trunc, nil
	}
	return hf_Ok, nil
}

func (h *Harvester) rewind() error {
	_, err := h.file.Seek(0, os.SEEK_SET)
	if err == nil {
		log.Printf("rewind %s", h.Path)
	}
	return err
}

func (h *Harvester) open(offset int64, opt int) *os.File {
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
			log.Printf("Failed opening stupid file %s: %s\n", h.Path, err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	// TODO(sissel): Only seek if the file is a file, not a pipe or socket.
	if offset > 0 {
		h.file.Seek(offset, os.SEEK_SET)
		log.Printf("reading from %d: %s", offset, h.Path)
	} else if *from_beginning || opt&h_Rewind > 0 {
		h.file.Seek(0, os.SEEK_SET)
		log.Printf("reading from beginning: %s", h.Path)
	} else {
		h.file.Seek(0, os.SEEK_END)
		log.Printf("reading from end: %s", h.Path)
	}

	var err error
	h.fi, err = h.file.Stat()
	if err != nil {
		log.Printf("unable to stat file: %s", err.Error())
	}

	return h.file
}
