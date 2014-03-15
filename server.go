package networktables

import (
	"log"
	"net"
	"time"
)

type NetworkTable struct {
	addr string
}

func (nt *NetworkTable) ListenAndServe() error {
	log.Printf("Listening on %s\n", nt.addr)
	listener, err := net.Listen("tcp", nt.addr)
	if err != nil {
		return err
	}
	return nt.Serve(listener)
}

func (nt *NetworkTable) Serve(listener net.Listener) error {
	defer listener.Close()
	log.Printf("Serving\n")
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		rwc, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("networktables: Accept error: %v; retrying in %v\n", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		log.Printf("Got connection\n")
		go nt.handleConnection(rwc)
	}
	return nil
}

func (nt *NetworkTable) handleConnection(rwc net.Conn) {
	defer rwc.Close()
	log.Printf("Got new connection from %s (%s)", rwc.RemoteAddr().String(), rwc.RemoteAddr().Network())
}

func ListenAndServe(addr string) error {
	nt := &NetworkTable{addr}
	return nt.ListenAndServe()
}
