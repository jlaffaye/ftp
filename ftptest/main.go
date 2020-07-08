package main

import (
	"log"
	"time"

	"github.com/jlaffaye/ftp"
)

func main() {
	_, err := ftp.Dial("localhost:22", ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}
}
