// Convert a binary file to Go string literal lines.
// usage: go run ./tools/convert2bin images/gocon2026.rgb565
package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	bb, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}

	for i, b := range bb {
		if i%32 == 0 {
			fmt.Printf("\"")
		}
		fmt.Printf("\\x%02X", b)
		if i%32 == 31 {
			fmt.Printf("\" +\n")
		}
	}

	return nil
}
