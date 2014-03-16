package networktables

import (
	"testing"
)

func TestBooleanEntry(t *testing.T) {
	for _, value := range []bool{false, true} {
		var expected []byte
		if value {
			expected = []byte{0X01}
		} else {
			expected = []byte{0X00}
		}
		e := newBooleanEntry("/boolean", 0X0042, 0X0000)
		e.dataFromBytes(bytesToChannel(expected))
		actual := e.dataToBytes()
		if string(actual) != string(expected) {
			t.Errorf("BooleanEntry(%t): expected %#X, actual %#X", value, expected, actual)
		}
	}
}

func TestDoubleEntry(t *testing.T) {
	source := randomSource(t)
	for i := 0; i < 1000; i++ {
		value := source.Float64()
		expected := getDoubleBytes(value)
		e := newDoubleEntry("/double", 0X0042, 0X0000)
		e.dataFromBytes(bytesToChannel(expected))
		actual := e.dataToBytes()
		if string(actual) != string(expected) {
			t.Errorf("Double(%f): expected %#X, actual %#X",
				value, expected, actual)
		}
	}
}

func TestStringEntry(t *testing.T) {
	source := randomSource(t)
	for i := 0; i < 20; i++ {
		value := randString(source)
		expected := getStringBytes(value)
		e := newStringEntry("/string", 0X0042, 0X0000)
		e.dataFromBytes(bytesToChannel(expected))
		actual := e.dataToBytes()
		if string(actual) != string(expected) {
			t.Errorf("String(%s): expected %#X, actual %#X",
				value, expected, actual)
		}
	}
}
