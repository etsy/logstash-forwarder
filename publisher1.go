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
	id       int
	buffer   bytes.Buffer
	socket   *tls.Conn
	sequence uint32
}

func newPublisher() *Publisher {
	p := Publisher{
		id:       publisherId,
		sequence: 1,
	}
	publisherId++
	return &p
}

func (p *Publisher) publish(input chan eventPage, registrar chan eventPage, config *NetworkConfig) {
	p.connect(config, p.id)
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
		if err := p.sendPayload(len(page), compressed_payload, config.timeout); err != nil {
			log.Printf("Socket error, will reconnect: %s\n", err)
			time.Sleep(1 * time.Second)
			p.socket.Close()
			p.connect(config, p.id)
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
				p.connect(config, p.id)
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

func (p *Publisher) sendPayload(size int, payload []byte, timeout time.Duration) error {
	p.socket.SetDeadline(time.Now().Add(timeout))

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

func (p *Publisher) connect(config *NetworkConfig, id int) {
	tlsconfig, err := config.TLS()
	if err != nil {
		// this was always a fatal but it shouldn't be.  This is enough change
		// for one commit.  I'll make this not a fatal soon enough.
		log.Fatalf("unable to connect: %v", err)
	}

	for {
		// Pick a random server from the list.
		address := config.Servers[rand.Int()%len(config.Servers)]
		log.Printf("Connecting publisher %v to %s\n", id, address)

		tcpsocket, err := net.DialTimeout("tcp", address, config.timeout)
		if err != nil {
			log.Printf("Failure connecting publisher %v to %s: %s\n", id, address, err)
			time.Sleep(1 * time.Second)
			continue
		}

		p.socket = tls.Client(tcpsocket, tlsconfig)
		p.socket.SetDeadline(time.Now().Add(config.timeout))
		err = p.socket.Handshake()
		if err != nil {
			log.Printf("Failed to tls handshake with %s %s\n", address, err)
			time.Sleep(1 * time.Second)
			p.socket.Close()
			continue
		}

		log.Printf("Publisher %v connected to %s\n", id, address)
		return
	}
	panic("not reached")
}
