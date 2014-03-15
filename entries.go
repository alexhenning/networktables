package networktables

type entry interface {
	Name() string
	Type() byte
	Marshal() []byte
}

// Commonalities between different entry types
type baseEntry struct {
	name      string
	id        uint16
	sequence  uint16
	entryType byte
}

func (e *baseEntry) Name() string {
	return e.name
}

func (e *baseEntry) Type() byte {
	return e.entryType
}

func (e *baseEntry) BaseMarshal() []byte {
	msg := []byte{EntryAssignment}
	msg = append(msg, getStringBytes(e.name)...)
	msg = append(msg, e.Type())
	msg = append(msg, getUint16Bytes(e.id)...)
	msg = append(msg, getUint16Bytes(e.sequence)...)
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

func (e *booleanEntry) Marshal() []byte {
	msg := e.BaseMarshal()
	msg = append(msg, getBooleanByte(e.value))
	return msg
}

// doubleEntry is an entry for a double value
type doubleEntry struct {
	baseEntry
	value float64
}

func newDoubleEntry(name string, value float64, id uint16, sequence uint16) entry {
	return &doubleEntry{baseEntry{name, id, sequence, Double}, value}
}

func (e *doubleEntry) Marshal() []byte {
	msg := e.BaseMarshal()
	msg = append(msg, getDoubleBytes(e.value)...)
	return msg
}

// stringEntry is an entry for a string value
type stringEntry struct {
	baseEntry
	value string
}

func newStringEntry(name string, value string, id uint16, sequence uint16) entry {
	return &stringEntry{baseEntry{name, id, sequence, String}, value}
}

func (e *stringEntry) Marshal() []byte {
	msg := e.BaseMarshal()
	msg = append(msg, getStringBytes(e.value)...)
	return msg
}
