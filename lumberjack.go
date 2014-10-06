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
	registry         *hregistry
	shutdownHandlers []func()
	log_file_handle  *os.File
)

// creates a file and writes the current process's pid into that file.  The
// file name is specified on the command line.
func writePid() {
	if options.PidFile == "" {
		return
	}
	f, err := os.OpenFile(options.PidFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("ERROR unable to open pidfile: %v", err)
		return
	}
	fmt.Fprintln(f, os.Getpid())
	onShutdown(rmPidfile)
}

// removes pidfile from disk
func rmPidfile() {
	if options.PidFile == "" {
		return
	}
	os.Remove(options.PidFile)
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
	if options.LogFile == "" {
		return
	}

	f, err := os.OpenFile(options.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	if options.UseSyslog {
		configureSyslog()
	} else if options.LogFile != "" {
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

var publisherId = 0

func startPublishers(conf NetworkConfig, out chan eventPage) error {
	for _, group := range conf {
		tlsConfig, err := group.TLS()
		if err != nil {
			return fmt.Errorf("unable to start publishers: %v", err)
		}

		for _, server := range group.Servers {
			p := &Publisher{
				id:        publisherId,
				sequence:  1,
				addr:      server,
				tlsConfig: *tlsConfig,
				timeout:   group.timeout,
			}
			log.Printf("TLS config: %v\n", tlsConfig)
			go p.publish(group.c_pages_unsent, out)
			publisherId++
		}
	}
	return nil
}

func startHttp() {
	if options.HttpPort != "" {
		log.Printf("starting http debug port on %s", options.HttpPort)
		if err := http.ListenAndServe(options.HttpPort, nil); err != nil {
			log.Printf("unable to open http port: %v", err)
		}
	} else {
		log.Println("no http port specified")
	}
}

// handles command line args.  That is, positional arguments, not flag
// arguments.  This is for handling subcommands, which at the time of writing,
// is just the ability to test a configuration file.
func handleArgs() {
	if flag.NArg() == 0 {
		return
	}

	switch flag.Arg(0) {
	case "test-config":
		if flag.NArg() < 2 {
			shutdown("not enough arguments specified for test-config")
		}
		testConfig(flag.Arg(1))
	default:
		shutdown(fmt.Sprintf("unrecognized positional arg: %v", flag.Arg(0)))
	}
}

func main() {
	flag.Parse()
	handleArgs()

	runtime.GOMAXPROCS(options.NumThreads)
	setupLogging()
	writePid()
	log.Println("lumberjack starting")

	startCPUProfile()

	config, err := LoadConfig(options.ConfigFile)
	if err != nil {
		fmt.Println(err)
		shutdown(err.Error())
	}

	go cmdListener()
	registry = newRegistry(config)

	registrar_chan := make(chan eventPage, 1)

	if len(config.Files) == 0 {
		shutdown("No paths given. What files do you want me to watch?\n")
	}

	go reportFSEvents()
	// Prospect the globs/paths given on the command line and launch harvesters
	for _, fileconfig := range config.Files {
		go Prospect(fileconfig, config.Network)
	}

	// Harvesters dump events into the spooler.
	for _, group := range config.Network {
		group.Spool()
	}

	if err := startPublishers(config.Network, registrar_chan); err != nil {
		shutdown(err)
	}

	// registrar records last acknowledged positions in all files.
	go Registrar(registrar_chan)
	go startHttp()
	awaitSignals()
}

func startCPUProfile() {
	if options.CPUProfile != "" {
		f, err := os.Create(options.CPUProfile)
		if err != nil {
			shutdown(err)
		}
		pprof.StartCPUProfile(f)
		go func() {
			time.Sleep(60 * time.Second)
			pprof.StopCPUProfile()
			panic("done")
		}()
	}
}
