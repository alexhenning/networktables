package networktables

import (
	"testing"
	"time"
)

const testAddr = ":11735"

func TestSingleClient(t *testing.T) {
	server, client := NewServer(testAddr), NewClient(testAddr)
	go server.ListenAndServe()
	go client.ConnectAndListen()

	testTable(t, client)
}

func TestSingleClientSubtable(t *testing.T) {
	server, client := NewServer(testAddr), NewClient(testAddr)
	go server.ListenAndServe()
	go client.ConnectAndListen()
	sd, err := client.GetSubtable("SmartDashboard")
	assertExpectedError(t, "nil", nil, err, "for key '/SmartDashboard'")

	testTable(t, sd)
}

func TestSingleClientSubSubtable(t *testing.T) {
	server, client := NewServer(testAddr), NewClient(testAddr)
	go server.ListenAndServe()
	go client.ConnectAndListen()
	sd, err := client.GetSubtable("SmartDashboard")
	assertExpectedError(t, "nil", nil, err, "for key '/SmartDashboard'")
	tbl, err := sd.GetSubtable("test")
	assertExpectedError(t, "nil", nil, err, "for key '/SmartDashboard/test'")

	testTable(t, tbl)
}

func testTable(t *testing.T, tbl Table) {
	assertGet(t, tbl, false, 0, "", "ErrHelloNotDone", ErrHelloNotDone)
	<-time.After(100 * time.Millisecond) // Note: relying on time for synchronization is sketchy
	// Check that keys don't exist
	assertGet(t, tbl, false, 0, "", "ErrNoSuchKey", ErrNoSuchKey)

	// Set initial values and check that they propogate after 100ms
	assertPut(t, tbl, true, 42.42, "NetworkTables Rocks!", "nil", nil)
	<-time.After(100 * time.Millisecond) // Note: relying on time for synchronization is sketchy
	assertGet(t, tbl, true, 42.42, "NetworkTables Rocks!", "nil", nil)

	// Set second set of values and check that updates propogate after 100ms
	assertPut(t, tbl, false, 8080, "NT exists!", "nil", nil)
	<-time.After(100 * time.Millisecond) // Note: relying on time for synchronization is sketchy
	assertGet(t, tbl, false, 8080, "NT exists!", "nil", nil)
}

func assertGet(t *testing.T, tbl Table, expectedB bool, expectedF float64, expectedS string,
	name string, expectedE error) {
	b, err := tbl.GetBoolean("bool")
	assertExpectedError(t, name, expectedE, err, "for key '/bool'")
	assertExpectedValue(t, expectedB, b, "for key '/bool'")
	f, err := tbl.GetFloat64("float")
	assertExpectedError(t, name, expectedE, err, "for key '/float'")
	assertExpectedValue(t, expectedF, f, "for key '/float'")
	s, err := tbl.GetString("str")
	assertExpectedError(t, name, expectedE, err, "for key '/str'")
	assertExpectedValue(t, expectedS, s, "for key '/str'")
}

func assertPut(t *testing.T, tbl Table, b bool, f float64, s string, name string, expectedE error) {
	err := tbl.PutBoolean("bool", b)
	assertExpectedError(t, name, expectedE, err, "for key '/bool'")
	err = tbl.PutFloat64("float", f)
	assertExpectedError(t, name, expectedE, err, "for key '/float'")
	err = tbl.PutString("str", s)
	assertExpectedError(t, name, expectedE, err, "for key '/str'")
}

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
