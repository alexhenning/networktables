package networktables

// The version of the protocol currently implemented
const Version = 0x02

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

// Types of data that can be sent over NetworkTables
const (
	Boolean      = 0x00
	Double       = 0x01
	String       = 0x02
	BooleanArray = 0x10
	DoubleArray  = 0x11
	StringArray  = 0x12
)
