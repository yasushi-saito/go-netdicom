package main

import (
	"log"
	"flag"
	"io/ioutil"
	"github.com/yasushi-saito/go-netdicom"
)

var (
	portFlag = flag.Int("port", 10000, "TCP port to listen to")
)

func main() {
	flag.Parse()
	if *portFlag <= 0 {
		log.Fatal("--port not set")
	}
	su := netdicom.NewServiceProvider(*portFlag)
}
