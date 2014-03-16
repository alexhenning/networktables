// This file contains the necessary methods and data structures to run
// a NetworkTables server that can handle multiple clients.

package networktables

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// ListenAndServe listens on the TCP network address addr and then
// calls Serve with handler to handle requests on incoming
// connections. The default port for a server should be :1735
//
// A trivial example server is:
//
//	package main
//
//	import "github.com/alexhenning/networktables"
//
//	func main() {
//	    networktables.ListenAndServe(":1735")
//	}
func ListenAndServe(addr string) error {
	nt := &NetworkTable{addr, 1, make(map[string]entry), make(map[uint16]entry), nil, sync.Mutex{}}
	return nt.ListenAndServe()
}

// NetworkTable is the structure for creating and handling the
// NetworkTable server. If using the ListenAndServe function, it is
// not necessary to create this manually.
type NetworkTable struct {
	addr          string
	nextID        uint16
	entriesByName map[string]entry
	entriesByID   map[uint16]entry
	connections   []*connection
	m             sync.Mutex
}

// ListenAndServe listens on the TCP network address nt.addr and then
// calls Serve to handle requests on incoming connections.
func (nt *NetworkTable) ListenAndServe() error {
	log.Printf("Listening on %s\n", nt.addr)
	listener, err := net.Listen("tcp", nt.addr)
	if err != nil {
		return err
	}
	return nt.Serve(listener)
}

// Serve accepts incoming connections on the listener, creating a new
// service goroutine for each. The service goroutines take care of
// receiving data from their client and sending out updates to all
// clients as necessary
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

// assignEntry sends the assign entry message for an entry to the
// client.
func (nt *NetworkTable) assignEntry(e entry, w io.Writer) {
	data := assignmentMessage(e)
	log.Printf("Send \"%X\"", data)
	written, err := w.Write(data)
	if err != nil {
		log.Println(err)
	}
	if written != len(data) {
		log.Printf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written)
	}
}

// assignEntryAll sends the assign entry message for an entry to all
// connected clients.
func (nt *NetworkTable) assignEntryAll(e entry) {
	for _, conn := range nt.connections {
		nt.assignEntry(e, conn)
	}
}

// updateEntry sends the update entry message for an entry to the
// client.
func (nt *NetworkTable) updateEntry(e entry, w io.Writer) {
	data := updateMessage(e)
	log.Printf("Send \"%X\"", data)
	written, err := w.Write(data)
	if err != nil {
		log.Println(err)
	}
	if written != len(data) {
		log.Printf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written)
	}
}

// updateEntryAll sends the update entry message for an entry to all
// connected clients.
func (nt *NetworkTable) updateEntryAll(e entry) {
	for _, conn := range nt.connections {
		nt.updateEntry(e, conn)
	}
}

func (nt *NetworkTable) set(e entry) {
	nt.m.Lock()
	defer nt.m.Unlock()
	nt.entriesByName[e.Name()] = e
	nt.entriesByID[e.ID()] = e
}

// connection handles a single client connection
type connection struct {
	rwc net.Conn
	nt  *NetworkTable
	m   sync.Mutex
}

// run handles the connection, processing all incoming bytes from a
// client until the connection is either closed remotely or an error
// occurs, such as invalid values being sent.
func (conn *connection) run() {
	defer conn.rwc.Close()
	log.Printf("Got new connection from %s", conn.rwc.RemoteAddr().String())
	done, c := make(chan error), make(chan byte)
	defer close(done)
	defer close(c)
	go conn.processBytes(done, c)
	for {
		data := make([]byte, 2048)
		n, err := conn.rwc.Read(data)
		if err != nil {
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
				return
			}
		}
	}
}

// processBytes takes the stream of bytes on the channel and processes
// them, negotiating the hello exchange and receiving entry updates
// from the client.
func (conn *connection) processBytes(done chan<- error, c <-chan byte) {
	for b := range c {
		switch b {
		case KeepAlive: // BUG(Alex) KeepAlive message currently ignored
		case Hello:
			version := getUint16(c)
			log.Printf("Received hello for version %d\n", version)
			if version == Version {
				for _, entry := range conn.nt.entriesByName {
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
			if err := conn.handleEntryUpdate(c); err != nil {
				done <- err
				return
			}
		default:
			done <- errors.New(fmt.Sprintf("networktables: received unexpected byte \"%X\"", b))
			return
		}
	}
}

// handleEntryAssignment handles entry assignment messages sent from
// the client, updating the table and notifying other connected clients.
func (conn *connection) handleEntryAssignment(c <-chan byte) error {
	name, entryType, id, sequence := getString(c), <-c, getUint16(c), sequenceNumber(getUint16(c))

	if _, exists := conn.nt.entriesByName[name]; exists {
		// BUG(Alex) Fix race condition when two clients create an entry at the same time
		log.Printf("Warning, client requesting an already existing key, ignoring.\n")
		return nil
	}
	if id != ClientRequestID {
		return ErrAssertiveClient
	}

	id, conn.nt.nextID = conn.nt.nextID, conn.nt.nextID+1 // BUG(Alex) Make getting the next id threadsafe
	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d\n", name, entryType, id, sequence)

	switch entryType {
	case Boolean:
		b := getBoolean(c)
		log.Printf("\tValue: %t\n", b)
		conn.nt.set(newBooleanEntry(name, b, id, sequence))
	case Double:
		d := getDouble(c)
		log.Printf("\tValue: %f\n", d)
		conn.nt.set(newDoubleEntry(name, d, id, sequence))
	case String:
		s := getString(c)
		log.Printf("\tValue: %s\n", s)
		conn.nt.set(newStringEntry(name, s, id, sequence))
	case BooleanArray, DoubleArray, StringArray:
		return ErrArraysUnsupported
	}
	conn.nt.assignEntryAll(conn.nt.entriesByName[name])
	return nil
}

// handleEntryUpdate handles entry update messages sent from the
// client, updating the table and notifying other connected clients.
func (conn *connection) handleEntryUpdate(c <-chan byte) error {
	id, sequence := getUint16(c), sequenceNumber(getUint16(c))
	e := conn.nt.entriesByID[id]
	if !e.SequenceNumber().gt(sequence) {
		log.Printf("Warning, client updating an entry with an out of date sequence number, ignoring.\n")
		return nil
	}

	// BUG(Alex) Make updating entries more threadsafe, possible race condition exists
	e.SetSequenceNumber(sequence)
	e.dataFromBytes(c)
	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d\n",
		e.Name(), e.Type(), e.ID(), e.SequenceNumber())
	conn.nt.updateEntryAll(e)
	return nil
}

// Write is a allows the connection to be written to safely from
// multiple goroutines, blocking if necessary.
func (conn *connection) Write(b []byte) (int, error) {
	conn.m.Lock()
	defer conn.m.Unlock()
	return conn.rwc.Write(b)
}
