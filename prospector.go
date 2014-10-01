package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// finds files in paths/globs to harvest, starts harvesters
func Prospect(fileconfig FileConfig, netconf NetworkConfig) {
	out := netconf.EventChan(fileconfig.Dest)
	if out == nil {
		log.Printf("ERROR unable to start prospector for %v: no event channel", fileconfig.Paths)
		return
	}

	// Handle any "-" (stdin) paths
	for i, path := range fileconfig.Paths {
		if path == "-" {
			harvester := Harvester{
				Path:   path,
				Fields: fileconfig.Fields,
				join:   fileconfig.Join,
				out:    out,
			}
			go harvester.Harvest(0, 0)

			// Remove it from the file list
			fileconfig.Paths = append(fileconfig.Paths[:i], fileconfig.Paths[i+1:]...)
		}
	}

	// Use the registrar db to reopen any files at their last positions
	fileinfo := make(map[string]os.FileInfo)
	resume_tracking(fileconfig, fileinfo, out)

	for {
		for _, path := range fileconfig.Paths {
			prospector_scan(path, &fileconfig, fileinfo, out)
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
						join:   fileconfig.Join,
						out:    output,
					}
					go harvester.Harvest(state.Offset, 0)
					break
				}
			}
		}
	}
}

func prospector_scan(path string, conf *FileConfig,
	fileinfo map[string]os.FileInfo,
	output chan *FileEvent) {

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
		info, err := os.Stat(file)
		if err != nil {
			log.Printf("prospector unable to stat file %s: %s\n", file, err)
			continue
		}

		if info.IsDir() {
			log.Printf("prospector skipping directory: %s\n", file)
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
				log.Printf("skipping old file: %s\n", file)
			} else if is_file_renamed(file, info, fileinfo) {
				// Check to see if this file was simply renamed (known inode+dev)
			} else {
				log.Printf("harvest new file: %s\n", file)
				harvester := Harvester{
					Path:   file,
					Fields: conf.Fields,
					join:   conf.Join,
					out:    output,
				}
				go harvester.Harvest(0, 0)
			}
		} else if !is_fileinfo_same(lastinfo, info) {
			log.Printf("harvest rotated file: %s\n", file)
			harvester := Harvester{
				Path:   file,
				Fields: conf.Fields,
				join:   conf.Join,
				out:    output,
			}
			go harvester.Harvest(0, h_Rewind)
		}
	} // for each file matched by the glob
}
