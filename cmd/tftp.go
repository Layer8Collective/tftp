package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Layer8Collective/tftp"
)

var (
	address      = flag.String("l", "127.0.0.1:69", "Listen Address")
	filename     = flag.String("f", "", "File to be Served")
	writeEnabled = flag.Bool("w", false, "Accept write request")
	readEnabled  = flag.Bool("r", true, "Accept read request")
	writedir     = flag.String("o", ".", "Directory to reside the files")
)

func main() {
	// Custom help message
	flag.Usage = func() {
		fmt.Println(`
 _____ _____ _____ ____  
|_   _|  ___|_   _|  _ \ 
  | | | |_    | | | |_) |
  | | |  _|   | | |  __/ 
  |_| |_|     |_| |_|    
			`)
		flag.PrintDefaults()
	}

	flag.Parse()

	if *readEnabled && *filename == "" {
		flag.Usage()
	}

	p, err := os.ReadFile(*filename)

	if err != nil {
		log.Fatal(err)
	}

	s := tftp.TFTPServer{Payload: p, WriteAllowed: *writeEnabled, WriteDir: *writedir, ReadAllowed: *readEnabled}
	log.Println("🚀 TFTP Server listening on: ", *address)
	log.Fatal(s.ListenAndServe(*address))
}
