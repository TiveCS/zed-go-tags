// Command go-tags-ls is a language server that suggests Go struct tags.
package main

import (
	"bufio"
	"log"
	"os"
)

// version is set at release build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("go-tags-ls: ")
	if err := newServer().serve(bufio.NewReader(os.Stdin), os.Stdout); err != nil {
		log.Fatal(err)
	}
}
