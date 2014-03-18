package networktables

import (
	"math"
	"testing"
)

// TODO: test helloMessage + parsing
// TODO: test assignmentMessage + parsing
// TODO: test updateMessage + parsing

func TestUint16EncodingDecoding(t *testing.T) {
	source := randomSource(t)
	for i := 0; i < 1000; i++ {
		expected := uint16(source.Int31n(1 << 16))
		actual := getUint16(bytesToChannel(getUint16Bytes(expected)))
		if actual != expected {
			t.Errorf("Uint16(%d != %d): expected %#X, actual %#X", expected, actual, expected, actual)
		}
	}
}

func TestBooleanEncodingDecoding(t *testing.T) {
	for _, expected := range []bool{false, true} {
		actual := getBoolean(bytesToChannel([]byte{getBooleanByte(expected)}))
		if actual != expected {
			t.Errorf("Boolean(%t): expected %t, actual %t", expected, expected, actual)
		}
	}
}

func TestDoubleEncodingDecoding(t *testing.T) {
	source := randomSource(t)
	for i := 0; i < 1000; i++ {
		expected := source.Float64()
		actual := getDouble(bytesToChannel(getDoubleBytes(expected)))
		if actual != expected {
			t.Errorf("Double(%f != %f): expected %#X, actual %#X",
				expected, actual, math.Float64bits(expected), math.Float64bits(actual))
		}
	}
}

func TestStringEncodingDecoding(t *testing.T) {
	source := randomSource(t)
	for i := 0; i < 20; i++ {
		expected := randString(source)
		actual := getString(bytesToChannel(getStringBytes(expected)))
		if actual != expected {
			t.Errorf("String: expected %s, actual %s", expected, actual)
		}
	}
}
