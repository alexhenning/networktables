package networktables

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

type NetworkTable struct {
	addr        string
	nextID      uint16
	entries     map[string]entry
	connections []*connection
	m           sync.Mutex
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
		conn := &connection{rwc, nt, sync.Mutex{}}
		nt.connections = append(nt.connections, conn)
		go conn.run()
	}
	return nil
}

func (nt *NetworkTable) assignEntry(e entry, w io.Writer) {
	data := e.Marshal()
	log.Printf("Send \"%X\"", data)
	written, err := w.Write(data)
	if err != nil {
		log.Println(err)
	}
	if written != len(data) {
		log.Printf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written)
	}
}

func (nt *NetworkTable) assignEntryAll(e entry) {
	// TODO: implement
}

func (nt *NetworkTable) set(key string, e entry) {
	nt.m.Lock()
	defer nt.m.Unlock()
	nt.entries[key] = e
}

func ListenAndServe(addr string) error {
	nt := &NetworkTable{addr, 1, make(map[string]entry), nil, sync.Mutex{}}
	return nt.ListenAndServe()
}

// connection handles a single client connection
type connection struct {
	rwc net.Conn
	nt  *NetworkTable
	m   sync.Mutex
}

func (conn *connection) run() {
	defer conn.rwc.Close()
	log.Printf("Got new connection from %s", conn.rwc.RemoteAddr().String())
	done, c := make(chan error), make(chan byte)
	go conn.processBytes(done, c)
	for {
		data := make([]byte, 2048)
		n, err := conn.rwc.Read(data)
		if err != nil || n < 0 {
			log.Printf("networktables: %s\n", err)
			return
		}
		for i := 0; i < n; i++ {
			select {
			case c <- data[i]:
			case err := <-done:
				if err != nil {
					log.Println(err)
				}
				close(done)
				close(c)
				return
			}
		}
	}
}

func (conn *connection) processBytes(done chan<- error, c <-chan byte) {
	for b := range c {
		switch b {
		case KeepAlive:
		case Hello:
			version := getUint16(c)
			log.Printf("Received hello for version %d\n", version)
			if version == Version {
				for _, entry := range conn.nt.entries {
					conn.nt.assignEntry(entry, conn)
				}
				conn.Write([]byte{HelloComplete})
			} else {
				conn.Write([]byte{VersionUnsupported})
				done <- ErrUnsupportedVersion
				return
			}
		case VersionUnsupported:
			done <- ErrUnsupportedVersionMsg
			return
		case HelloComplete:
			done <- ErrHelloCompleteMsg
			return
		case EntryAssignment:
			log.Printf("Received entry assignment\n")
			if err := conn.handleEntryAssignment(c); err != nil {
				done <- err
				return
			}
		case EntryUpdate:
			log.Printf("Received entry update\n")
		default:
			done <- errors.New(fmt.Sprintf("networktables: received unexpected byte \"%X\"", b))
			return
		}
	}
}

func (conn *connection) handleEntryAssignment(c <-chan byte) error {
	name, entryType, id, sequence := getString(c), <-c, getUint16(c), getUint16(c)

	if _, exists := conn.nt.entries[name]; exists {
		log.Printf("Warning, client requesting an already existing key, ignoring.\n")
		return nil
	}
	if id != ClientRequestID {
		return ErrAssertiveClient
	}

	id, conn.nt.nextID = conn.nt.nextID, conn.nt.nextID+1
	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d\n", name, entryType, id, sequence)

	switch entryType {
	case Boolean:
		b := getBoolean(c)
		log.Printf("\tValue: %t\n", b)
		conn.nt.set(name, newBooleanEntry(name, b, id, sequence))
	case Double:
		d := getDouble(c)
		log.Printf("\tValue: %f\n", d)
		conn.nt.set(name, newDoubleEntry(name, d, id, sequence))
	case String:
		s := getString(c)
		log.Printf("\tValue: %s\n", s)
		conn.nt.set(name, newStringEntry(name, s, id, sequence))
	case BooleanArray, DoubleArray, StringArray:
		return ErrArraysUnsupported
	}
	conn.nt.assignEntryAll(conn.nt.entries[name])
	return nil
}

func (conn *connection) Write(b []byte) (int, error) {
	conn.m.Lock()
	defer conn.m.Unlock()
	return conn.rwc.Write(b)
}

// Encoding and decoding methods

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

func getUint16Bytes(val uint16) []byte {
	bytes := make([]byte, 2, 2)
	binary.BigEndian.PutUint16(bytes, val)
	return bytes
}

func getBooleanByte(val bool) byte {
	if val {
		return 0x01
	} else {
		return 0x00
	}
}

func getDoubleBytes(val float64) []byte {
	bytes := make([]byte, 8, 8)
	bits := math.Float64bits(val)
	binary.BigEndian.PutUint64(bytes, bits)
	return bytes
}

func getStringBytes(val string) []byte {
	bytes := make([]byte, 0, 2+len(val))
	bytes = append(bytes, getUint16Bytes((uint16)(len(val)))...)
	bytes = append(bytes, []byte(val)...)
	return bytes
}
