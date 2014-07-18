package main

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type fileId string

type hregistry struct {
	sync.RWMutex
	runningIds   map[fileId]*Harvester
	runningPaths map[string]*Harvester
	paths        map[string]bool
}

func newRegistry(conf *Config) *hregistry {
	r := &hregistry{
		runningIds:   make(map[fileId]*Harvester, len(conf.Files)),
		runningPaths: make(map[string]*Harvester, len(conf.Files)),
		paths:        make(map[string]bool, len(conf.Files)),
	}
	for _, f := range conf.Files {
		for _, path := range f.Paths {
			r.paths[path] = true
		}
	}
	return r
}

func (r hregistry) register(v *Harvester) error {
	r.Lock()
	defer r.Unlock()

	id, err := v.fileId()
	if err != nil {
		return fmt.Errorf("unable to register harvester: %v", err)
	}

	if _, ok := r.runningIds[id]; ok {
		return fmt.Errorf("file is already being harvested: %v", v)
	}

	if _, ok := r.runningPaths[v.Path]; ok {
		return fmt.Errorf("path is already being harvested: %v", v)

	}
	r.runningIds[id] = v
	r.runningPaths[v.Path] = v

	log.Printf("registrary registered: %v", v)
	return nil
}

func (r hregistry) unregister(v *Harvester) error {
	r.Lock()
	defer r.Unlock()

	id, err := v.fileId()
	if err != nil {
		return fmt.Errorf("unable to unregister harvester: %v", err)
	}

	if _, ok := r.runningIds[id]; !ok {
		return fmt.Errorf("unable to unregister harvester: id %s wasn't registered", id)
	}

	if _, ok := r.runningPaths[v.Path]; !ok {
		return fmt.Errorf("unable to unregister harvester: path %s wasn't registered", v.Path)
	}
	delete(r.runningIds, id)
	delete(r.runningPaths, v.Path)

	log.Printf("registrar unregistered: %v", v)
	return nil
}

func (r hregistry) byPath(path string) *Harvester {
	r.RLock()
	defer r.RUnlock()

	return r.runningPaths[path]
}

func (r hregistry) byPathStat(path string) *Harvester {
	fi, err := os.Stat(path)
	if err != nil {
		log.Printf("registry can't stat file: %v", err)
		return nil
	}
	return r.byId(filestring(fi))
}

func (r hregistry) byId(id fileId) *Harvester {
	r.RLock()
	defer r.RUnlock()

	return r.runningIds[id]
}

func (r hregistry) rename(prev, curr string) {
	r.Lock()
	defer r.Unlock()

	h, ok := r.runningPaths[prev]
	if !ok {
		// log.Printf("registry didn't have a record for any harvester at %s", prev)
		return
	}

	if h.Path != prev {
		log.Printf("registry rename failed sanity check: harvester's prev path %s does not match expected path %s", h.Path, prev)
		return
	}

	log.Printf("file renamed: %s -> %s", prev, curr)
	h.Path = curr
	h.moved = true
	r.runningPaths[curr] = h
	delete(r.runningPaths, prev)
}
