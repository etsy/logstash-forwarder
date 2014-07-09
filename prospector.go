package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// finds files in paths/globs to harvest, starts harvesters
func Prospect(fileconfig FileConfig, output chan *FileEvent) {
	// Handle any "-" (stdin) paths
	for i, path := range fileconfig.Paths {
		if path == "-" {
			harvester := Harvester{Path: path, Fields: fileconfig.Fields, out: output}
			go harvester.Harvest()

			// Remove it from the file list
			fileconfig.Paths = append(fileconfig.Paths[:i], fileconfig.Paths[i+1:]...)
		}
	}

	// Use the registrar db to reopen any files at their last positions
	fileinfo := make(map[string]os.FileInfo)
	resume_tracking(fileconfig, fileinfo, output)

	for {
		for _, path := range fileconfig.Paths {
			prospector_scan(path, fileconfig.Fields, fileinfo, output)
		}

		// Defer next scan for a bit.
		time.Sleep(10 * time.Second) // Make this tunable
	}
} /* Prospect */

func resume_tracking(fileconfig FileConfig, fileinfo map[string]os.FileInfo, output chan *FileEvent) {
	var p progress
	if err := p.load(*history_path); err != nil {
		log.Printf("unable to load lumberjack progress file: %s", err.Error())
		return
	}

	for path, state := range p {
		info, err := os.Stat(path)
		if err != nil {
			log.Printf("unable to stat file in resume_tracking: %s", err.Error())
			continue
		}

		if is_file_same(path, info, state) {
			// same file, seek to last known position
			fileinfo[path] = info

			for _, pathglob := range fileconfig.Paths {
				match, err := filepath.Match(pathglob, path)
				if err != nil {
					log.Printf("error matching file path: %s", err.Error())
					continue
				}
				if match {
					log.Printf("resume tracking %s", path)
					harvester := Harvester{
						Path:   path,
						Fields: fileconfig.Fields,
						Offset: state.Offset,
						out:    output,
					}
					go harvester.Harvest()
					break
				}
			}
		}
	}
}

func prospector_scan(path string, fields map[string]string,
	fileinfo map[string]os.FileInfo,
	output chan *FileEvent) {
	// log.Printf("Prospecting %v", path)

	// Evaluate the path as a wildcards/shell glob
	matches, err := filepath.Glob(path)
	if err != nil {
		log.Printf("glob(%s) failed: %v\n", path, err)
		return
	}

	// If the glob matches nothing, use the path itself as a literal.
	if len(matches) == 0 && path == "-" {
		matches = append(matches, path)
	}

	// Check any matched files to see if we need to start a harvester
	for _, file := range matches {
		// Stat the file, following any symlinks.
		info, err := os.Stat(file)
		// TODO(sissel): check err
		if err != nil {
			log.Printf("stat(%s) failed: %s\n", file, err)
			continue
		}

		if info.IsDir() {
			log.Printf("Skipping directory: %s\n", file)
			continue
		}

		// Check the current info against fileinfo[file]
		lastinfo, is_known := fileinfo[file]
		// Track the stat data for this file for later comparison to check for
		// rotation/etc
		fileinfo[file] = info

		// Conditions for starting a new harvester:
		// - file path hasn't been seen before
		// - the file's inode or device changed
		if !is_known {
			// TODO(sissel): Skip files with modification dates older than N
			// TODO(sissel): Make the 'ignore if older than N' tunable
			if time.Since(info.ModTime()) > 24*time.Hour {
				log.Printf("Skipping old file: %s\n", file)
			} else if is_file_renamed(file, info, fileinfo) {
				// Check to see if this file was simply renamed (known inode+dev)
			} else {
				// Most likely a new file. Harvest it!
				log.Printf("Launching harvester on new file: %s\n", file)
				harvester := Harvester{Path: file, Fields: fields, out: output}
				go harvester.Harvest()
			}
		} else if !is_fileinfo_same(lastinfo, info) {
			log.Printf("Launching harvester on rotated file: %s\n", file)
			// TODO(sissel): log 'file rotated' or osmething
			// Start a harvester on the path; a new file appeared with the same name.
			harvester := Harvester{Path: file, Fields: fields, out: output}
			go harvester.Harvest()
		}
	} // for each file matched by the glob
}
