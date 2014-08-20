package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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

func (n *NetworkConfig) TLS() (*tls.Config, error) {
	var c tls.Config
	c.InsecureSkipVerify = true
	if n.SSLCertificate != "" && n.SSLKey != "" {
		cert, err := tls.LoadX509KeyPair(n.SSLCertificate, n.SSLKey)
		if err != nil {
			log.Printf("cert: %s, key: %s", n.SSLCertificate, n.SSLKey)
			return nil, fmt.Errorf("unable to load x509 keypair: %v", err)
		}
		c.Certificates = []tls.Certificate{cert}
	}
	c.RootCAs = x509.NewCertPool()
	if n.SSLCA != "" {
		raw, err := ioutil.ReadFile(n.SSLCA)
		if err != nil {
			return nil, fmt.Errorf("unable to read CA from file: %v", err)
		}
		if !c.RootCAs.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("illegal x509 CA")
		}
	}
	return &c, nil
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
