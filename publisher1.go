package main

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

var hostname string

func init() {
	log.Printf("publisher init\n")
	hostname, _ = os.Hostname()
	rand.Seed(time.Now().UnixNano())
}

var publisherId = 0

type Publisher struct {
	id        int           // unique publisher id
	buffer    bytes.Buffer  // recyclable buffer for data to be sent
	socket    *tls.Conn     // currently active connection. may be nil.
	sequence  uint32        // incremental event id for current connection.
	addr      string        // tcp address to connect to
	tlsConfig tls.Config    // tls config to use for establishing secure connection
	timeout   time.Duration // send timeout
}

func (p *Publisher) publish(input chan eventPage, registrar chan eventPage) {
	p.connect()
	defer func() {
		log.Printf("publisher %v done", p.id)
		if err := p.socket.Close(); err != nil {
			log.Printf("unable to close connection to logstash server %s on publisher end: %v\n", p.addr, err)
		}
	}()

SENDING:
	for page := range input {
		if err := page.compress(p.sequence, &p.buffer); err != nil {
			log.Println(err)
			//  if we hit this, we've lost log lines.  This is potentially
			//  fatal and should alert a human.
			continue
		}
		p.sequence += uint32(len(page))
		compressed_payload := p.buffer.Bytes()

	SENDPAYLOAD:
		if err := p.sendPayload(len(page), compressed_payload); err != nil {
			input <- page
			sleep := time.Duration(1e9 + rand.Intn(1e10))
			log.Printf("Socket error, will reconnect in %v: %s\n", sleep, err)
			time.Sleep(sleep)
			if err := p.socket.Close(); err != nil {
				log.Printf("unable to close connection to logstash server %s during sendpayload: %v\n", p.addr, err)
			}
			p.connect()
			continue SENDING
		}

		// read ack
		response := make([]byte, 6)
		ackbytes := 0
		for ackbytes != 6 {
			n, err := p.socket.Read(response)
			if err != nil {
				log.Printf("Read error after %d bytes looking for ack: %s\n", n, err)
				log.Println("page will be re-sent")
				log.Println("closing socket to %s", p.addr)
				if err := p.socket.Close(); err != nil {
					log.Printf("unable to close connection to logstash server %s during ack: %v\n", p.addr, err)
				} else {
					log.Printf("publisher closed connection to %s\n", p.addr)
				}
				p.connect()
				goto SENDPAYLOAD
			} else {
				ackbytes += n
			}
		}

		// TODO(sissel): verify ack

		// Tell the registrar that we've successfully sent these events
		log.Printf("publisher %d sent %d events to %s", p.id, len(page), p.addr)
		registrar <- page
	} /* for each event payload */

}

func (p *Publisher) sendPayload(size int, payload []byte) error {
	if err := p.socket.SetDeadline(time.Now().Add(p.timeout)); err != nil {
		return fmt.Errorf("unable to set deadline in sendPayload: %v", err)
	}

	w := &errorWriter{Writer: p.socket}

	// Set the window size to the length of this payload in events.
	w.Write([]byte("1W"))
	binary.Write(w, binary.BigEndian, uint32(size))

	// Write compressed frame
	w.Write([]byte("1C"))
	binary.Write(w, binary.BigEndian, uint32(len(payload)))
	w.Write(payload)

	return w.Err()
}

func (p *Publisher) connect() {
	for {
		sock, err := net.DialTimeout("tcp", p.addr, p.timeout)
		if err != nil {
			sleep := time.Duration(1e9 + rand.Intn(1e10))
			log.Printf("Failure connecting publisher %v to %s: %s\n", p.id, p.addr, err)
			log.Printf("reconnect in %v", sleep)
			time.Sleep(sleep)
			continue
		}
		p.socket = tls.Client(sock, &p.tlsConfig)
		if err := p.socket.SetDeadline(time.Now().Add(p.timeout)); err != nil {
			log.Printf("unable to set deadline in connect: %v\n", err)
			continue
		}
		if err := p.socket.Handshake(); err != nil {
			sleep := time.Duration(1e9 + rand.Intn(1e10))
			log.Printf("Failed to tls handshake with %s %s\n", p.addr, err)
			time.Sleep(sleep)
			if err := p.socket.Close(); err != nil {
				log.Printf("unable to close connection to logstash server %s during handshake: %v\n", p.addr, err)
			} else {
				log.Printf("publisher closed connection to %s during handshake\n", p.addr)
			}
			continue
		}
		log.Printf("Publisher %v connected to %s\n", p.id, p.addr)
		return
	}
}
