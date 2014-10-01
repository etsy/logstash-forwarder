// +build !windows

package main

import (
	"fmt"
	"log"
	"log/syslog"
)

func configureSyslog() {
	writer, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "lumberjack")
	if err != nil {
		shutdown(fmt.Sprintf("Failed to open syslog: %v\n", err))
	}
	log.SetOutput(writer)
}
