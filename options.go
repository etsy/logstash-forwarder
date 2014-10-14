package main

import (
	"flag"
	"time"
)

var options struct {
	CPUProfile    string
	SpoolSize     uint64
	NumWorkers    int
	IdleTimeout   time.Duration
	ConfigFile    string
	LogFile       string
	PidFile       string
	UseSyslog     bool
	FromBeginning bool
	HistoryPath   string
	TempDir       string
	NumThreads    int
	CmdPort       int
	HttpPort      string
}

func init() {
	flag.StringVar(&options.CPUProfile, "cpuprofile", "", "write cpu profile to file")
	flag.Uint64Var(&options.SpoolSize, "spool-size", 1024,
		"Maximum number of events to spool before a flush is forced.")
	flag.IntVar(&options.NumWorkers, "num-workers", 1,
		"deprecated option, strictly for backwards compatibility. does nothing.")
	flag.DurationVar(&options.IdleTimeout, "idle-flush-time", 5*time.Second,
		"Maximum time to wait for a full spool before flushing anyway")
	flag.StringVar(&options.ConfigFile, "config", "", "The config file to load")
	flag.StringVar(&options.LogFile, "log-file", "", "Log file output")
	flag.StringVar(&options.PidFile, "pid-file", "lumberjack.pid",
		"destination to which a pidfile will be written")
	flag.BoolVar(&options.UseSyslog, "log-to-syslog", false,
		"Log to syslog instead of stdout. This option overrides the --log-file option.")
	flag.BoolVar(&options.FromBeginning, "from-beginning", false,
		"Read new files from the beginning, instead of the end")
	flag.StringVar(&options.HistoryPath, "progress-file", ".lumberjack",
		"path of file used to store progress data")
	flag.StringVar(&options.TempDir, "temp-dir", "/tmp",
		"directory for creating temp files")
	flag.IntVar(&options.NumThreads, "threads", 1, "Number of OS threads to use")
	flag.IntVar(&options.CmdPort, "cmd-port", 42586, "tcp command port number")
	flag.StringVar(&options.HttpPort, "http", "",
		"http port for debug info. No http server is run if this is left off. E.g.: http=:6060")
}
