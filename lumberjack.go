// The basic model of execution:
// - prospector: finds files in paths/globs to harvest, starts harvesters
// - harvester: reads a file, sends events to the spooler
// - spooler: buffers events until ready to flush to the publisher
// - publisher: writes to the network, notifies registrar
// - registrar: records positions of files read
// Finally, prospector uses the registrar information, on restart, to
// determine where in each file to resume a harvester.
package main

import (
	_ "expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"time"
)

var (
	default_workers = runtime.NumCPU() * 2
	cpuprofile      = flag.String("cpuprofile", "", "write cpu profile to file")
	spool_size      = flag.Uint64("spool-size", 1024, "Maximum number of events to spool before a flush is forced.")
	num_workers     = flag.Int("num-workers", default_workers, "Number of concurrent publish workers. Defaults to 2*CPU")
	idle_timeout    = flag.Duration("idle-flush-time", 5*time.Second, "Maximum time to wait for a full spool before flushing anyway")
	config_file     = flag.String("config", "", "The config file to load")
	use_syslog      = flag.Bool("log-to-syslog", false, "Log to syslog instead of stdout")
	from_beginning  = flag.Bool("from-beginning", false, "Read new files from the beginning, instead of the end")
	history_path    = flag.String("progress-file", ".lumberjack", "path of file used to store progress data")
	temp_dir        = flag.String("temp-dir", "/tmp", "directory for creating temp files")
	num_threads     = flag.Int("threads", 1, "Number of OS threads to use")
	cmd_port        = flag.Int("cmd-port", 42586, "tcp command port number")
	http_port       = flag.String("http", "", "http port for debug info. No http server is run if this is left off. E.g.: http=:6060")

	event_chan chan *FileEvent
	registry   *hregistry
)

func awaitSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
	log.Println("lumberjack shutting down")
}

func setupLogging() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	if *use_syslog {
		configureSyslog()
	}
}

func shutdown(v interface{}) {
	log.Fatal(v)
}

func startPublishers(conf *NetworkConfig, in, out chan eventPage) error {
	tlsConfig, err := conf.TLS()
	if err != nil {
		return fmt.Errorf("unable to start publishers: %v", err)
	}

	for i, server := range conf.Servers {
		p := &Publisher{
			id:        i,
			sequence:  1,
			addr:      server,
			tlsConfig: *tlsConfig,
			timeout:   conf.timeout,
		}
		go p.publish(in, out)
	}
	return nil
}

func startHttp() {
	if *http_port != "" {
		log.Printf("starting http debug port on %s", *http_port)
		if err := http.ListenAndServe(*http_port, nil); err != nil {
			log.Printf("unable to open http port: %v", err)
		}
	} else {
		log.Println("no http port specified")
	}
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*num_threads)
	setupLogging()
	go cmdListener()
	log.Println("lumberjack starting")

	startCPUProfile()

	config, err := LoadConfig(*config_file)
	if err != nil {
		log.Fatal(err.Error())
	}
	registry = newRegistry(config)

	event_chan = make(chan *FileEvent, 16)
	publisher_chan := make(chan eventPage, len(config.Network.Servers))
	registrar_chan := make(chan eventPage, 1)

	if len(config.Files) == 0 {
		log.Fatalf("No paths given. What files do you want me to watch?\n")
	}

	go reportFSEvents()
	// Prospect the globs/paths given on the command line and launch harvesters
	for _, fileconfig := range config.Files {
		go Prospect(fileconfig, event_chan)
	}

	// Harvesters dump events into the spooler.
	go Spool(event_chan, publisher_chan, *spool_size, *idle_timeout)

	if err := startPublishers(&config.Network, publisher_chan, registrar_chan); err != nil {
		shutdown(err)
	}

	// registrar records last acknowledged positions in all files.
	go Registrar(registrar_chan)
	go startHttp()
	awaitSignals()
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
