// +build !windows

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func loadRegistry(fname string) (map[string]*FileState, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var existingState map[string]*FileState
	if err := json.NewDecoder(f).Decode(&existingState); err != nil {
		return nil, err
	}
	return existingState, nil
}

func WriteRegistry(state map[string]*FileState, path string) {
	f, err := ioutil.TempFile(*temp_dir, "lumberjack")
	if err != nil {
		log.Printf("failed to create temp file for writing: %s\n", err)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Printf("unable to stat temp file: %s", err.Error())
		return
	}

	existingState, err := loadRegistry(path)
	if err != nil {
		log.Printf("Failed to read existing state at path %s: %s", path, err.Error())
		existingState = make(map[string]*FileState)
	}

	for name, fs := range state {
		existingState[name] = fs
	}

	if err := json.NewEncoder(f).Encode(existingState); err != nil {
		log.Printf("Failed to write log state to file: %v", err)
		return
	}
	if err := os.Rename(filepath.Join(*temp_dir, fi.Name()), path); err != nil {
		log.Printf("Failed to move log state file: %v", err)
	}
}
