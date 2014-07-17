package main

import (
	"code.google.com/p/go.exp/fsnotify"
	"log"
	"regexp"
	"strings"
	"sync"
)

var (
	watcher   *fsnotify.Watcher
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

func reportFSEvents() {
	for {
		select {
		case ev := <-watcher.Event:
			switch {
			case ev.IsRename():
				// is this useful?
				if h := registry.byPath(ev.Name); h != nil {
					h.moved = true
				}
			case ev.IsDelete():

			case ev.IsCreate():
				if h := registry.byPathStat(ev.Name); h != nil {
					registry.rename(h.Path, ev.Name)
					break
				}

				if strings.Contains(ev.Name, "logrotate_temp") {
					break
				}

				path, ok := lrStrip(ev.Name)
				if !ok {
					break
				}

				if h := registry.byPath(path); h != nil {
					h.nextPath = ev.Name
				}
			}
		case err := <-watcher.Error:
			log.Printf("watcher saw error: %v", err)
		}
	}
}

func watchDir(path string) {
	if !watchDirs[path] {
		if err := watcher.Watch(path); err != nil {
			log.Printf("unable to watch directory: %s", err.Error())
		} else {
			watchDirs[path] = true
		}
	}
}

func init() {
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Printf("unable to start watcher: %s", err.Error())
		return
	} else {
		log.Println("watcher is waiting")
	}
}
