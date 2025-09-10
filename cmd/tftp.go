package main

import (
	"flag"
	"log"
	"os"

	"github.com/Layer8Collective/tftp"
)

var (
	address      = flag.String("l", "127.0.0.1:69", "TFTP Server Listen Address")
	filename     = flag.String("f", "gopher.png", "File to Serve")
	writeEnabled = flag.Bool("w", false, "Accept write request")
	writedir     = flag.String("o", ".", "Directory to reside the files")
)

func main() {
	flag.Parse()

	p, err := os.ReadFile(*filename)

	if err != nil {
		log.Fatal(err)
	}

	s := tftp.TFTPServer{Payload: p, WriteAllowed: *writeEnabled, WriteDir: *writedir}
	log.Println("🚀 TFTP Server listening on: ", *address)
	log.Fatal(s.ListenAndServe(*address))
}
