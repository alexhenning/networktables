package networktables

import (
	"encoding/binary"
	"log"
	"math"
	"net"
	"time"
)

type NetworkTable struct {
	addr    string
	nextID  uint16
	entries map[string]entry
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
	done, c := make(chan interface{}), make(chan byte)
	go nt.processBytes(done, c, rwc)
	for {
		data := make([]byte, 2048)
		n, err := rwc.Read(data)
		if err != nil || n < 0 {
			log.Printf("networktables: %s\n", err)
			return
		}
		for i := 0; i < n; i++ {
			select {
			case c <- data[i]:
			case <-done:
				close(done)
				close(c)
				return
			}
		}
	}
}

func (nt *NetworkTable) processBytes(done chan<- interface{}, c <-chan byte, rwc net.Conn) {
	for b := range c {
		switch b {
		case KeepAlive:
		case Hello:
			version := getUint16(c)
			log.Printf("Received hello for version %d\n", version)
			if version == Version {
				// TODO: Send known entries
				rwc.Write([]byte{HelloComplete})
			} else {
				rwc.Write([]byte{VersionUnsupported})
				done <- true
				return
			}
		case VersionUnsupported:
			log.Printf("Error, server shouldn't get VersionUnsupported message, closing connection.\n")
			done <- true
			return
		case HelloComplete:
			log.Printf("Error, server shouldn't get HelloComplete message, closing connection.\n")
			done <- true
			return
		case EntryAssignment:
			log.Printf("Received entry assignment\n")
			name, entryType, id, sequence := getString(c), <-c, getUint16(c), getUint16(c)
			log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d\n", name, entryType, id, sequence)
			_, exists := nt.entries[name]
			if exists {
				log.Printf("Warning, client requesting an already existing key, ignoring.\n")
				continue
			}
			if id != ClientRequestID {
				log.Printf("Error, assertive client trying to pick the ID it assigns, closing connection.\n")
				done <- true
				return
			}
			id, nt.nextID = nt.nextID, nt.nextID+1
			log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d\n", name, entryType, id, sequence)
			switch entryType {
			case Boolean:
				b := getBoolean(c)
				log.Printf("\tValue: %t\n", b)
				nt.entries[name] = newBooleanEntry(b, id, sequence)
				// TODO: send to clients
			case Double:
				d := getDouble(c)
				log.Printf("\tValue: %f\n", d)
				nt.entries[name] = newDoubleEntry(d, id, sequence)
				// TODO: send to clients
			case String:
				s := getString(c)
				log.Printf("\tValue: %s\n", s)
				nt.entries[name] = newStringEntry(s, id, sequence)
				// TODO: send to clients
			case BooleanArray, DoubleArray, StringArray:
				log.Printf("Error, server currently can't handle array types, closing connection.\n")
				done <- true
				return
			}
		case EntryUpdate:
			log.Printf("Received entry update\n")
		default:
			log.Printf("Received byte \"%X\"", b)
		}
	}
}

func ListenAndServe(addr string) error {
	nt := &NetworkTable{addr, 1, make(map[string]entry)}
	return nt.ListenAndServe()
}

// getUint16 returns a 16 byte unsigned number read from the channel
// in little endian form. This may be a bug with the protocol.
func getUint16(c <-chan byte) uint16 {
	return binary.BigEndian.Uint16([]byte{<-c, <-c})
}

func getBoolean(c <-chan byte) bool {
	return (<-c) != 0x00
}

func getDouble(c <-chan byte) float64 {
	bits := binary.BigEndian.Uint64([]byte{<-c, <-c, <-c, <-c, <-c, <-c, <-c, <-c})
	return math.Float64frombits(bits)
}

func getString(c <-chan byte) string {
	length := getUint16(c)
	bytes := make([]byte, length, length)
	for i := uint16(0); i < length; i++ {
		bytes[i] = <-c
	}
	return string(bytes)
}
