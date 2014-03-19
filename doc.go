/*
Package networktables provides a server and client implementation of
the FRC NetworkTables 2.0 protocol.

This project provides an experimental Go implementation of the
NetworkTables protocol. In its current state, it has some
limitations:

- It lacks listeners/callbacks for the client

- Doesn't support the array types

- Servers can't act as clients

- Has limited real world testing (though ~80% code coverage)

- Can't get or put values when not connected to a server (returns errors)

Resources:

- The main project with support for Java and C++ is located at https://usfirst.collab.net/sf/projects/networktables/

- A pdf of the protocol specification is available at https://usfirst.collab.net/sf/docman/do/downloadDocument/projects.networktables/docman.root/doc1001/1

- Public API documentation is available on http://godoc.org/github.com/alexhenning/networktables

Server:

Starting a server is as easy as

    networktables.ListenAndServe(":1735")

Client:

Getting a client is as easy as

    client := networktables.ConnectAndListen(":1735")

Once you have a client, gets are easy and return the value and
error. If an error occurs, the default values returned are false, 0
and "" for booleans, numbers and strings.

    b, err := client.GetBoolean("/bool")
    n, err := client.GetFloat64("/num")
    s, err := client.GetString("/str")
    t, err := client.GetSubtable("/vision")

Puts are also straightforward
    err := client.PutBoolean("/bool", true)
    err := client.PutFloat64("/num", 42)
    err := client.PutString("/str", "FRC")

The default client and subtables implement the table interface for
gets and puts, so when code only cares about getting and putting, it
should use the networktables.Table type and when it cares about
connection information it should use the *networktables.Client type.

*/
package networktables
