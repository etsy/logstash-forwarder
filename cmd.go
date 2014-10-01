package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

var commands = make(map[string]cmd)

type cmd struct {
	name string
	run  func([]string, io.Writer)
}

func defineReplayCmd(conf *Config) {
	replayCmd := cmd{
		name: "replay",
		run: func(args []string, w io.Writer) {
			var offset int64
			var dest string
			flags := flag.NewFlagSet("replay", flag.ContinueOnError)
			flags.Int64Var(&offset, "offset", 0, "offset to read file from")
			flags.StringVar(&dest, "dest", "", "logstash server group destination")

			if err := flags.Parse(args); err != nil {
				fmt.Fprintf(w, "argument error: %v")
				fmt.Fprintf(w, "usage: replay [filename] [offset]")
				return
			}
			args = flags.Args()
			if len(args) == 0 {
				fmt.Fprintf(w, "usage: replay [--offset=N] [--dest=...] [filename] [field1=value1 field2=value2 ... fieldN=valueN]\n")
				return
			}
			for i, _ := range args {
				args[i] = strings.TrimSpace(args[i])
			}
			fields := make(map[string]string, len(args)-1)
			if len(args) > 1 {
				for i := 1; i < len(args); i++ {
					parts := strings.Split(args[i], "=")
					if len(parts) != 2 {
						fmt.Fprintf(w, "unable to parse field: %s", args[i])
						return
					}
					fields[parts[0]] = strings.TrimSpace(parts[1])
				}
			}
			var c chan *FileEvent
			if dest != "" {
				c = conf.Network.EventChan(dest)
			} else {
				c = conf.Network.EventChan(conf.FileDest(args[0]))
			}
			if c != nil {
				fmt.Fprintf(w, "unable to get event chan for file path")
				return
			}
			h := &Harvester{Path: args[0], Fields: fields, out: c, join: nil}
			fmt.Fprintln(w, "ok")
			go h.Harvest(offset, h_Rewind)
		},
	}
	registerCmd(replayCmd)
}

var infoCmd = cmd{
	name: "info",
	run: func(args []string, w io.Writer) {
		fmt.Fprintln(w, "[inode_harvesters]")
		for id, h := range registry.RunningIds {
			fmt.Fprintf(w, "%v: %v\n", id, h)
		}
		fmt.Fprintln(w, "[name_harvesters]")
		for path, h := range registry.RunningPaths {
			fmt.Fprintf(w, "%v: %v\n", path, h)
		}
		fmt.Fprintln(w, registry)
	},
}

func registerCmd(c cmd) {
	commands[c.name] = c
}

func cmdListener() {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *cmd_port))
	if err != nil {
		log.Println("unable to open command port: %v", err)
		return
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("error accepting connection: %v", err)
			continue
		}
		go cmdHandler(conn)
	}
}

func cmdHandler(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	for {
		line, err := r.ReadString('\n')
		switch err {
		case nil:
			if strings.TrimSpace(line) == "" {
				break
			}
			runCmd(conn, line)
		case io.EOF:
			return
		default:
			log.Println("err on cmd connection: %v", err)
		}
	}
}

func runCmd(conn net.Conn, line string) {
	parts := strings.Split(line, " ")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			cleaned = append(cleaned, strings.TrimSpace(part))
		}
	}
	if len(cleaned) == 0 {
		return
	}
	if c, ok := commands[cleaned[0]]; ok {
		c.run(cleaned[1:], conn)
	} else {
		fmt.Fprintf(conn, "unknown command: %s", cleaned[0])
		return
	}
}

func init() {
	registerCmd(infoCmd)
}
