package main

import (
	"github.com/alexhenning/networktables"
	"log"
	"time"
)

func main() {
	log.Println("Starting NetworkTables client...")
	client := networktables.NewClient(":1735", true)
	tick := time.Tick(time.Duration(1 * time.Second))
	for {
		<-tick
		b, err := client.GetBoolean("/bool")
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("%t\n", b)
	}
}
