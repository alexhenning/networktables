package networktables

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// TODO: test assignmentMessage
// TODO: test updateMessage

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

// Testing utilities

func bytesToChannel(bytes []byte) <-chan byte {
	c := make(chan byte)
	go func() {
		for _, b := range bytes {
			c <- b
		}
		close(c)
	}()
	return c
}

func randomSource(t *testing.T) *rand.Rand {
	seed := time.Now().UnixNano()
	t.Logf("Seed: %d\n", seed)
	return rand.New(rand.NewSource(seed))
}

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.,!?/\\$&[{}(=*+)]!#`8642091357%~^@<>'\";:"

func randString(source *rand.Rand) string {
	length := uint16(source.Int31n(1 << 16))
	bytes := make([]byte, length, length)
	for i := uint16(0); i < length; i++ {
		bytes[i] = chars[source.Intn(len(chars))]
	}
	return string(bytes)
}
