// This file contains the necessary methods and data structures to run
// a NetworkTables client that can connect to a server.

package networktables

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// Table is an interface that allows getting values out of
// NetworkTables and Subtables in a consistent way.
type Table interface {
	GetBoolean(key string) bool
	GetFloat64(key string) float64
	GetString(key string) string
	GetSubtable(key string) Table
}

// State of the client
type state int

// The two states the client can be in
const (
	disconnected = state(0)
	connected    = state(1)
)

// Client is the structure for creating and handling the NetworkTable
// client. The recommended way to create a new client is through
// networktables.NewClient().
type Client struct {
	addr          string
	entriesByName map[string]entry
	entriesByID   map[uint16]entry
	state         state
	conn          net.Conn
	m             sync.Mutex
}

// NewServer creates a new server object that can be used to listen
// and serve clients connected to the given address.
func NewClient(addr string, listen bool) *Client {
	client := &Client{
		addr:          addr,
		entriesByName: make(map[string]entry),
		entriesByID:   make(map[uint16]entry),
	}
	if listen {
		go client.ConnectAndListen()
	}
	return client
}

// ConnectAndListen connects to the NetworkTable server at cl.addr and
// listens for updates sent to the client.
func (cl *Client) ConnectAndListen() error {
	conn, err := net.Dial("tcp", cl.addr)
	if err != nil {
		log.Println(err)
		return err
	}
	defer conn.Close()
	cl.conn = conn
	log.Printf("Got new connection to %s", conn.RemoteAddr().String())

	err = cl.hello()
	if err != nil {
		return err
	}

	done, c := make(chan error), make(chan byte)
	defer close(done)
	defer close(c)
	go cl.processBytes(done, c)

	for {
		data := make([]byte, 2048)
		n, err := conn.Read(data)
		if err != nil {
			log.Printf("networktables: %s\n", err)
			return err
		}
		for i := 0; i < n; i++ {
			select {
			case c <- data[i]:
			case err := <-done:
				if err != nil {
					log.Println(err)
				}
				return err
			}
		}
	}
}

// hello sends the hello message for the implemented version.
func (cl *Client) hello() error {
	data := helloMessage(version)
	log.Printf("Send \"%X\"", data)
	written, err := cl.Write(data)
	if written != len(data) && err == nil {
		err = errors.New(fmt.Sprintf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written))
	}
	return err
}

// processBytes takes the stream of bytes on the channel and processes
// them, negotiating the hello exchange and receiving entry updates
// from the client.
func (cl *Client) processBytes(done chan<- error, c <-chan byte) {
	for b := range c {
		switch b {
		case keepAlive: // BUG(Alex) KeepAlive message currently ignored
		case hello:
			done <- ErrUnsupportedHelloMsg
			return
		case versionUnsupported:
			done <- ErrUnsupportedVersionMsg
			return
		case helloComplete:
			log.Printf("Received hello complete\n")
			if cl.state == disconnected {
				cl.state = connected
			} else {
				done <- ErrMultipleHellosCompleted
				return
			}
			// TODO: Send quequed assignments
		case entryAssignment:
			log.Printf("Received entry assignment\n")
			if err := cl.handleEntryAssignment(c); err != nil {
				done <- err
				return
			}
		case entryUpdate:
			log.Printf("Received entry update\n")
			if err := cl.handleEntryUpdate(c); err != nil {
				done <- err
				return
			}
		default:
			done <- errors.New(fmt.Sprintf("networktables: received unexpected byte \"%X\"", b))
			return
		}
	}
}

// handleEntryAssignment handles entry assignment messages sent to the
// client, updating the table.
func (cl *Client) handleEntryAssignment(c <-chan byte) error {
	name, entryType, id, sequence := getString(c), <-c, getUint16(c), sequenceNumber(getUint16(c))

	e, err := newEntry(name, id, sequence, entryType)
	if err != nil {
		return err
	}
	e.dataFromBytes(c)

	if _, exists := cl.entriesByName[name]; !exists {
		cl.set(e)
	} else {
		return errors.New("Warning, server assigning an already existing key.\n")
	}

	log.Printf("Name: %s Type: %X, ID: %X, Sequence Number: %d, Value %v\n",
		name, entryType, id, sequence, e.Value())
	return nil
}

// handleEntryUpdate handles entry update messages sent to the
// client and updates the table.
func (cl *Client) handleEntryUpdate(c <-chan byte) error {
	id, sequence := getUint16(c), sequenceNumber(getUint16(c))
	e := cl.entriesByID[id]
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
	return nil
}

// set stores an entry so that it can be referenced by ID or name in a
// manner that is safe to use from multiple goroutines.
func (cl *Client) set(e entry) {
	cl.m.Lock()
	defer cl.m.Unlock()
	cl.entriesByName[e.Name()] = e
	cl.entriesByID[e.ID()] = e
}

// Write is a allows the connection to be written to safely from
// multiple goroutines, blocking if necessary.
func (cl *Client) Write(b []byte) (int, error) {
	cl.m.Lock()
	defer cl.m.Unlock()
	return cl.conn.Write(b)
}
