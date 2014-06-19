// +build !windows

package main

import (
	"encoding/json"
	"log"
	"os"
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
	// Open tmp file, write, flush, rename
	file, err := os.Create(".lumberjack.new")
	if err != nil {
		log.Printf("Failed to open .lumberjack.new for writing: %s\n", err)
		return
	}
	defer file.Close()

	existingState, err := loadRegistry(path)
	if err != nil {
		log.Printf("Failed to read existing state at path %s: %s", path, err.Error())
		existingState = make(map[string]*FileState)
	}

	for name, fs := range state {
		existingState[name] = fs
	}

	if err := json.NewEncoder(file).Encode(existingState); err != nil {
		log.Printf("Failed to write log state to file: %v", err)
		return
	}
	if err := os.Rename(".lumberjack.new", path); err != nil {
		log.Printf("Failed to move log state file: %v", err)
	}
}
