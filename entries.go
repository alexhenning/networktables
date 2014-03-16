package networktables

type entry interface {
	Name() string
	ID() uint16
	SequenceNumber() sequenceNumber
	SetSequenceNumber(sequenceNumber)
	Type() byte
	dataFromBytes(<-chan byte)
	dataToBytes() []byte
}

// baseEntry abstracts out the commonalities between different entry
// types, including name, id, sequence number and type.
type baseEntry struct {
	name      string
	id        uint16
	sequence  sequenceNumber
	entryType byte
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

func assignmentMessage(e entry) []byte {
	msg := []byte{EntryAssignment}
	msg = append(msg, getStringBytes(e.Name())...)
	msg = append(msg, e.Type())
	msg = append(msg, getUint16Bytes(e.ID())...)
	msg = append(msg, getUint16Bytes(uint16(e.SequenceNumber()))...)
	msg = append(msg, e.dataToBytes()...)
	return msg
}

func updateMessage(e entry) []byte {
	msg := []byte{EntryUpdate}
	msg = append(msg, getUint16Bytes(e.ID())...)
	msg = append(msg, getUint16Bytes(uint16(e.SequenceNumber()))...)
	msg = append(msg, e.dataToBytes()...)
	return msg
}

// booleanEntry is an entry for a boolean value
type booleanEntry struct {
	baseEntry
	value bool
}

func newBooleanEntry(name string, value bool, id uint16, sequence sequenceNumber) entry {
	return &booleanEntry{baseEntry{name, id, sequence, Boolean}, value}
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

func newDoubleEntry(name string, value float64, id uint16, sequence sequenceNumber) entry {
	return &doubleEntry{baseEntry{name, id, sequence, Double}, value}
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

func newStringEntry(name string, value string, id uint16, sequence sequenceNumber) entry {
	return &stringEntry{baseEntry{name, id, sequence, String}, value}
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
