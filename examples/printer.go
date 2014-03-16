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
		b, err := client.GetBoolean("/bool")
		if err != nil {
			log.Println(err)
		}

		f, err := client.GetFloat64("/test")
		if err != nil {
			log.Println(err)
		}

		s, err := client.GetString("/str")
		if err != nil {
			log.Println(err)
		}

		log.Printf("%t, %f, %s\n", b, f, s)
		<-tick
	}
}
