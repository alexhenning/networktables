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
	return NewServer(addr).ListenAndServe()
}

// Server is the structure for creating and handling the NetworkTable
// server. If using the ListenAndServe function, it is not necessary
// to create this manually.
type Server struct {
	addr          string
	nextID        uint16
	entriesByName map[string]entry
	entriesByID   map[uint16]entry
	connections   []*connection
	m             sync.Mutex
	idM           sync.Mutex
}

// NewServer creates a new server object that can be used to listen
// and serve clients connected to the given address.
func NewServer(addr string) *Server {
	return &Server{
		addr:          addr,
		entriesByName: make(map[string]entry),
		entriesByID:   make(map[uint16]entry),
	}
}

// ListenAndServe listens on the TCP network address srv.addr and then
// calls Serve to handle requests on incoming connections.
func (srv *Server) ListenAndServe() error {
	log.Printf("Listening on %s\n", srv.addr)
	listener, err := net.Listen("tcp", srv.addr)
	if err != nil {
		return err
	}
	return srv.Serve(listener)
}

// Serve accepts incoming connections on the listener, creating a new
// service goroutine for each. The service goroutines take care of
// receiving data from their client and sending out updates to all
// clients as necessary
func (srv *Server) Serve(listener net.Listener) error {
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
		conn := &connection{rwc, srv, sync.Mutex{}}
		srv.m.Lock()
		srv.connections = append(srv.connections, conn)
		srv.m.Unlock()
		go conn.run()
	}
	return nil
}

// assignEntry sends the assign entry message for an entry to the
// cliesrv.
func (srv *Server) assignEntry(e entry, w io.Writer) {
	e.Lock()
	defer e.Unlock()
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
func (srv *Server) assignEntryAll(e entry) {
	for _, conn := range srv.connections {
		srv.assignEntry(e, conn)
	}
}

// updateEntry sends the update entry message for an entry to the
// client.
func (srv *Server) updateEntry(e entry, w io.Writer) {
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
func (srv *Server) updateEntryAll(e entry) {
	for _, conn := range srv.connections {
		srv.updateEntry(e, conn)
	}
}

// set stores an entry so that it can be referenced by ID or name in a
// manner that is safe to use from multiple goroutines.
func (srv *Server) set(e entry) {
	srv.m.Lock()
	defer srv.m.Unlock()
	srv.entriesByName[e.Name()] = e
	srv.entriesByID[e.ID()] = e
}

// id returns a unique id in a manner that is safe to use from
// multiple goroutines.
func (srv *Server) id() uint16 {
	srv.idM.Lock()
	defer srv.idM.Unlock()
	id := srv.nextID
	srv.nextID++
	return id
}

func (srv *Server) removeConnection(conn *connection) {
	srv.m.Lock()
	defer srv.m.Unlock()
	for i, c := range srv.connections {
		if c == conn {
			srv.connections = append(srv.connections[:i], srv.connections[i+1:]...)
			return
		}
	}
}

// connection handles a single client connection.
type connection struct {
	rwc net.Conn
	srv *Server
	m   sync.Mutex
}

// run handles the connection, processing all incoming bytes from a
// client until the connection is either closed remotely or an error
// occurs, such as invalid values being sent.
func (conn *connection) run() {
	defer conn.rwc.Close()
	defer conn.srv.removeConnection(conn)
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
		case keepAlive: // BUG(Alex) KeepAlive message currently ignored
		case hello:
			v := getUint16(c)
			log.Printf("Received hello for version %d\n", version)
			if v == version {
				for _, entry := range conn.srv.entriesByName {
					conn.srv.assignEntry(entry, conn)
				}
				conn.Write([]byte{helloComplete})
			} else {
				conn.Write([]byte{versionUnsupported})
				done <- ErrUnsupportedVersion
				return
			}
		case versionUnsupported:
			done <- ErrUnsupportedVersionMsg
			return
		case helloComplete:
			done <- ErrHelloCompleteMsg
			return
		case entryAssignment:
			log.Printf("Received entry assignment\n")
			if err := conn.handleEntryAssignment(c); err != nil {
				done <- err
				return
			}
		case entryUpdate:
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

	if id != clientRequestID {
		return ErrAssertiveClient
	}
	id = conn.srv.id()

	e, err := newEntry(name, id, sequence, entryType)
	if err != nil {
		return err
	}
	e.dataFromBytes(c)

	if _, exists := conn.srv.entriesByName[name]; !exists {
		conn.srv.set(e)
	} else {
		log.Printf("Warning, client requesting an already existing key, ignoring.\n")
		return nil
	}

	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d, Value %v\n",
		name, entryType, id, sequence, e.Value())
	conn.srv.assignEntryAll(e)
	return nil
}

// handleEntryUpdate handles entry update messages sent from the
// client, updating the table and notifying other connected clients.
func (conn *connection) handleEntryUpdate(c <-chan byte) error {
	id, sequence := getUint16(c), sequenceNumber(getUint16(c))
	e := conn.srv.entriesByID[id]
	e.Lock()
	defer e.Unlock()

	if !e.SequenceNumber().gt(sequence) {
		log.Printf("Warning, client updating an entry with an out of date sequence number, ignoring.\n")
		return nil
	}

	e.SetSequenceNumber(sequence)
	e.dataFromBytes(c)
	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d, Value %v\n",
		e.Name(), e.Type(), e.ID(), e.SequenceNumber(), e.Value())
	conn.srv.updateEntryAll(e)
	return nil
}

// Write is a allows the connection to be written to safely from
// multiple goroutines, blocking if necessary.
func (conn *connection) Write(b []byte) (int, error) {
	conn.m.Lock()
	defer conn.m.Unlock()
	return conn.rwc.Write(b)
}
