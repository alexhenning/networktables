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
	GetBoolean(string) (bool, error)
	GetFloat64(string) (float64, error)
	GetString(string) (string, error)
	GetSubtable(string) (Table, error)
	PutBoolean(string, bool) error
	PutFloat64(string, float64) error
	PutString(string, string) error
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
	// BUG(Alex) KeepAlive message currently not sent
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

// updateEntry sends the update entry message for an entry to the
// server.
func (cl *Client) updateEntry(e entry) {
	data := updateMessage(e)
	log.Printf("Send \"%X\"", data)
	written, err := cl.conn.Write(data)
	if err != nil {
		log.Println(err)
	}
	if written != len(data) {
		log.Printf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written)
	}
}

// processBytes takes the stream of bytes on the channel and processes
// them, negotiating the hello exchange and receiving entry updates
// from the client.
func (cl *Client) processBytes(done chan<- error, c <-chan byte) {
	for b := range c {
		switch b {
		case keepAlive:
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
			// TODO: Send queued assignments
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
		e, err := newEntry(e.Name(), id, sequence, e.Type())
		if err != nil {
			return err
		}
		e.dataFromBytes(c)
		log.Printf("Warning, server updating an entry with an out of date sequence number, ignoring.\n")
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

// get gets the entry associated with the key if the hello finished
// command has been received and an entry with the key exists.
func (cl *Client) get(key string) (entry, error) {
	if cl.state != connected {
		return nil, ErrHelloNotDone
	}
	e, ok := cl.entriesByName[key]
	if !ok {
		return nil, ErrNoSuchKey
	}
	return e, nil
}

// GetBoolean returns the boolean value associated with the key if
// possible. This method is safe to use from multiple goroutines.
func (cl *Client) GetBoolean(key string) (bool, error) {
	e, err := cl.get(key)
	if err != nil {
		return false, err
	}
	e.Lock()
	defer e.Unlock()

	if e.Type() != tBoolean {
		return false, ErrWrongType
	}

	return e.Value().(bool), nil
}

// GetFloat64 returns the float64 value associated with the key if
// possible. This method is safe to use from multiple goroutines.
func (cl *Client) GetFloat64(key string) (float64, error) {
	e, err := cl.get(key)
	if err != nil {
		return 0, err
	}
	e.Lock()
	defer e.Unlock()

	if e.Type() != tDouble {
		return 0, ErrWrongType
	}

	return e.Value().(float64), nil
}

// GetString returns the string value associated with the key if
// possible. This method is safe to use from multiple goroutines.
func (cl *Client) GetString(key string) (string, error) {
	e, err := cl.get(key)
	if err != nil {
		return "", err
	}
	e.Lock()
	defer e.Unlock()

	if e.Type() != tDouble {
		return "", ErrWrongType
	}

	return e.Value().(string), nil
}

// put wraps the common functionality of the various put methods.
func (cl *Client) put(key string, val interface{}, entryType byte) error {
	e, err := cl.get(key)
	if err != nil {
		return err
	}
	e.Lock()
	defer e.Unlock()

	if e.Type() != entryType {
		return ErrWrongType
	}

	// BUG(Alex) Improperly handles sending, needs to wait for server to send value.
	// BUG(Alex) Doesn't handle entry assignments
	e.SetValue(val)
	e.SetSequenceNumber(e.SequenceNumber() + 1)
	cl.updateEntry(e)
	return nil
}

// PutBoolean associates the value with the key if possible. This
// method is safe to use from multiple goroutines.
func (cl *Client) PutBoolean(key string, val bool) error {
	return cl.put(key, val, tBoolean)
}

// PutFloat64 associates the value with the key if possible. This
// method is safe to use from multiple goroutines.
func (cl *Client) PutFloat64(key string, val float64) error {
	return cl.put(key, val, tDouble)
}

// PutString associates the value with the key if possible. This
// method is safe to use from multiple goroutines.
func (cl *Client) PutString(key string, val string) error {
	return cl.put(key, val, tString)
}
