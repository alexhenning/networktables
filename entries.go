package networktables

type entry interface {
	Name() string
	ID() uint16
	SequenceNumber() uint16
	Type() byte
	dataToBytes() []byte
}

// baseEntry abstracts out the commonalities between different entry
// types, including name, id, sequence number and type.
type baseEntry struct {
	name      string
	id        uint16
	sequence  uint16
	entryType byte
}

func (e *baseEntry) Name() string {
	return e.name
}

func (e *baseEntry) ID() uint16 {
	return e.id
}

func (e *baseEntry) SequenceNumber() uint16 {
	return e.sequence
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
	msg = append(msg, getUint16Bytes(e.SequenceNumber())...)
	msg = append(msg, e.dataToBytes()...)
	return msg
}

// booleanEntry is an entry for a boolean value
type booleanEntry struct {
	baseEntry
	value bool
}

func newBooleanEntry(name string, value bool, id uint16, sequence uint16) entry {
	return &booleanEntry{baseEntry{name, id, sequence, Boolean}, value}
}

func (e *booleanEntry) dataToBytes() []byte {
	return []byte{getBooleanByte(e.value)}
}

// doubleEntry is an entry for a double value
type doubleEntry struct {
	baseEntry
	value float64
}

func newDoubleEntry(name string, value float64, id uint16, sequence uint16) entry {
	return &doubleEntry{baseEntry{name, id, sequence, Double}, value}
}

func (e *doubleEntry) dataToBytes() []byte {
	return getDoubleBytes(e.value)
}

// stringEntry is an entry for a string value
type stringEntry struct {
	baseEntry
	value string
}

func newStringEntry(name string, value string, id uint16, sequence uint16) entry {
	return &stringEntry{baseEntry{name, id, sequence, String}, value}
}

func (e *stringEntry) dataToBytes() []byte {
	return getStringBytes(e.value)
}
