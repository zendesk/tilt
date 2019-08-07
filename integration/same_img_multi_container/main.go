package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

import "flag"

var port = flag.Int("port", 9999, "port to run server on")

var allWell = "Status: all is well"
var statusMsg = &allWell

func main() {
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("🍄 hello from port %d 🍄", *port)
		log.Printf("Got HTTP request for %s", r.URL.Path)
		_, _ = w.Write([]byte(msg))

		statusMsg = nil // will cause a nil pointer panic
	})

	log.Printf("Serving oneup on container port %d\n", *port)
	go http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	for {
		fmt.Println(*statusMsg)
		time.Sleep(2 * time.Second)
	}
}
