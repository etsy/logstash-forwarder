package main

import (
	"code.google.com/p/go.exp/fsnotify"
	"log"
	"regexp"
	"sync"
)

var (
	watcher    *fsnotify.Watcher
	watchDirs  = make(map[string]bool)
	watchLock  sync.Mutex
	harvesters = make(map[string]*Harvester)

	lr_suffixes = []*regexp.Regexp{
		regexp.MustCompile("\\.\\d+$"), // numeric suffix (default sufix)
		regexp.MustCompile("-\\d{8}$"), // dateext option
	}
)

func registerHarvester(h *Harvester) {
	watchLock.Lock()
	defer watchLock.Unlock()
	harvesters[h.Path] = h
}

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
			// if !ev.IsModify() {
			// 	log.Println(ev)
			// }
			switch {
			case ev.IsRename(), ev.IsDelete():
				if h, ok := harvesters[ev.Name]; ok {
					h.moved = true
					delete(harvesters, ev.Name)
				}
			case ev.IsCreate():
				path, ok := lrStrip(ev.Name)
				if !ok {
					break
				}
				if h, ok := harvesters[path]; ok {
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
