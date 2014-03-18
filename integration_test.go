package networktables

import (
	"testing"
	"time"
)

const testAddr = ":11735"

func assertExpectedError(t *testing.T, name string, expected, actual error, extra string) {
	if actual != expected {
		t.Errorf("Expected %s(%s), but got %s --- %s\n", name, expected, actual, extra)
	}
}

func assertExpectedValue(t *testing.T, expected, actual interface{}, extra string) {
	if actual != expected {
		t.Errorf("Expected %v, but got %v --- %s\n", expected, actual, extra)
	}
}

func TestSingleClient(t *testing.T) {
	server, client := NewServer(testAddr), NewClient(testAddr, false)
	go server.ListenAndServe()
	go client.ConnectAndListen()
	<-time.After(100 * time.Millisecond) // Note: relying on time for synchronization is sketchy

	// Check that keys don't exist
	_, err := client.GetBoolean("bool")
	assertExpectedError(t, "ErrNoSuchKey", ErrNoSuchKey, err, "for key '/bool'")
	_, err = client.GetFloat64("float")
	assertExpectedError(t, "ErrNoSuchKey", ErrNoSuchKey, err, "for key '/float'")
	_, err = client.GetString("str")
	assertExpectedError(t, "ErrNoSuchKey", ErrNoSuchKey, err, "for key '/str'")

	// Set initial values for keys
	err = client.PutBoolean("bool", true)
	assertExpectedError(t, "nil", nil, err, "for key '/bool'")
	err = client.PutFloat64("float", 42.42)
	assertExpectedError(t, "nil", nil, err, "for key '/float'")
	err = client.PutString("str", "NetworkTables Rocks!")
	assertExpectedError(t, "nil", nil, err, "for key '/str'")

	<-time.After(100 * time.Millisecond) // Note: relying on time for synchronization is sketchy

	// Check for actual values
	b, err := client.GetBoolean("bool")
	assertExpectedError(t, "nil", nil, err, "for key '/bool'")
	assertExpectedValue(t, true, b, "for key '/bool'")
	f, err := client.GetFloat64("float")
	assertExpectedError(t, "nil", nil, err, "for key '/float'")
	assertExpectedValue(t, 42.42, f, "for key '/float'")
	s, err := client.GetString("str")
	assertExpectedError(t, "nil", nil, err, "for key '/str'")
	assertExpectedValue(t, "NetworkTables Rocks!", s, "for key '/str'")
}
