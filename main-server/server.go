package main

import (
	"github.com/alexhenning/networktables"
	"log"
)

func main() {
	log.Println("Starting NetworkTables server...")
	networktables.ListenAndServe(":1735")
}
