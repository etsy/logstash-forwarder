package main

import ()

type fileId string

type hregistry struct {
	harvesting    map[string]*Harvester
	shouldHarvest map[string]bool
}

func (h hregistry) register(v *Harvester) {

}

func (h hregistry) unregister(v *Harvester) {

}

func (h hregistry) getByPath(path string) {

}
