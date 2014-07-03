package main

import (
	"encoding/json"
	"testing"
)

var source = []byte(`
{
    "paths": [
        "/var/log/fail2ban.log"
    ],
    "fields": {
        "type": "fail2ban"
    }
}
`)

func TestJSONLoading(t *testing.T) {
	var f FileConfig
	if err := json.Unmarshal(source, &f); err != nil {
		t.Fatalf("json.Unmarshal failed")
	}
	if len(f.Paths) != 1 {
		t.FailNow()
	}
	if f.Paths[0] != "/var/log/fail2ban.log" {
		t.FailNow()
	}
	if f.Fields["type"] != "fail2ban" {
		t.FailNow()
	}
}
