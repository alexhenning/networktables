// This file contains the types and methods for handling different
// entry types in NetworkTables

package networktables

import (
	"sync"
)

type entry interface {
	Name() string
	ID() uint16
	SequenceNumber() sequenceNumber
	SetSequenceNumber(sequenceNumber)
	Type() byte
	Value() interface{}
	SetValue(interface{})
	Lock()
	Unlock()

	// dataFromBytes updates the entry value based off of the bytes
	// being received.
	dataFromBytes(<-chan byte)

	// dataToBytes returns the bytes representing the current value of
	// the entry in a form that can be sent over the network.
	dataToBytes() []byte
}

func clone(e entry) entry {
	c, _ := newEntry(e.Name(), e.ID(), e.SequenceNumber(), e.Type())
	c.SetValue(e.Value())
	return c
}

func newEntry(name string, id uint16, sequence sequenceNumber, entryType byte) (entry, error) {
	base := baseEntry{name, id, sequence, entryType, sync.Mutex{}}
	var e entry
	switch entryType {
	case tBoolean:
		e = &booleanEntry{base, false}
	case tDouble:
		e = &doubleEntry{base, 0}
	case tString:
		e = &stringEntry{base, ""}
	case tBooleanArray, tDoubleArray, tStringArray:
		return nil, ErrArraysUnsupported
	}
	return e, nil
}

// baseEntry abstracts out the commonalities between different entry
// types, including name, id, sequence number and type.
type baseEntry struct {
	name      string
	id        uint16
	sequence  sequenceNumber
	entryType byte
	sync.Mutex
}

func (e *baseEntry) Name() string {
	return e.name
}

func (e *baseEntry) ID() uint16 {
	return e.id
}

func (e *baseEntry) SequenceNumber() sequenceNumber {
	return e.sequence
}

func (e *baseEntry) SetSequenceNumber(sequence sequenceNumber) {
	e.sequence = sequence
}

func (e *baseEntry) Type() byte {
	return e.entryType
}

func (e *baseEntry) dataToBytes() []byte {
	return []byte{}
}

// booleanEntry is an entry for a boolean value
type booleanEntry struct {
	baseEntry
	value bool
}

func (e *booleanEntry) Value() interface{} {
	return e.value
}

func (e *booleanEntry) SetValue(val interface{}) {
	switch val.(type) {
	case bool:
		e.value = val.(bool)
	}
}

func (e *booleanEntry) dataFromBytes(c <-chan byte) {
	e.value = getBoolean(c)
}

func (e *booleanEntry) dataToBytes() []byte {
	return []byte{getBooleanByte(e.value)}
}

// doubleEntry is an entry for a double value
type doubleEntry struct {
	baseEntry
	value float64
}

func (e *doubleEntry) Value() interface{} {
	return e.value
}

func (e *doubleEntry) SetValue(val interface{}) {
	switch val.(type) {
	case float64:
		e.value = val.(float64)
	}
}

func (e *doubleEntry) dataFromBytes(c <-chan byte) {
	e.value = getDouble(c)
}

func (e *doubleEntry) dataToBytes() []byte {
	return getDoubleBytes(e.value)
}

// stringEntry is an entry for a string value.
type stringEntry struct {
	baseEntry
	value string
}

func (e *stringEntry) Value() interface{} {
	return e.value
}

func (e *stringEntry) SetValue(val interface{}) {
	switch val.(type) {
	case string:
		e.value = val.(string)
	}
}

func (e *stringEntry) dataFromBytes(c <-chan byte) {
	e.value = getString(c)
}

func (e *stringEntry) dataToBytes() []byte {
	return getStringBytes(e.value)
}

// Dividing point for 16 bit sequenece numbers using RFC 1982.
const sequenceNumberDividingPoint = 32768

// A value representing a sequence number.
type sequenceNumber uint16

// gt returns whether or not one sequenceNumber is greater than another.
func (s sequenceNumber) gt(other sequenceNumber) bool {
	return (s < other && other-s < sequenceNumberDividingPoint) ||
		(s > other && s-other > sequenceNumberDividingPoint)
}
