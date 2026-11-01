package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Ryoku-dev/ryomarket/internal/ui"
)

var version = "development"

func main() {
	showVersion := flag.Bool("version", false, "print the RyoMarket version")
	flag.Parse()
	if *showVersion {
		fmt.Println("RyoMarket " + version)
		return
	}
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
