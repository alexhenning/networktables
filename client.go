// This file contains the necessary methods and data structures to run
// a NetworkTables client that can connect to a server.

package networktables

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
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
	toSend        map[string]entry
	state         state
	conn          net.Conn
	writeM        sync.Mutex
	m             sync.Mutex
}

// NewServer creates a new server object that can be used to listen
// and serve clients connected to the given address.
func NewClient(addr string, listen bool) *Client {
	cl := &Client{
		addr:          addr,
		entriesByName: make(map[string]entry),
		entriesByID:   make(map[uint16]entry),
		toSend:        make(map[string]entry),
	}
	if listen {
		go cl.ConnectAndListen()
	}
	return cl
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

	ticks, kaTicks := time.Tick(20*time.Millisecond), time.Tick(time.Second)
	go cl.sendUpdates(done, ticks)
	go cl.sendKeepAlives(done, kaTicks)

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
	written, err := cl.write(data)
	if written != len(data) && err == nil {
		err = errors.New(fmt.Sprintf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written))
	}
	return err
}

// assignEntry sends the assign entry message for an entry to the
// cliesrv.
func (cl *Client) assignEntry(e entry) {
	data := assignmentMessage(e)
	log.Printf("Send \"%X\"", data)
	written, err := cl.write(data)
	if err != nil {
		log.Println(err)
	}
	if written != len(data) {
		log.Printf("Tried to write %d bytes, but only wrote %d bytes.", len(data), written)
	}
}

// updateEntry sends the update entry message for an entry to the
// server.
func (cl *Client) updateEntry(e entry) {
	data := updateMessage(e)
	written, err := cl.write(data)
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
			if cl.state == disconnected {
				cl.state = connected
			} else {
				done <- ErrMultipleHellosCompleted
				return
			}
			// TODO: Send queued assignments
		case entryAssignment:
			if err := cl.handleEntryAssignment(c); err != nil {
				done <- err
				return
			}
		case entryUpdate:
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

// sendUpdates periodically sends the updates at a regular rate.
func (cl *Client) sendUpdates(done chan<- error, ticks <-chan time.Time) {
	// BUG(Alex) sendUpdates may run even after connection is lost.
	for _ = range ticks {
		cl.m.Lock()

		for _, e := range cl.toSend {
			e.Lock()
			cl.updateEntry(e)
			e.Unlock()
		}
		cl.toSend = make(map[string]entry)

		cl.m.Unlock()
	}
}

// sendKeepAlives periodically sends the keep-alive message
func (cl *Client) sendKeepAlives(done chan<- error, ticks <-chan time.Time) {
	for _ = range ticks {
		if cl.state == connected {
			data := []byte{keepAlive}
			written, err := cl.write(data)
			if written != len(data) && err == nil {
				err = errors.New(fmt.Sprintf("Tried to write %d bytes, but only wrote %d bytes.",
					len(data), written))
			}
			if err != nil {
				done <- err
				return
			}
		}
	}
}

// Write is a allows the connection to be written to safely from
// multiple goroutines, blocking if necessary.
func (cl *Client) write(b []byte) (int, error) {
	cl.writeM.Lock()
	defer cl.writeM.Unlock()
	return cl.conn.Write(b)
}

// entry gets the entry associated with the key if the hello finished
// command has been received and an entry with the key exists.
func (cl *Client) entry(key string) (entry, error) {
	if cl.state != connected {
		return nil, ErrHelloNotDone
	}
	e, ok := cl.entriesByName[normalizeKey(key)]
	if !ok {
		return nil, ErrNoSuchKey
	}
	return e, nil
}

// get gets the value of the entry associated with the key if the
// hello finished command has been received and an entry with the
// key exists, otherwise it returns the default value.
func (cl *Client) get(key string, defaultVal interface{}, entryType byte) (interface{}, error) {
	e, err := cl.entry(key)
	if err != nil {
		return defaultVal, err
	}
	e.Lock()
	defer e.Unlock()

	if e.Type() != entryType {
		return defaultVal, ErrWrongType
	}
	return e.Value(), nil
}

// GetBoolean returns the boolean value associated with the key if
// possible. This method is safe to use from multiple goroutines.
func (cl *Client) GetBoolean(key string) (bool, error) {
	val, err := cl.get(key, false, tBoolean)
	return val.(bool), err
}

// GetFloat64 returns the float64 value associated with the key if
// possible. This method is safe to use from multiple goroutines.
func (cl *Client) GetFloat64(key string) (float64, error) {
	val, err := cl.get(key, float64(0), tDouble)
	return val.(float64), err
}

// GetString returns the string value associated with the key if
// possible. This method is safe to use from multiple goroutines.
func (cl *Client) GetString(key string) (string, error) {
	val, err := cl.get(key, "", tString)
	return val.(string), err
}

// GetSubtable returns the subtable value associated with the
// key. This method is safe to use from multiple goroutines.
func (cl *Client) GetSubtable(key string) (Table, error) {
	return &subtable{normalizeKey(key), cl}, nil
}

// put wraps the common functionality of the various put methods.
func (cl *Client) put(key string, val interface{}, entryType byte) error {
	e, err := cl.entry(normalizeKey(key))
	if err == ErrNoSuchKey {
		err = nil
		e, err = newEntry(normalizeKey(key), clientRequestID, sequenceNumber(0), entryType)
		e.SetValue(val)
		cl.assignEntry(e)
		return err
	} else if err != nil {
		return err
	}
	cl.m.Lock()
	defer cl.m.Unlock()
	e.Lock()
	defer e.Unlock()

	if e.Type() != entryType {
		return ErrWrongType
	}

	c := clone(e)
	c.SetValue(val)
	c.SetSequenceNumber(e.SequenceNumber() + 1)
	cl.toSend[c.Name()] = c
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

// subtable implements the table interface to provide a standard way
// of accessing subtables, which are seperated by '/' just like unix
// style file paths.
type subtable struct {
	prefix string
	cl     *Client
}

func (st *subtable) GetBoolean(key string) (bool, error) {
	return st.cl.GetBoolean(st.prefix + normalizeKey(key))
}

func (st *subtable) GetFloat64(key string) (float64, error) {
	return st.cl.GetFloat64(st.prefix + normalizeKey(key))
}

func (st *subtable) GetString(key string) (string, error) {
	return st.cl.GetString(st.prefix + normalizeKey(key))
}

func (st *subtable) GetSubtable(key string) (Table, error) {
	return st.cl.GetSubtable(st.prefix + normalizeKey(key))
}

func (st *subtable) PutBoolean(key string, val bool) error {
	return st.cl.PutBoolean(st.prefix+normalizeKey(key), val)
}

func (st *subtable) PutFloat64(key string, val float64) error {
	return st.cl.PutFloat64(st.prefix+normalizeKey(key), val)
}
func (st *subtable) PutString(key string, val string) error {
	return st.cl.PutString(st.prefix+normalizeKey(key), val)
}

// Separator between tables and subtables.
const TableSeparator = "/"

// Normalizes all keys to begin with the TableSeperator and end
// without one. This allows keys to be safely appended for subtables.
func normalizeKey(key string) string {
	if string(key[0]) != TableSeparator {
		key = TableSeparator + key
	}
	if string(key[len(key)-1]) == TableSeparator {
		key = key[:len(key)-1]
	}
	return key
}
