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
	"syscall"
	"time"
)

var (
	default_workers = runtime.NumCPU() * 2
	cpuprofile      = flag.String("cpuprofile", "", "write cpu profile to file")
	spool_size      = flag.Uint64("spool-size", 1024, "Maximum number of events to spool before a flush is forced.")
	num_workers     = flag.Int("num-workers", default_workers, "Number of concurrent publish workers. Defaults to 2*CPU")
	idle_timeout    = flag.Duration("idle-flush-time", 5*time.Second, "Maximum time to wait for a full spool before flushing anyway")
	config_file     = flag.String("config", "", "The config file to load")
	log_file_path   = flag.String("log-file", "", "Log file output")
	pid_file_path   = flag.String("pid-file", "lumberjack.pid", "destination to which a pidfile will be written")
	use_syslog      = flag.Bool("log-to-syslog", false, "Log to syslog instead of stdout. This option overrides the --log-file option.")
	from_beginning  = flag.Bool("from-beginning", false, "Read new files from the beginning, instead of the end")
	history_path    = flag.String("progress-file", ".lumberjack", "path of file used to store progress data")
	temp_dir        = flag.String("temp-dir", "/tmp", "directory for creating temp files")
	num_threads     = flag.Int("threads", 1, "Number of OS threads to use")
	cmd_port        = flag.Int("cmd-port", 42586, "tcp command port number")
	http_port       = flag.String("http", "", "http port for debug info. No http server is run if this is left off. E.g.: http=:6060")

	event_chan       chan *FileEvent
	registry         *hregistry
	shutdownHandlers []func()
	log_file_handle  *os.File
)

// creates a file and writes the current process's pid into that file.  The
// file name is specified on the command line.
func writePid() {
	if *pid_file_path == "" {
		return
	}
	f, err := os.OpenFile(*pid_file_path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("ERROR unable to open pidfile: %v", err)
		return
	}
	fmt.Fprintln(f, os.Getpid())
	onShutdown(rmPidfile)
}

// removes pidfile from disk
func rmPidfile() {
	if *pid_file_path == "" {
		return
	}
	os.Remove(*pid_file_path)
}

func awaitSignals() {
	die, hup := make(chan os.Signal, 1), make(chan os.Signal, 1)
	signal.Notify(die, os.Interrupt, os.Kill)
	signal.Notify(hup, syscall.SIGHUP)
	for {
		select {
		case <-die:
			log.Println("lumberjack shutting down")
			shutdown(nil)
		case <-hup:
			refreshLogfileHandle()
		}
	}
}

func refreshLogfileHandle() {
	if *log_file_path == "" {
		return
	}

	f, err := os.OpenFile(*log_file_path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("unable to open logfile destination: %v", err)
	} else {
		log.SetOutput(f)
	}

	if log_file_handle != nil {
		log_file_handle.Close()
	}

	log_file_handle = f
}

func setupLogging() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	if *use_syslog {
		configureSyslog()
	} else if *log_file_path != "" {
		refreshLogfileHandle()
	}
}

// adds a shutdown handler to the list of shutdown handlers.  These handlers
// are called when we exit lumberjack with a call to shutdown, but not with
// log.Fatal, so... don't use log.Fatal.
func onShutdown(fn func()) {
	if shutdownHandlers == nil {
		shutdownHandlers = make([]func(), 0, 8)
	}
	shutdownHandlers = append(shutdownHandlers, fn)
}

func shutdown(v interface{}) {
	for _, fn := range shutdownHandlers {
		fn()
	}
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
	writePid()
	go cmdListener()
	log.Println("lumberjack starting")

	startCPUProfile()

	config, err := LoadConfig(*config_file)
	if err != nil {
		fmt.Println("one")
		fmt.Println(err)
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
