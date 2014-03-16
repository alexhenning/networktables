package main

import (
	"github.com/alexhenning/networktables"
	"log"
)

func main() {
	log.Println("Starting NetworkTables client...")
	client := networktables.NewClient(":1735", false)
	client.ConnectAndListen()
}
