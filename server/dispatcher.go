package server

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type dispatcher struct {
	// Registered subscribers.
	subscribers map[string]*Subscriber

	// Subscribe clients to the stream.
	subscribe chan *net.UDPAddr

	// Unsubscribe clients to the stream.
	unsubscribe chan *net.UDPAddr

	// Update alive clients status.
	alive chan *net.UDPAddr

	// Inbound package from the input stream.
	stream chan []byte
}

func NewDispatcher() *dispatcher {
	return &dispatcher{
		subscribers: make(map[string]*Subscriber),
		subscribe:   make(chan *net.UDPAddr),
		unsubscribe: make(chan *net.UDPAddr),
		alive:       make(chan *net.UDPAddr),
		stream:      make(chan []byte, 100),
	}
}

func (d *dispatcher) Run(outputStreamAddr *net.UDPAddr) {
	fmt.Println("Run dispatcher...")

	outputStreamConn, err := net.ListenUDP("udp", outputStreamAddr)
	CheckError(err)

	defer outputStreamConn.Close()

	// client manager
	go func() {
		clientBuf := make([]byte, BUF_SIZE)

		for {
			n, cliaddr, err := outputStreamConn.ReadFromUDP(clientBuf)
			if err != nil {
				fmt.Println("Error: ", err)
				continue
			}

			msg := string(clientBuf[0:n])

			fmt.Printf("< \"%s\" from %s\n", msg, cliaddr.String())

			if strings.HasPrefix(msg, "CONNECT") {
				if len(msg) > 7 {
					altPort, _ := strconv.Atoi(msg[8:])
					if altPort > 0 {
						altAddr := &net.UDPAddr{
							IP:   cliaddr.IP,
							Port: altPort,
							Zone: cliaddr.Zone,
						}
						cliaddr = altAddr
					}
				}
				fmt.Printf("sub %d\n", cliaddr.Port)
				d.subscribe <- cliaddr
				continue
			}

			if strings.HasPrefix(msg, "DISCONNECT") {
				d.unsubscribe <- cliaddr
				continue
			}

			if strings.HasPrefix(msg, "ALIVE") {
				d.alive <- cliaddr
				continue
			}
		}
	}()

	for {
		select {
		case address := <-d.subscribe:
			if _, ok := d.subscribers[address.String()]; !ok {
				s := newSubscriber(address)

				d.subscribers[address.String()] = s
				//fmt.Println("+", address.String())

				go s.CheckAlive(d.unsubscribe)
			}

		case address := <-d.unsubscribe:
			if _, ok := d.subscribers[address.String()]; ok {
				fmt.Println("unsubscribe -", address.String())
				delete(d.subscribers, address.String())
				//fmt.Println("-", address.String())
			}
		case address := <-d.alive:
			if _, ok := d.subscribers[address.String()]; ok {
				d.subscribers[address.String()].UpdateAliveTime()
			}
		case bytes := <-d.stream:
			for _, s := range d.subscribers {
				outputStreamConn.WriteToUDP(bytes, s.Address)
			}
		}
	}
}
