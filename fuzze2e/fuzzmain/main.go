package main

import (
	"flag"
	"github.com/yasushi-saito/go-netdicom/fuzztest"
	"io/ioutil"
)

func main() {
	flag.Parse()
	fuzzData, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	fuzztest.Fuzz(fuzzData)
}
