package main

import (
	"bytes"
	"encoding/json"
	"expvar"
	"fmt"
	"log"
	"os"
	"sync"
)

type fileId string

type hregistry struct {
	sync.RWMutex
	RunningIds   map[fileId]*Harvester `json:"by_id"`
	RunningPaths map[string]*Harvester `json:"by_path"`
	paths        map[string]bool
}

func newRegistry(conf *Config) *hregistry {
	r := &hregistry{
		RunningIds:   make(map[fileId]*Harvester, len(conf.Files)),
		RunningPaths: make(map[string]*Harvester, len(conf.Files)),
		paths:        make(map[string]bool, len(conf.Files)),
	}
	for _, f := range conf.Files {
		for _, path := range f.Paths {
			r.paths[path] = true
		}
	}
	expvar.Publish("tailing", r)
	return r
}

func (r hregistry) String() string {
	r.RLock()
	defer r.RUnlock()

	harvesters := make([]*Harvester, 0, len(r.RunningIds))
	for _, h := range r.RunningIds {
		harvesters = append(harvesters, h)
	}

	var buf bytes.Buffer

	json.NewEncoder(&buf).Encode(harvesters)

	return buf.String()
}

func (r hregistry) register(v *Harvester) error {
	r.Lock()
	defer r.Unlock()

	id, err := v.fileId()
	if err != nil {
		return fmt.Errorf("unable to register harvester: %v", err)
	}

	if _, ok := r.RunningIds[id]; ok {
		return fmt.Errorf("file is already being harvested: %v", v)
	}

	if _, ok := r.RunningPaths[v.Path]; ok {
		return fmt.Errorf("path is already being harvested: %v", v)

	}
	r.RunningIds[id] = v
	r.RunningPaths[v.Path] = v

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

	if _, ok := r.RunningIds[id]; !ok {
		return fmt.Errorf("unable to unregister harvester: id %s wasn't registered", id)
	}

	if _, ok := r.RunningPaths[v.Path]; !ok {
		return fmt.Errorf("unable to unregister harvester: path %s wasn't registered", v.Path)
	}
	delete(r.RunningIds, id)
	delete(r.RunningPaths, v.Path)

	log.Printf("registrar unregistered: %v", v)
	return nil
}

func (r hregistry) byPath(path string) *Harvester {
	r.RLock()
	defer r.RUnlock()

	return r.RunningPaths[path]
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

	return r.RunningIds[id]
}

func (r hregistry) rename(prev, curr string) {
	r.Lock()
	defer r.Unlock()

	h, ok := r.RunningPaths[prev]
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
	r.RunningPaths[curr] = h
	delete(r.RunningPaths, prev)
}
