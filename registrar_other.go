// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func (p *progress) writeFile(path string) error {
	f, err := ioutil.TempFile(*temp_dir, "lumberjack")
	if err != nil {
		return fmt.Errorf("failed to create temp file for writing: %s\n", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("unable to stat temp file: %s", err.Error())
	}

	var existing progress
	if err := existing.load(path); err != nil {
		return fmt.Errorf("failed to read existing state at path %s: %s", path, err.Error())
	}

	for name, fs := range *p {
		existing[name] = fs
	}

	if err := json.NewEncoder(f).Encode(existing); err != nil {
		return fmt.Errorf("failed to write log state to file: %v", err)
	}
	if err := os.Rename(filepath.Join(*temp_dir, fi.Name()), path); err != nil {
		return fmt.Errorf("failed to move log state file: %v", err)
	}
	return nil
}
