package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type progress map[string]*FileState

func (p *progress) load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to open progress file: %s", err.Error())
	}
	defer f.Close()

	*p = make(progress, 32)
	if err := json.NewDecoder(f).Decode(p); err != nil {
		return fmt.Errorf("unable to decode progress file: %s", err.Error())
	}
	return nil
}

// records positions of files read
func Registrar(input chan eventPage) {
	for page := range input {
		if page.empty() {
			continue
		}

		p := page.progress()

		log.Printf("registrar received %d events. %s", len(page), page.countString())

		if err := p.writeFile(options.HistoryPath); err != nil {
			log.Printf("unable to write history to file: %s", err.Error())
		}
	}
}
