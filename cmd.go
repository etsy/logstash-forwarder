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

var replayCmd = cmd{
	name: "replay",
	run: func(args []string, w io.Writer) {
		var offset int64
		flags := flag.NewFlagSet("replay", flag.ContinueOnError)
		flags.Int64Var(&offset, "offset", 0, "offset to read file from")
		if err := flags.Parse(args); err != nil {
			fmt.Fprintf(w, "argument error: %v")
			fmt.Fprintf(w, "usage: replay [filename] [offset]")
			return
		}
		args = flags.Args()
		if len(args) == 0 {
			fmt.Fprintf(w, "usage: replay [filename] [--offset=N] [field1=value1 field2=value2 ... fieldN=valueN]")
			return
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
		log.Println(fields)
		h := &Harvester{Path: args[0], Fields: fields, out: event_chan}
		fmt.Fprintln(w, "ok")
		go h.Harvest(offset, h_Rewind)
	},
}

func registerCmd(c cmd) {
	commands[c.name] = c
}

func cmdListener() {
	l, err := net.Listen("tcp", ":9001")
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
			cleaned = append(cleaned, part)
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
	registerCmd(replayCmd)
}
