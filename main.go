package main

import (
	"log"

	"github.com/jylitalo/mystats/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
