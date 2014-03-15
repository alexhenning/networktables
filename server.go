package networktables

import (
	"encoding/binary"
	"errors"
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
				for _, entry := range nt.entries {
					nt.assignEntry(entry, rwc)
				}
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
			if err := nt.handleEntryAssignment(c, rwc); err != nil {
				log.Println(err)
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

func (nt *NetworkTable) handleEntryAssignment(c <-chan byte, rwc net.Conn) error {
	name, entryType, id, sequence := getString(c), <-c, getUint16(c), getUint16(c)

	if _, exists := nt.entries[name]; exists {
		log.Printf("Warning, client requesting an already existing key, ignoring.\n")
		return nil
	}
	if id != ClientRequestID {
		return errors.New("Error, assertive client trying to pick the ID it assigns, closing connection.")
	}

	id, nt.nextID = nt.nextID, nt.nextID+1
	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d\n", name, entryType, id, sequence)

	switch entryType {
	case Boolean:
		b := getBoolean(c)
		log.Printf("\tValue: %t\n", b)
		nt.entries[name] = newBooleanEntry(name, b, id, sequence)
	case Double:
		d := getDouble(c)
		log.Printf("\tValue: %f\n", d)
		nt.entries[name] = newDoubleEntry(name, d, id, sequence)
	case String:
		s := getString(c)
		log.Printf("\tValue: %s\n", s)
		nt.entries[name] = newStringEntry(name, s, id, sequence)
	case BooleanArray, DoubleArray, StringArray:
		return errors.New("Error, server currently can't handle array types, closing connection.\n")
	}
	nt.assignEntryAll(nt.entries[name])
	return nil
}

func (nt *NetworkTable) assignEntry(e entry, rwc net.Conn) {
	data := e.Marshal()
	log.Printf("Send \"%X\"", data)
	written, err := rwc.Write(data)
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

func ListenAndServe(addr string) error {
	nt := &NetworkTable{addr, 1, make(map[string]entry)}
	return nt.ListenAndServe()
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
