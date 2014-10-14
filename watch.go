package main

import (
	"code.google.com/p/go.exp/inotify"
	"log"
	"regexp"
	"strings"
	"sync"
)

var (
	watcher   *inotify.Watcher
	watchDirs = make(map[string]bool)
	watchLock sync.Mutex

	lr_suffixes = []*regexp.Regexp{
		regexp.MustCompile("\\.\\d+$"), // numeric suffix (default sufix)
		regexp.MustCompile("-\\d{8}$"), // dateext option
	}
)

// strips a path name of logrotate suffixes.
func lrStrip(path string) (string, bool) {
	for _, p := range lr_suffixes {
		if indices := p.FindStringIndex(path); indices != nil {
			return path[:indices[0]], true
		}
	}
	return path, false
}

func copyTruncate(fullPath string) {
	path, ok := lrStrip(fullPath)
	if !ok {
		return
	}
	h := registry.byPath(path)
	if h != nil {
		h.nextPath = fullPath
	}
}

func reportFSEvents() {
	defer func() {
		log.Println("reportFSEvents ending")
	}()
	cookies := make(map[uint32]*inotify.Event, 4)

	for {
		select {
		case ev := <-watcher.Event:
			switch {
			case ev.Mask&inotify.IN_MOVE > 0:
				prev, ok := cookies[ev.Cookie]
				if !ok {
					cookies[ev.Cookie] = ev
					break
				}

				if strings.Contains(prev.Name, "logrotate_temp") {
					copyTruncate(ev.Name)
				} else {
					registry.rename(prev.Name, ev.Name)
				}
				delete(cookies, ev.Cookie)

			case ev.Mask&inotify.IN_DELETE > 0:
			case ev.Mask&inotify.IN_CREATE > 0:
			default:
				log.Printf("unknown: %v (%v)", ev, ev.Cookie)
			}
		case err := <-watcher.Error:
			log.Printf("watcher saw error: %v", err)
		}
	}
}

func watchDir(path string) {
	if !watchDirs[path] {
		flags := inotify.IN_CREATE | inotify.IN_DELETE | inotify.IN_MOVE
		if err := watcher.AddWatch(path, flags); err != nil {
			log.Printf("unable to watch directory: %s", err.Error())
		} else {
			watchDirs[path] = true
		}
	}
}

func init() {
	var err error
	watcher, err = inotify.NewWatcher()
	if err != nil {
		log.Printf("unable to start watcher: %s", err.Error())
		return
	}
}
