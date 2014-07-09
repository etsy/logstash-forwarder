package main

import (
	"code.google.com/p/go.exp/fsnotify"
	"log"
	"sync"
)

var (
	watcher    *fsnotify.Watcher
	watchDirs  = make(map[string]bool)
	watchLock  sync.Mutex
	harvesters = make(map[string]*Harvester)
)

func registerHarvester(h *Harvester) {
	watchLock.Lock()
	defer watchLock.Unlock()
	harvesters[h.Path] = h
}

func reportFSEvents() {
	for {
		select {
		case ev := <-watcher.Event:
			switch {
			case ev.IsRename(), ev.IsDelete():
				// if ev.IsDelete() {
				//     log.Println(ev)
				// }
				if h, ok := harvesters[ev.Name]; ok {
					// log.Println(ev)
					h.moved = true
					delete(harvesters, ev.Name)
				}
			case ev.IsCreate():
				if _, ok := harvesters[ev.Name]; ok {
					// log.Println(ev)
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
