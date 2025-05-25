package main

import (
	"fmt"
	"log"
	"os"

	"easybtrf5/ui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: easybtrf5 <path to btrfs partition>")
		os.Exit(1)
	}

	ui.SetupPrefixes()
	if err := ui.Run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
