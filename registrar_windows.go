package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func (p *progress) writeFile(path string) error {
	tmp := path + ".new"
	file, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("unable to write progress to file: %s", err.Error())
	}

	encoder := json.NewEncoder(file)
	encoder.Encode(state)
	file.Close()

	old := path + ".old"
	os.Rename(path, old)
	os.Rename(tmp, path)
}
