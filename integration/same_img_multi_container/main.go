package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

import "flag"

var port = flag.Int("port", 9999, "port to run server on")

var shouldPanic bool

func main() {
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := "🍄 mushroomtime! 🍄"
		log.Printf("Got HTTP request for %s", r.URL.Path)
		_, _ = w.Write([]byte(msg))
		time.Sleep(200 * time.Millisecond)
		shouldPanic = true
	})

	log.Printf("Serving oneup on container port %d\n", *port)
	go http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	for {
		if shouldPanic {
			panic("egads!")
		}
	}
}
