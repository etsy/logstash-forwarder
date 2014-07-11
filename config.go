package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Network NetworkConfig `json:network`
	Files   []FileConfig  `json:files`
}

func (c *Config) UnmarshalJSON(b []byte) error {
	type t Config
	var raw t
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	*c = Config(raw)
	if c.Network.Timeout == 0 {
		c.Network.Timeout = 15
	}
	c.Network.timeout = time.Duration(c.Network.Timeout) * time.Second
	return nil
}

type NetworkConfig struct {
	Servers        []string `json:servers`
	SSLCertificate string   `json:"ssl certificate"`
	SSLKey         string   `json:"ssl key"`
	SSLCA          string   `json:"ssl ca"`
	Timeout        int64    `json:timeout`
	timeout        time.Duration
}

type FileConfig struct {
	Paths  []string          `json:paths`
	Fields map[string]string `json:fields`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file '%s': %s\n", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat config file '%s': %s\n", path, err)
	}
	if fi.Size() > (10 << 20) {
		return nil, fmt.Errorf("Config file too large? Aborting, just in case. '%s' is %d bytes\n",
			path, fi)
	}

	var conf Config
	if err := json.NewDecoder(f).Decode(&conf); err != nil {
		return nil, fmt.Errorf("failed unmarshalling config json: %s\n", err)
	}
	return &conf, nil
}
