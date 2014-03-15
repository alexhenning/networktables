package networktables

type entry interface {
	Type() uint16
}

// booleanEntry is an entry for a boolean value
type booleanEntry struct {
	value    bool
	id       uint16
	sequence uint16
}

func newBooleanEntry(value bool, id uint16, sequence uint16) entry {
	return &booleanEntry{value, id, sequence}
}

func (e *booleanEntry) Type() uint16 {
	return Boolean
}

// doubleEntry is an entry for a double value
type doubleEntry struct {
	value    float64
	id       uint16
	sequence uint16
}

func newDoubleEntry(value float64, id uint16, sequence uint16) entry {
	return &doubleEntry{value, id, sequence}
}

func (e *doubleEntry) Type() uint16 {
	return Double
}

// stringEntry is an entry for a string value
type stringEntry struct {
	value    string
	id       uint16
	sequence uint16
}

func newStringEntry(value string, id uint16, sequence uint16) entry {
	return &stringEntry{value, id, sequence}
}

func (e *stringEntry) Type() uint16 {
	return String
}
