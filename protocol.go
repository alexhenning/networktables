// This file contains constants and methods for handling the
// NetworkTables protocol, including encoding and decoding bytes for
// messages.

package networktables

import (
	"encoding/binary"
	"errors"
	"math"
)

// The version of the protocol currently implemented.
const Version = 0x0200

// Values used to indicate the various message types used in the
// NetworkTables protocol.
const (
	KeepAlive          = 0x00
	Hello              = 0x01
	VersionUnsupported = 0x02
	HelloComplete      = 0x03
	EntryAssignment    = 0x10
	EntryUpdate        = 0x11
)

// Types of data that can be sent over NetworkTables.
const (
	Boolean      = 0x00
	Double       = 0x01
	String       = 0x02
	BooleanArray = 0x10
	DoubleArray  = 0x11
	StringArray  = 0x12
)

// ClientRequestID is the id clients use when requesting the server
// assign an id to the key.
const ClientRequestID = 0xFFFF

// Errors that can occur while handling connections and dealing with the protocol
var (
	ErrUnsupportedVersion    = errors.New("networktables: unsupported client version tried to connect")
	ErrUnsupportedVersionMsg = errors.New("networktables: server shouldn't get unsupported version message")
	ErrHelloCompleteMsg      = errors.New("networktables: server shouldn't get hello complete message")
	ErrAssertiveClient       = errors.New("networktables: assertive client trying to select entry ID")
	ErrArraysUnsupported     = errors.New("networktables: server currently doesn't support array types")
)

// assignmentMessage returns the bytes to send for the assignment
// message of a given entry.
func assignmentMessage(e entry) []byte {
	msg := []byte{EntryAssignment}
	msg = append(msg, getStringBytes(e.Name())...)
	msg = append(msg, e.Type())
	msg = append(msg, getUint16Bytes(e.ID())...)
	msg = append(msg, getUint16Bytes(uint16(e.SequenceNumber()))...)
	msg = append(msg, e.dataToBytes()...)
	return msg
}

// updateMessage returns the bytes to send for the update message of a
// given entry.
func updateMessage(e entry) []byte {
	msg := []byte{EntryUpdate}
	msg = append(msg, getUint16Bytes(e.ID())...)
	msg = append(msg, getUint16Bytes(uint16(e.SequenceNumber()))...)
	msg = append(msg, e.dataToBytes()...)
	return msg
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
