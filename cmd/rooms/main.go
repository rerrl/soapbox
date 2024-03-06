package main

import (
	"fmt"
	"os"

	"github.com/soapboxsocial/soapbox/cmd/rooms/cmd"
)

func main() {
	fmt.Printf("Rooms starting...\n\n")
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
