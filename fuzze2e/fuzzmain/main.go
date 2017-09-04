package main

import (
	"flag"
	"github.com/yasushi-saito/go-netdicom/fuzze2e"
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
