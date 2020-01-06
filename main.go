package main

import (
	"flag"
)

func main() {
	var port int

	flag.IntVar(&port, "port", 9001, "tcp port to listen")
	flag.Parse()

	server := NewServer(port)
	server.Start()
}
