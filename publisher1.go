package main

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
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
		p.socket.Close()
	}()

	for page := range input {
		if err := page.compress(p.sequence, &p.buffer); err != nil {
			log.Println(err)
			//  if we hit this, we've lost log lines.  This is potentially
			//  fatal and should alert a human.
			continue
		}
		p.sequence += uint32(len(page))
		compressed_payload := p.buffer.Bytes()

	SendPayload:
		if err := p.sendPayload(len(page), compressed_payload); err != nil {
			log.Printf("Socket error, will reconnect: %s\n", err)
			time.Sleep(1 * time.Second)
			p.socket.Close()
			p.connect()
			goto SendPayload
		}

		// read ack
		response := make([]byte, 0, 6)
		ackbytes := 0
		for ackbytes != 6 {
			n, err := p.socket.Read(response[len(response):cap(response)])
			if err != nil {
				log.Printf("Read error looking for ack: %s\n", err)
				p.socket.Close()
				p.connect()
				goto SendPayload // retry sending on new connection
			} else {
				ackbytes += n
			}
		}

		// TODO(sissel): verify ack

		// Tell the registrar that we've successfully sent these events
		registrar <- page
	} /* for each event payload */

}

func (p *Publisher) sendPayload(size int, payload []byte) error {
	p.socket.SetDeadline(time.Now().Add(p.timeout))

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
			log.Printf("Failure connecting publisher %v to %s: %s\n", p.id, p.addr, err)
			time.Sleep(1 * time.Second)
			continue
		}
		p.socket = tls.Client(sock, &p.tlsConfig)
		p.socket.SetDeadline(time.Now().Add(p.timeout))
		if err := p.socket.Handshake(); err != nil {
			log.Printf("Failed to tls handshake with %s %s\n", p.addr, err)
			time.Sleep(1 * time.Second)
			p.socket.Close()
		}
		log.Printf("Publisher %v connected to %s\n", p.id, p.addr)
		return
	}
}
