package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"time"
)

const filename = "./foo"

func main() {
	logger := log.New(os.Stdout, "nest: ", log.LstdFlags)

	f, err := os.Open(filename)
	defer f.Close()
	if err != nil {
		logger.Fatalf("Could not open file handle %v", err)
	}

	prev := []byte("0")

	ticker := time.NewTicker(250 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				logger.Fatalf("Could not seek back to beginning of file: %v", err)
			}
			b := make([]byte, 1)
			n, err := f.Read(b)
			if err != nil {
				logger.Fatalf("Could not read bytes: %v, %d", err, n)
			}
			if !bytes.Equal(b, prev) {
				logger.Printf("Read %v\n", string(b))
				prev = b
			}
		}
	}
}
