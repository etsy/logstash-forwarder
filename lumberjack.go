package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var (
	cpuprofile     = flag.String("cpuprofile", "", "write cpu profile to file")
	spool_size     = flag.Uint64("spool-size", 1024, "Maximum number of events to spool before a flush is forced.")
	num_workers    = flag.Int("num-workers", runtime.NumCPU()*2, "Number of concurrent publish workers. Defaults to 2*CPU")
	idle_timeout   = flag.Duration("idle-flush-time", 5*time.Second, "Maximum time to wait for a full spool before flushing anyway")
	config_file    = flag.String("config", "", "The config file to load")
	use_syslog     = flag.Bool("log-to-syslog", false, "Log to syslog instead of stdout")
	from_beginning = flag.Bool("from-beginning", false, "Read new files from the beginning, instead of the end")
)

func main() {
	flag.Parse()

	startCPUProfile()

	config, err := LoadConfig(*config_file)
	if err != nil {
		log.Fatal(err.Error())
	}

	event_chan := make(chan *FileEvent, 16)
	publisher_chan := make(chan []*FileEvent, 1)
	registrar_chan := make(chan []*FileEvent, 1)

	if len(config.Files) == 0 {
		log.Fatalf("No paths given. What files do you want me to watch?\n")
	}

	// The basic model of execution:
	// - prospector: finds files in paths/globs to harvest, starts harvesters
	// - harvester: reads a file, sends events to the spooler
	// - spooler: buffers events until ready to flush to the publisher
	// - publisher: writes to the network, notifies registrar
	// - registrar: records positions of files read
	// Finally, prospector uses the registrar information, on restart, to
	// determine where in each file to resume a harvester.

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	if *use_syslog {
		configureSyslog()
	}

	// Prospect the globs/paths given on the command line and launch harvesters
	for _, fileconfig := range config.Files {
		go Prospect(fileconfig, event_chan)
	}

	// Harvesters dump events into the spooler.
	go Spool(event_chan, publisher_chan, *spool_size, *idle_timeout)

	if *num_workers <= 0 {
		*num_workers = runtime.NumCPU() * 2
	}
	for i := 0; i < *num_workers; i++ {
		log.Printf("adding publish worker")
		go Publishv1(publisher_chan, registrar_chan, &config.Network)
	}

	// registrar records last acknowledged positions in all files.
	Registrar(registrar_chan)
}

func startCPUProfile() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		go func() {
			time.Sleep(60 * time.Second)
			pprof.StopCPUProfile()
			panic("done")
		}()
	}
}
