package main

import (
	"net"

	"github.com/getlantern/enhttp"
	"github.com/getlantern/golog"
)

var (
	log = golog.LoggerFor("server")
)

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}

	log.Debugf("Listening at: %v", l.Addr())
	enhttp.Serve(l)
}
