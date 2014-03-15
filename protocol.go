package networktables

import (
	"errors"
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
