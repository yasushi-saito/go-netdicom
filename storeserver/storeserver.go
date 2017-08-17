package main

import (
	"log"
	"flag"
	"strings"
	"github.com/yasushi-saito/go-netdicom"
)

var (
	portFlag = flag.String("port", "10000", "TCP port to listen to")
)

func main() {
	flag.Parse()
	port := *portFlag
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	log.Printf("Listening on %s", port)
	su := netdicom.NewServiceProvider(port)
	err := su.Run()
	if err != nil {
		panic(err)
	}
}
